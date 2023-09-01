package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// SyncStrategyList is list of SyncStrategy resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SyncStrategyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Items           []SyncStrategy `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// SyncStrategy defines a strategy to sync Applications from an ApplicationSet
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=syncstrategies,shortName=syncstrategy;syncstrategies
type SyncStrategy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              SyncStrategySpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
}

// ClusterSyncStrategyList is list of ClusterSyncStrategy resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterSyncStrategyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Items           []ClusterSyncStrategy `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ClusterSyncStrategy defines a strategy to sync Applications from ApplicationSet
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,path=clustersyncstrategies,shortName=clustersyncstrategy;clustersyncstrategies
type ClusterSyncStrategy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              SyncStrategySpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
}

// Utility struct for a reference to a sync strategy.
type SyncStrategyRef struct {
	Kind string `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`
	Name string `json:"name,omitempty" protobuf:"bytes,2,name=name"`
}

// SyncStrategySpec configures how generated Applications are updated in sequence.
type SyncStrategySpec struct {
	Type        string                       `json:"type,omitempty" protobuf:"bytes,1,opt,name=type"`
	RollingSync *SyncStrategyRolloutStrategy `json:"rollingSync,omitempty" protobuf:"bytes,2,opt,name=rollingSync"`
	// RollingUpdate *ApplicationSetSyncStrategyRolloutStrategy `json:"rollingUpdate,omitempty" protobuf:"bytes,3,opt,name=rollingUpdate"`
}

type SyncStrategyRolloutStrategy struct {
	Steps []SyncStrategyRolloutStep `json:"steps,omitempty" protobuf:"bytes,1,opt,name=steps"`
}

type SyncStrategyRolloutStep struct {
	MatchExpressions []ApplicationMatchExpression `json:"matchExpressions,omitempty" protobuf:"bytes,1,opt,name=matchExpressions"`
	MaxUpdate        *intstr.IntOrString          `json:"maxUpdate,omitempty" protobuf:"bytes,2,opt,name=maxUpdate"`
}
