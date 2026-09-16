package engine

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
)

type PhaseResult struct {
	Result ctrl.Result
	Err error 
	Done bool
}

type PhaseHandler func(ctx context.Context) PhaseResult

type Runner struct {
	phases []PhaseHandler
}

func NewRunner() *Runner { 
	return &Runner{
		phases: []PhaseHandler{},
	} 
}

func (r *Runner) Register(handler PhaseHandler) *Runner { 
	r.phases = append(r.phases, handler)
	return r
}

func (r *Runner) Execute(ctx context.Context) (ctrl.Result, error) { 
	for i, phase := range r.phases { 
		res := phase(ctx)
		if res.Err != nil { 
			return res.Result, fmt.Errorf("phase %d has failed: %w", i, res.Err)
		}
		if res.Done || res.Result.Requeue || res.Result.RequeueAfter > 0 {
			return res.Result, nil
		}
	}
	return ctrl.Result{}, nil
}