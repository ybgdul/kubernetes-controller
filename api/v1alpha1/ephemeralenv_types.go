package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type EphemeralEnvSpec struct {
	// +kubebuilder:default=1
	Replicas        int32  `json:"replicas,omitempty"`
	IncludeDatabase bool   `json:"includeDatabase,omitempty"`
	Branch          string `json:"branch"`
	Image           string `json:"image"`
}

type EphemeralEnvStatus struct {
	Phase      string             `json:"phase,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

type EphemeralEnv struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EphemeralEnvSpec   `json:"spec,omitempty"`
	Status EphemeralEnvStatus `json:"status,omitempty"`
}

// DeepCopyObject implements [cacheapi.Object].
func (e *EphemeralEnv) DeepCopyObject() runtime.Object {
	panic("unimplemented")
}

// +kubebuilder:object:root=true

type EphemeralEnvList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EphemeralEnv `json:"items"`
}
