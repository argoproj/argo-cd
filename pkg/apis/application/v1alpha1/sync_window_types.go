package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SyncWindowResource is a CRD that defines reusable sync windows which can be referenced
// by AppProjects and Applications.
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=syncwindows,shortName=sw;sws
type SyncWindowResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              SyncWindowResourceSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
}

// SyncWindowResourceSpec defines the desired state of a SyncWindowResource
type SyncWindowResourceSpec struct {
	// Windows is a list of sync window definitions contained in this resource.
	// Multiple windows are allowed for easy transition from projects and to group related windows.
	Windows []SyncWindowDefinition `json:"windows" protobuf:"bytes,1,rep,name=windows"`
}

// SyncWindowDefinition defines a single sync window entry within a SyncWindowResource.
// When referenced from an Application, the Applications/Namespaces/Clusters filters are ignored
// since the window applies directly to the referencing application.
type SyncWindowDefinition struct {
	// Kind defines if the window allows or blocks syncs. Must be "allow" or "deny".
	Kind string `json:"kind" protobuf:"bytes,1,opt,name=kind"`
	// Schedule is the time the window will begin, specified in cron format.
	Schedule string `json:"schedule" protobuf:"bytes,2,opt,name=schedule"`
	// Duration is the amount of time the sync window will be open (e.g. "1h", "30m").
	Duration string `json:"duration" protobuf:"bytes,3,opt,name=duration"`
	// Applications contains a list of applications that the window will apply to (glob patterns supported).
	// Ignored when the SyncWindowResource is referenced directly from an Application.
	Applications []string `json:"applications,omitempty" protobuf:"bytes,4,rep,name=applications"`
	// Namespaces contains a list of namespaces that the window will apply to (glob patterns supported).
	// Ignored when the SyncWindowResource is referenced directly from an Application.
	Namespaces []string `json:"namespaces,omitempty" protobuf:"bytes,5,rep,name=namespaces"`
	// Clusters contains a list of clusters that the window will apply to (glob patterns supported).
	// Ignored when the SyncWindowResource is referenced directly from an Application.
	Clusters []string `json:"clusters,omitempty" protobuf:"bytes,6,rep,name=clusters"`
	// ManualSync enables manual syncs when they would otherwise be blocked.
	ManualSync bool `json:"manualSync,omitempty" protobuf:"bytes,7,opt,name=manualSync"`
	// TimeZone of the sync window schedule (IANA timezone, e.g. "America/New_York"). Defaults to UTC.
	TimeZone string `json:"timeZone,omitempty" protobuf:"bytes,8,opt,name=timeZone"`
	// UseAndOperator uses AND operator for matching applications, namespaces and clusters instead of the default OR.
	UseAndOperator bool `json:"andOperator,omitempty" protobuf:"bytes,9,opt,name=andOperator"`
	// Description of the sync window (max 255 chars).
	// +kubebuilder:validation:MaxLength=255
	Description string `json:"description,omitempty" protobuf:"bytes,10,opt,name=description"`
	// SyncOverrun allows ongoing syncs to continue past window boundaries.
	SyncOverrun bool `json:"syncOverrun,omitempty" protobuf:"bytes,11,opt,name=syncOverrun"`
}

// SyncWindowResourceList is a list of SyncWindowResource resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SyncWindowResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Items           []SyncWindowResource `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// SyncWindowRef is a reference to a SyncWindowResource CRD.
type SyncWindowRef struct {
	// Name is the metadata.name of the SyncWindowResource to reference.
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// Selector is a label selector to match SyncWindowResource objects.
	// Either Name or Selector should be specified, not both.
	Selector *metav1.LabelSelector `json:"selector,omitempty" protobuf:"bytes,2,opt,name=selector"`
}

// SyncWindowProjectRef is a reference to a SyncWindowResource from an AppProject.
// It allows specifying additional filters that restrict which apps the referenced
// sync windows apply to.
type SyncWindowProjectRef struct {
	// Ref references a SyncWindowResource by name or selector.
	Ref SyncWindowRef `json:"ref" protobuf:"bytes,1,opt,name=ref"`
	// Applications restricts which applications the referenced sync windows apply to (glob patterns supported).
	Applications []string `json:"applications,omitempty" protobuf:"bytes,2,rep,name=applications"`
	// Namespaces restricts which namespaces the referenced sync windows apply to (glob patterns supported).
	Namespaces []string `json:"namespaces,omitempty" protobuf:"bytes,3,rep,name=namespaces"`
	// Clusters restricts which clusters the referenced sync windows apply to (glob patterns supported).
	Clusters []string `json:"clusters,omitempty" protobuf:"bytes,4,rep,name=clusters"`
}
