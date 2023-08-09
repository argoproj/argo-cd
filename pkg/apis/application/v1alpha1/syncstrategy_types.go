package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// SyncStrategy is a set of Application resources
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=syncstrategies
// +kubebuilder:subresource:status
type SyncStrategy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              SyncStrategySpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
}

type SyncStrategyRef struct {
	Kind string `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`
	Name string `json:"name,omitempty" protobuf:"bytes,2,name=name"`
}

// SyncStrategyStrategy configures how generated Applications are updated in sequence.
type SyncStrategySpec struct {
	Type        string                       `json:"type,omitempty" protobuf:"bytes,1,opt,name=type"`
	RollingSync *SyncStrategyRolloutStrategy `json:"rollingSync,omitempty" protobuf:"bytes,2,opt,name=rollingSync"`
	// RollingUpdate *SyncStrategyRolloutStrategy `json:"rollingUpdate,omitempty" protobuf:"bytes,3,opt,name=rollingUpdate"`
}

type SyncStrategyRolloutStrategy struct {
	Steps []SyncStrategyRolloutStep `json:"steps,omitempty" protobuf:"bytes,1,opt,name=steps"`
}

type SyncStrategyRolloutStep struct {
	MatchExpressions []ApplicationMatchExpression `json:"matchExpressions,omitempty" protobuf:"bytes,1,opt,name=matchExpressions"`
	MaxUpdate        *intstr.IntOrString          `json:"maxUpdate,omitempty" protobuf:"bytes,2,opt,name=maxUpdate"`
}
