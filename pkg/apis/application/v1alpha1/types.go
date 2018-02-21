package v1alpha1

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Application is a definition of Application resource.
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              ApplicationSpec   `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	Status            ApplicationStatus `json:"status" protobuf:"bytes,3,opt,name=status"`
}

// ApplicationList is list of Application resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Application `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ApplicationSpec represents desired application state. Contains link to repository with application definition and additional parameters link definition revision.
type ApplicationSpec struct {
	TargetRevision string            `json:"targetRevision" protobuf:"bytes,1,opt,name=targetRevision"`
	Source         ApplicationSource `json:"source" protobuf:"bytes,2,opt,name=source"`
}

// ApplicationSource contains secret reference which has information about github repository, path within repository and target application environment.
type ApplicationSource struct {
	// GitRepoSecret is a secret reference which has information about github repository.
	GitRepoSecret apiv1.LocalObjectReference `json:"gitRepoSecret" protobuf:"bytes,1,opt,name=gitRepoSecret"`
	// Path is a directory path within repository which contains ksonnet project.
	Path string `json:"path" protobuf:"bytes,2,opt,name=path"`
	// Environment is a ksonnet project environment name.
	Environment string `json:"environment" protobuf:"bytes,3,opt,name=environment"`
}

// ComparisonResult is a comparison result of application spec and deployed application.
type ComparisonResult string

// Possible comparison results
const (
	ComparisonResultUnknown   ComparisonResult = "Unknown"
	ComparisonResultError     ComparisonResult = "Error"
	ComparisonResultEqual     ComparisonResult = "Equal"
	ComparisonResultDifferent ComparisonResult = "Different"
)

// ApplicationStatus contains information about application status in target environment.
type ApplicationStatus struct {
	// ComparisonResult is a comparison result of application spec and deployed application.
	ComparisonResult ComparisonResult `json:"comparisonResult" protobuf:"bytes,1,opt,name=comparisonResult,casttype=ComparisonResult"`
	// DifferenceDetails contains string representation of detected differences between application spec and deployed application.
	DifferenceDetails string `json:"differenceDetails" protobuf:"bytes,2,opt,name=differenceDetails"`
}

// Cluster is the definition of a cluster resource
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              ClusterSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
}

// ClusterList is a collection of Clusters.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Cluster `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ClusterSpec is the cluster specification
type ClusterSpec struct {
	// Server is the API server URL of the Kubernetes cluster
	Server string `json:"server" protobuf:"bytes,1,opt,name=server"`
}

// Repository is a Git repository holding application configurations
type Repository struct {
	Repo     string `json:"repo" protobuf:"bytes,1,opt,name=repo"`
	Username string `json:"username" protobuf:"bytes,2,opt,name=username"`
	Password string `json:"password" protobuf:"bytes,3,opt,name=password"`
}

// RepositoryList is a collection of Repositories.
type RepositoryList struct {
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Repository `json:"items" protobuf:"bytes,2,rep,name=items"`
}
