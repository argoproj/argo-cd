package kube

import (
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var resourceNamePattern = regexp.MustCompile("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$")

// IsValidResourceName returns true if given string a valid Kubernetes resource name
func IsValidResourceName(name string) bool {
	return len(name) < 64 && resourceNamePattern.MatchString(name)
}

// SetAppInstanceLabel the recommended app.kubernetes.io/instance label against an unstructured object
func SetAppInstanceLabel(target *unstructured.Unstructured, key, val string) error {
	labels, _, err := nestedNullableStringMap(target.Object, "metadata", "labels")
	if err != nil {
		return fmt.Errorf("failed to get labels from target object %s %s/%s: %w", target.GroupVersionKind().String(), target.GetNamespace(), target.GetName(), err)
	}
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[key] = val
	target.SetLabels(labels)
	return nil
}

// SetAppInstanceAnnotation the recommended app.kubernetes.io/instance annotation against an unstructured object
func SetAppInstanceAnnotation(target *unstructured.Unstructured, key, val string) error {
	annotations, _, err := nestedNullableStringMap(target.Object, "metadata", "annotations")
	if err != nil {
		return fmt.Errorf("failed to get annotations from target object %s %s/%s: %w", target.GroupVersionKind().String(), target.GetNamespace(), target.GetName(), err)
	}

	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[key] = val
	target.SetAnnotations(annotations)
	return nil
}

// GetAppInstanceAnnotation returns the application instance name from annotation
func GetAppInstanceAnnotation(un *unstructured.Unstructured, key string) (string, error) {
	annotations, _, err := nestedNullableStringMap(un.Object, "metadata", "annotations")
	if err != nil {
		return "", fmt.Errorf("failed to get annotations from target object %s %s/%s: %w", un.GroupVersionKind().String(), un.GetNamespace(), un.GetName(), err)
	}
	if annotations != nil {
		return annotations[key], nil
	}
	return "", nil
}

// GetAppInstanceLabel returns the application instance name from labels
func GetAppInstanceLabel(un *unstructured.Unstructured, key string) (string, error) {
	labels, _, err := nestedNullableStringMap(un.Object, "metadata", "labels")
	if err != nil {
		return "", fmt.Errorf("failed to get labels for %s %s/%s: %w", un.GroupVersionKind().String(), un.GetNamespace(), un.GetName(), err)
	}
	if labels != nil {
		return labels[key], nil
	}
	return "", nil
}

// RemoveLabel removes label with the specified name
func RemoveLabel(un *unstructured.Unstructured, key string) error {
	labels, _, err := nestedNullableStringMap(un.Object, "metadata", "labels")
	if err != nil {
		return fmt.Errorf("failed to get labels for %s %s/%s: %w", un.GroupVersionKind().String(), un.GetNamespace(), un.GetName(), err)
	}
	if labels == nil {
		return nil
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
	return nil
}

// nestedNullableStringMap returns a copy of map[string]string value of a nested field.
// Returns false if value is not found and an error if not one of map[string]interface{} or nil, or contains non-string values in the map.
func nestedNullableStringMap(obj map[string]interface{}, fields ...string) (map[string]string, bool, error) {
	var m map[string]string
	val, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if err != nil {
		return nil, found, err
	}
	if found && val != nil {
		return unstructured.NestedStringMap(obj, fields...)
	}
	return m, found, err
}
