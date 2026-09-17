package controller

import (

	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

const finalizerName = "example.com/ephemeralenv-cleanup"

type EphemeralEnvReconiler struct{ 
	client.Client
	Scheme *runtime.Scheme
}