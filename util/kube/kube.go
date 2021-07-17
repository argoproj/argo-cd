package kube

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/argo-cd/v2/common"
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
	if key != common.LabelKeyLegacyApplicationName {
		// we no longer label the pod template sub resources in v0.11
		return nil
	}

	gvk := schema.FromAPIVersionAndKind(target.GetAPIVersion(), target.GetKind())
	// special case for deployment and job types: make sure that derived replicaset, and pod has
	// the application label
	switch gvk.Group {
	case "apps", "extensions":
		switch gvk.Kind {
		case kube.DeploymentKind, kube.ReplicaSetKind, kube.StatefulSetKind, kube.DaemonSetKind:
			templateLabels, ok, err := unstructured.NestedMap(target.UnstructuredContent(), "spec", "template", "metadata", "labels")
			if err != nil {
				return err
			}
			if !ok || templateLabels == nil {
				templateLabels = make(map[string]interface{})
			}
			templateLabels[key] = val
			err = unstructured.SetNestedMap(target.UnstructuredContent(), templateLabels, "spec", "template", "metadata", "labels")
			if err != nil {
				return err
			}
			// The following is a workaround for issue #335. In API version extensions/v1beta1 or
			// apps/v1beta1, if a spec omits spec.selector then k8s will default the
			// spec.selector.matchLabels to match spec.template.metadata.labels. This means Argo CD
			// labels can potentially make their way into spec.selector.matchLabels, which is a bad
			// thing. The following logic prevents this behavior.
			switch target.GetAPIVersion() {
			case "apps/v1beta1", "extensions/v1beta1":
				selector, _, err := unstructured.NestedMap(target.UnstructuredContent(), "spec", "selector")
				if err != nil {
					return err
				}
				if len(selector) == 0 {
					// If we get here, user did not set spec.selector in their manifest. We do not want
					// our Argo CD labels to get defaulted by kubernetes, so we explicitly set the labels
					// for them (minus the Argo CD labels).
					delete(templateLabels, key)
					err = unstructured.SetNestedMap(target.UnstructuredContent(), templateLabels, "spec", "selector", "matchLabels")
					if err != nil {
						return err
					}
				}
			}
		}
	case "batch":
		switch gvk.Kind {
		case kube.JobKind:
			templateLabels, ok, err := unstructured.NestedMap(target.UnstructuredContent(), "spec", "template", "metadata", "labels")
			if err != nil {
				return err
			}
			if !ok || templateLabels == nil {
				templateLabels = make(map[string]interface{})
			}
			templateLabels[key] = val
			err = unstructured.SetNestedMap(target.UnstructuredContent(), templateLabels, "spec", "template", "metadata", "labels")
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// isNullList checks if the object is a "List" type where items is null instead of an empty list.
// Handles a corner case where obj.IsList() returns false when a manifest is like:
// ---
// apiVersion: v1
// items: null
// kind: ConfigMapList
func isNullList(obj *unstructured.Unstructured) bool {
	if _, ok := obj.Object["spec"]; ok {
		return false
	}
	if _, ok := obj.Object["status"]; ok {
		return false
	}
	field, ok := obj.Object["items"]
	if !ok {
		return false
	}
	return field == nil
}

func FromUnstructured(objs []*unstructured.Unstructured) ([]string, error) {
	manifests := make([]string, 0)
	for _, obj := range objs {
		var targets []*unstructured.Unstructured
		if obj.IsList() {
			err := obj.EachListItem(func(object runtime.Object) error {
				unstructuredObj, ok := object.(*unstructured.Unstructured)
				if ok {
					targets = append(targets, unstructuredObj)
					return nil
				}
				return fmt.Errorf("resource list item has unexpected type")
			})
			if err != nil {
				return nil, err
			}
		} else if isNullList(obj) {
			// noop
		} else {
			targets = []*unstructured.Unstructured{obj}
		}

		for _, target := range targets {
			manifestStr, err := json.Marshal(target.Object)
			if err != nil {
				return nil, err
			}
			manifests = append(manifests, string(manifestStr))
		}
	}
	return manifests, nil
}
