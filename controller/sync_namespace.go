package controller

import (
	"fmt"
	cdcommon "github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo"
	gitopscommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func syncNamespace(resourceTracking argo.ResourceTracking, appLabelKey string, trackingMethod v1alpha1.TrackingMethod, appName string, syncPolicy *v1alpha1.SyncPolicy) func(un *unstructured.Unstructured) (bool, error) {
	return func(liveNs *unstructured.Unstructured) (bool, error) {
		if liveNs != nil && kube.GetAppInstanceLabel(liveNs, cdcommon.LabelKeyAppInstance) != "" {
			kube.UnsetLabel(liveNs, cdcommon.LabelKeyAppInstance)
			return true, nil
		}

		isNewNamespace := liveNs != nil && liveNs.GetUID() == "" && liveNs.GetResourceVersion() == ""

		if liveNs != nil && syncPolicy != nil {
			// managedNamespaceMetadata relies on SSA, and since the diffs are computed by the k8s control plane we
			// always need to call the k8s api server, so we'll always need to return true if managedNamespaceMetadata is set.
			hasManagedMetadata := syncPolicy.ManagedNamespaceMetadata != nil
			if hasManagedMetadata {
				managedNamespaceMetadata := syncPolicy.ManagedNamespaceMetadata
				liveNs.SetLabels(managedNamespaceMetadata.Labels)
				liveNs.SetAnnotations(appendSSAAnnotation(managedNamespaceMetadata.Annotations))

				err := resourceTracking.SetAppInstance(liveNs, appLabelKey, appName, "", trackingMethod)
				if err != nil {
					return false, fmt.Errorf("failed to set app instance tracking on the namespace %s: %s", liveNs.GetName(), err)
				}

				return true, nil
			}
		}

		return isNewNamespace, nil
	}
}

func appendSSAAnnotation(in map[string]string) map[string]string {
	r := map[string]string{}
	for k, v := range in {
		r[k] = v
	}
	r[gitopscommon.AnnotationSyncOptions] = gitopscommon.SyncOptionServerSideApply
	return r
}
