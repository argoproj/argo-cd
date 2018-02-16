package v1alpha1

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Application is a definition of Application resource.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              ApplicationSpec   `json:"spec"`
	Status            ApplicationStatus `json:"status"`
}

// ApplicationList is list of Application resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Application `json:"items"`
}

// ApplicationSpec represents desired application state. Contains link to repository with application definition and additional parameters link definition revision.
type ApplicationSpec struct {
	TargetRevision string            `json:"targetRevision"`
	Source         ApplicationSource `json:"source"`
}

// ApplicationSource contains secret reference which has information about github repository, path within repository and target application environment.
type ApplicationSource struct {
	// GitRepoSecret is a secret reference which has information about github repository.
	GitRepoSecret apiv1.LocalObjectReference `json:"gitRepoSecret"`
	// Path is a directory path within repository which contains ksonnet project.
	Path string `json:"path"`
	// Environment is a ksonnet project environment name.
	Environment string `json:"environment"`
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
	ComparisonResult ComparisonResult `json:"comparisonResult"`
	// DifferenceDetails contains string representation of detected differences between application spec and deployed application.
	DifferenceDetails string `json:"differenceDetails"`
}
