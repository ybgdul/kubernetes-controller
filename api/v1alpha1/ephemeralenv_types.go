// +kubebuilder:object:generate=true
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EphemeralEnvSpec struct {
	// +kubebuilder:default=1
	Replicas        int32  `json:"replicas,omitempty"`
	IncludeDatabase bool   `json:"includeDatabase,omitempty"`
	Branch          string `json:"branch"`
	Image           string `json:"image"`
	TTL string `json:"ttl,omitempty"`
}

type EphemeralEnvStatus struct {
	Phase      string             `json:"phase,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	URL string `json:"url,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

type EphemeralEnv struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EphemeralEnvSpec   `json:"spec,omitempty"`
	Status EphemeralEnvStatus `json:"status,omitempty"`
}


// +kubebuilder:object:root=true

type EphemeralEnvList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EphemeralEnv `json:"items"`
}

