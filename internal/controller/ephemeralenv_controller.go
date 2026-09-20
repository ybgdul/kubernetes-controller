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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dev1alpha1 "operator/api/v1alpha1"
	"operator/engine"
	"operator/finalizer"
	"operator/status"
)

const finalizerName = "example.com/ephemeralenv-cleanup"

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
	runner.Register(r.reconcileDatabasePhase(ctx, &env))
	runner.Register(r.reconcileDeploymentPhase(ctx, &env))

	return runner.Execute(ctx)
}

func (r *EphemeralEnvReconiler) reconcileDatabasePhase(ctx context.Context, env *dev1alpha1.EphemeralEnv) engine.PhaseHandler { 
	return func(ctx context.Context) engine.PhaseResult { 
		if !env.Spec.IncludeDatabase {
			return engine.PhaseResult{}
		}

		var secret corev1.Secret
		err := r.Get(ctx, types.NamespacedName{Name: env.Name + "-db-secret", Namespace: env.Namespace}, &secret)
		if errors.IsNotFound(err) { 
			newSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: env.Name + "-db-secret",
					Namespace: env.Namespace,
				},
				StringData: map[string]string{"password": "supersecretpassword"},
			}
			if err := r.Create(ctx, newSecret); err != nil { 
				return engine.PhaseResult{
					Err: fmt.Errorf("failed to create db secret: %w", err),
				}
			}
			return engine.PhaseResult{Result: ctrl.Result{RequeueAfter: 2 *time.Second}}
		}
		return engine.PhaseResult{}
	}
}

func(r *EphemeralEnvReconiler) reconcileDeploymentPhase(ctx context.Context, env *dev1alpha1.EphemeralEnv) engine.PhaseHandler { 
	return func(ctx context.Context) engine.PhaseResult {
		var deploy appsv1.Deployment
		err := r.Get(ctx, types.NamespacedName{Name: env.Name, Namespace: env.Namespace}, &deploy)

		if errors.IsNotFound(err) {
			newDeploy := r.buildDeployment(env)
			if err := r.Create(ctx, newDeploy); err != nil {
				return engine.PhaseResult{
					Err: fmt.Errorf("failed to create deployment: %w", err),
				}
			}
		} else if err != nil { 
			return engine.PhaseResult{ Err: err} 
		}

		if deploy.Status.ReadyReplicas == env.Spec.Replicas {
			updated := status.SetCondition(&env.Status.Conditions, metav1.Condition{
				Type: "Ready",
				Status: metav1.ConditionTrue,
				Reason: "DeploymentReady",
				Message: "All reaplicas are healthy and ready",
			})
			if updated { 
				env.Status.Phase = "Ready"
				r.Status().Update(ctx, env)
			}
		}
	return engine.PhaseResult{Done: true}
	}
}

func(r *EphemeralEnvReconiler) buildDeployment(env *dev1alpha1.EphemeralEnv) *appsv1.Deployment { 
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: env.Name,
			Namespace: env.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(env, dev1alpha1.GroupVersion.WithKind("EphemeralEnv")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &env.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app" : env.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": env.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:"app",
							Image: env.Spec.Image,
						},
					},
				},
			},
		},
	}
}

func(r *EphemeralEnvReconiler) SetupWithManager(manager ctrl.Manager) error { 
	return ctrl.NewControllerManagedBy(manager).For(&dev1alpha1.EphemeralEnv{}).Owns(&appsv1.Deployment{}).Complete(r)
}