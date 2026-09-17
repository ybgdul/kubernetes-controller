package controller

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dev1alpha1 "operator/api/v1alpha1"
	"operator/engine"
	"operator/finalizer"
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
	runner.Register()

	return runner.Execute(ctx)
}

func (r *EphemeralEnvReconiler) reconcileDatabasePhase(ctx context.Context, env *dev1alpha1.EphemeralEnv) engine.PhaseHandler { 
	return func(ctx context.Context) engine.PhaseResult { 
		if !env.Spec.IncludeDatabase {
			return engine.PhaseResult{}
		}

		var secret corev1.Secret
		err := r.Get(ctx, &env)
	}
}