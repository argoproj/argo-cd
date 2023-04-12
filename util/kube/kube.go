package kube

import (
	"regexp"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var resourceNamePattern = regexp.MustCompile("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$")

// IsValidResourceName returns true if given string a valid Kubernetes resource name
func IsValidResourceName(name string) bool {
	return len(name) < 64 && resourceNamePattern.MatchString(name)
}

// SetAppInstanceLabel the recommended app.kubernetes.io/instance label against an unstructured object
// Uses the legacy labeling if environment variable is set
func SetAppInstanceLabel(target *unstructured.Unstructured, key, val string) error {
	labels := target.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[key] = val
	target.SetLabels(labels)
	return nil
}

// SetAppInstanceAnnotation the recommended app.kubernetes.io/instance annotation against an unstructured object
// Uses the legacy labeling if environment variable is set
func SetAppInstanceAnnotation(target *unstructured.Unstructured, key, val string) error {
	annotations := target.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[key] = val
	target.SetAnnotations(annotations)
	return nil
}

// GetAppInstanceAnnotation returns the application instance name from annotation
func GetAppInstanceAnnotation(un *unstructured.Unstructured, key string) string {
	if annotations := un.GetAnnotations(); annotations != nil {
		return annotations[key]
	}
	return ""
}

// GetAppInstanceLabel returns the application instance name from labels
func GetAppInstanceLabel(un *unstructured.Unstructured, key string) string {
	if labels := un.GetLabels(); labels != nil {
		return labels[key]
	}
	return ""
}

// RemoveLabel removes label with the specified name
func RemoveLabel(un *unstructured.Unstructured, key string) {
	labels := un.GetLabels()
	if labels == nil {
		return
	}

	for k := range labels {
		if k == key {
			delete(labels, k)
			if len(labels) == 0 {
				un.SetLabels(nil)
			} else {
				un.SetLabels(labels)
			}
			break
		}
	}
}
