package v1alpha

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EphemeralEnvSpec struct{ 
	Replicas int32 `json:"replicas,omitempty"`
	IncludeDatabase bool `"json:includeDatabase,omitempty"`
	Branch string `json:"branch"`
	Image string `json:"image"`
}

type EphemeralEnvStatus struct{ 
	Phase string `json:"phase,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type EphemeralEnv struct{
	metav1.TypeMeta `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec EphemeralEnvSpec `json:"spec,omitempty"`
	Status EphemeralEnvStatus `json:"status,omitempty"`
}

type EphemeralEnvList struct{ 
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items []EphemeralEnv `json:"items"`
}