package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Cluster is the definition of a cluster resource
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              ClusterSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
}

// ClusterSpec is the cluster specification
type ClusterSpec struct {
	Server string `json:"server" protobuf:"bytes,1,req,name=server"`
}

// ClusterList is a collection of Clusters.
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Cluster `json:"items" protobuf:"bytes,2,rep,name=items"`
}
