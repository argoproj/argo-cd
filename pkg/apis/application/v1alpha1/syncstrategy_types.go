package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ApplicationSetSyncStrategyList is list of AppProject resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ApplicationSetSyncStrategyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Items           []ApplicationSetSyncStrategy `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ApplicationSetSyncStrategy is a set of Application resources
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=syncstrategies
// +kubebuilder:subresource:status
type ApplicationSetSyncStrategy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              ApplicationSetSyncStrategySpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
}

type ApplicationSetSyncStrategyRef struct {
	Kind string `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`
	Name string `json:"name,omitempty" protobuf:"bytes,2,name=name"`
}

// ApplicationSetSyncStrategyStrategy configures how generated Applications are updated in sequence.
type ApplicationSetSyncStrategySpec struct {
	Type        string                                     `json:"type,omitempty" protobuf:"bytes,1,opt,name=type"`
	RollingSync *ApplicationSetSyncStrategyRolloutStrategy `json:"rollingSync,omitempty" protobuf:"bytes,2,opt,name=rollingSync"`
	// RollingUpdate *ApplicationSetSyncStrategyRolloutStrategy `json:"rollingUpdate,omitempty" protobuf:"bytes,3,opt,name=rollingUpdate"`
}

type ApplicationSetSyncStrategyRolloutStrategy struct {
	Steps []ApplicationSetSyncStrategyRolloutStep `json:"steps,omitempty" protobuf:"bytes,1,opt,name=steps"`
}

type ApplicationSetSyncStrategyRolloutStep struct {
	MatchExpressions []ApplicationMatchExpression `json:"matchExpressions,omitempty" protobuf:"bytes,1,opt,name=matchExpressions"`
	MaxUpdate        *intstr.IntOrString          `json:"maxUpdate,omitempty" protobuf:"bytes,2,opt,name=maxUpdate"`
}
