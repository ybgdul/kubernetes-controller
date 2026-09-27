package controller

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	appsv1 "k8s.io/api/apps/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	applyappsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"

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
		// deletion logic 
		return nil
	})
	if handled || err != nil { 
		return ctrl.Result{}, err
	}

	if updated, err := finalizerManager.EnsureFinalizer(ctx, &env); updated || err != nil { 
		return ctrl.Result{}, err
	}

	runner := engine.NewRunner()
	runner.Register(r.reconcileDeploymentSSA(&env))

	return runner.Execute(ctx)
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
			return engine.PhaseResult{
					Err: fmt.Errorf("failed to apply deployment with SSA: %w", err),
			}
		}
		
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
