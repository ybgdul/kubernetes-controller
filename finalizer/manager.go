package finalizer

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Manager struct{ 
	client.Client
	FinalizerName string
}

func NewManager(c client.Client, finalizerName string) *Manager { 
	return &Manager{
		Client: c,
		FinalizerName: finalizerName,
	}
}

func (m *Manager) EnsureFinalizer(ctx context.Context, obj client.Object) (bool, error) { 
	if !controllerutil.ContainsFinalizer(obj, m.FinalizerName) { 
		controllerutil.AddFinalizer(obj, m.FinalizerName)
		return true, m.Update(ctx, obj)
	}
	return false, nil 
}

func (m *Manager) HandleDeletion(ctx context.Context, obj client.Object, cleanUp func(context.Context) error) (bool, error) { 
	if obj.GetDeletionTimestamp().IsZero() { 
		return false, nil
	}

	if controllerutil.ContainsFinalizer(obj, m.FinalizerName) {
		if err := cleanUp(ctx); err != nil { 
			return true, err
		}

		controllerutil.RemoveFinalizer(obj, m.FinalizerName)
		if err := m.Update(ctx, obj); err != nil {
			return true, err
		}
	}

	return true, nil
}