package kube

import (
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/argo-cd/common"
)

// GetAppInstanceLabel returns the application instance name from labels
func GetAppInstanceIdentifier(un *unstructured.Unstructured, key string) string {
	if labels := un.GetLabels(); labels != nil {
		return labels[key]
	}
	return ""
}

func SetAppInstanceIdentifier(target *unstructured.Unstructured, key, val string) error {
	return SetAppInstanceIdentifier(target, key, val)
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

// SetAppInstanceAnnotation the recommended app.kubernetes.io/instance annotation against an unstructured object
// Uses the legacy annotation if environment variable is set
func SetAppInstanceAnnotation(target *unstructured.Unstructured, key, val string) error {
	annotations := target.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[key] = val
	target.SetAnnotations(annotations)
	if key != common.LabelKeyLegacyApplicationName {
		// we no longer annotate the pod template sub resources in v0.11
		return nil
	}

	gvk := schema.FromAPIVersionAndKind(target.GetAPIVersion(), target.GetKind())
	// special case for deployment and job types: make sure that derived replicaset, and pod has
	// the application annotation
	switch gvk.Group {
	case "apps", "extensions":
		switch gvk.Kind {
		case kube.DeploymentKind, kube.ReplicaSetKind, kube.StatefulSetKind, kube.DaemonSetKind:
			templateAnnotations, ok, err := unstructured.NestedMap(target.UnstructuredContent(), "spec", "template", "metadata", "annotations")
			if err != nil {
				return err
			}
			if !ok || templateAnnotations == nil {
				templateAnnotations = make(map[string]interface{})
			}
			templateAnnotations[key] = val
			err = unstructured.SetNestedMap(target.UnstructuredContent(), templateAnnotations, "spec", "template", "metadata", "annotations")
			if err != nil {
				return err
			}
		}
	case "batch":
		switch gvk.Kind {
		case kube.JobKind:
			templateAnnotations, ok, err := unstructured.NestedMap(target.UnstructuredContent(), "spec", "template", "metadata", "annotations")
			if err != nil {
				return err
			}
			if !ok || templateAnnotations == nil {
				templateAnnotations = make(map[string]interface{})
			}
			templateAnnotations[key] = val
			err = unstructured.SetNestedMap(target.UnstructuredContent(), templateAnnotations, "spec", "template", "metadata", "annotations")
			if err != nil {
				return err
			}
		}
	}
	return nil
}
