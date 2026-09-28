package controller

import (
	"context"
	"fmt"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	applyappsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/tools/record"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dev1alpha1 "operator/api/v1alpha1"
	"operator/engine"
	"operator/finalizer"
	"operator/status"
)

const (
	finalizerName = "opearator/ephemeral-cleanup"
	fieldManager = "ephemeral-env-operator"
)

type EphemeralEnvReconiler struct{ 
	client.Client
	Scheme *runtime.Scheme
	Record record.EventRecorder
}

func (r *EphemeralEnvReconiler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) { 
	logger := log.FromContext(ctx)

	var env dev1alpha1.EphemeralEnv
	if err := r.Get(ctx, req.NamespacedName, &env); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	finalizerManager := finalizer.NewManager(r.Client, finalizerName)

	handled, err := finalizerManager.HandleDeletion(ctx, &env, func(ctx context.Context) error { 
		logger.Info("Cleaning up external environment resources", "name", env.Name)
		r.Record.Event(
				&env,
				corev1.EventTypeNormal,
				"CleanupStarted",
				"Deleting external resources",
			)
		// deletion logic 
		return nil
	})
	if handled || err != nil { 
		r.Record.Event(
				&env,
				corev1.EventTypeWarning,
				"CleanupFailed",
				fmt.Sprintf("Failed to delete external resources: %v", err),
			)
		return ctrl.Result{}, err
	}

	if updated, err := finalizerManager.EnsureFinalizer(ctx, &env); updated || err != nil { 
		return ctrl.Result{}, err
	}

	runner := engine.NewRunner()
	runner.Register(r.reconcileTTLPhase(&env))
	runner.Register(r.reconcileDeploymentSSA(&env))

	return runner.Execute(ctx)
}

func (r *EphemeralEnvReconiler) reconcileTTLPhase(env *dev1alpha1.EphemeralEnv) engine.PhaseHandler { 
	return func(ctx context.Context) engine.PhaseResult{ 
		logger := log.FromContext(ctx)

		if env.Spec.TTL == "" { 
			return engine.PhaseResult{}
		}

		ttlDuration, err := time.ParseDuration(env.Spec.TTL)
		if err != nil { 
			logger.Error(err, "invalid TTL format: ", env.Spec.TTL)
			return engine.PhaseResult{}
		}

		creationTime := env.CreationTimestamp.Time
		expirationTime := creationTime.Add(ttlDuration)
		now := time.Now()

		if now.After(expirationTime) || now.Equal(expirationTime) {
			r.Record.Event(
				env,
				corev1.EventTypeNormal,
				"TTL expired",
				fmt.Sprintf("TTL has expired (%s) for resource, initiating deletion", env.Spec.TTL),
			)
			logger.Info("TTL has expired for resource, initiating deletion",
				"name", env.Name,
				"created", creationTime,
				"ttl", env.Spec.TTL,
			)

			if err := r.Delete(ctx, env); err != nil { 
				return engine.PhaseResult{Err: fmt.Errorf("failed to delete expired EnvironmentalEnv: %w", err)}
			}

			return engine.PhaseResult{Done: true}
		}
		remaining := expirationTime.Sub(now)
		logger.Info("Scheduling TTL auto-deletion check", "remaining", remaining.String())
		return engine.PhaseResult{
			Result: ctrl.Result{
				RequeueAfter: remaining,
			},
		}
	}
}

func (r *EphemeralEnvReconiler) reconcileDeploymentSSA(env *dev1alpha1.EphemeralEnv) engine.PhaseHandler { 
	return func(ctx context.Context) engine.PhaseResult { 
		deployApply := applyappsv1.Deployment(env.Name, env.Namespace).
					WithKind("Deployment").
					WithAPIVersion("apps/v1").
					WithOwnerReferences(
						applymetav1.OwnerReference().
							WithAPIVersion(dev1alpha1.GroupVersion.String()).
							WithKind("EphemeralEnv").
							WithName(env.Name).
							WithUID(env.UID).
							WithController(true).
							WithBlockOwnerDeletion(true),
					).
					WithSpec(applyappsv1.DeploymentSpec().
						WithReplicas(env.Spec.Replicas).
						WithSelector(applymetav1.LabelSelector().
							WithMatchLabels(map[string]string{"app": env.Name}),
						).
						WithTemplate(applycorev1.PodTemplateSpec().
							WithLabels(map[string]string{"app": env.Name}).
							WithSpec(applycorev1.PodSpec().
								WithContainers(applycorev1.Container().
									WithName("app").
									WithImage(env.Spec.Image),
								),
							),
						),
					)
		err := r.Apply(ctx, deployApply, client.FieldOwner(fieldManager), client.ForceOwnership)
		if err != nil {
			r.Record.Event(
				env,
				corev1.EventTypeWarning,
				"ApplyFailed",
				fmt.Sprintf("Failed to apply child Deployment via SSA: %v", err),
			)
			return engine.PhaseResult{
					Err: fmt.Errorf("failed to apply deployment with SSA: %w", err),
			}
		}
		r.Record.Event(
			env,
			corev1.EventTypeWarning,
			"DeploymentApplied",
			fmt.Sprintf("Succesfully applied child Deployment %s/%s with image %s", env.Namespace, env.Name, env.Spec.Image),
		)
		
		var currentDeploy appsv1.Deployment
		if err := r.Get(ctx, types.NamespacedName{Name: env.Name, Namespace: env.Namespace}, &currentDeploy); err != nil {
			if currentDeploy.Status.ReadyReplicas == env.Spec.Replicas { 
				updated := status.SetCondition(&env.Status.Conditions, metav1.Condition{
					Type: "Ready",
					Status: metav1.ConditionTrue,
					Reason: "DeploymentReady",
					Message: "All replicas are healthy and ready",
				})
				if updated { 
					env.Status.Phase = "Ready"
					r.Status().Update(ctx, env)
				}
			}
		}
		
		return engine.PhaseResult{Done: true}
	}
}

func (r *EphemeralEnvReconiler) SetupWithManager(manager ctrl.Manager) error { 
	return ctrl.NewControllerManagedBy(manager).For(&dev1alpha1.EphemeralEnv{}).Owns(&appsv1.Deployment{}).Complete(r)
}
