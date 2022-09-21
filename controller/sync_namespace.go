package controller

import (
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo"
	gitopscommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func syncNamespace(resourceTracking argo.ResourceTracking, appLabelKey string, trackingMethod v1alpha1.TrackingMethod, appName string, syncPolicy *v1alpha1.SyncPolicy) func(un *unstructured.Unstructured) (bool, error) {
	return func(un *unstructured.Unstructured) (bool, error) {
		isNewNamespace := un != nil && un.GetUID() == "" && un.GetResourceVersion() == ""

		if un != nil && syncPolicy != nil {
			// managedNamespaceMetadata relies on SSA, and since the diffs are computed by the k8s control plane we
			// always need to call the k8s api server, so we'll always need to return true if managedNamespaceMetadata is set.
			hasManagedMetadata := syncPolicy.ManagedNamespaceMetadata != nil
			if hasManagedMetadata {
				managedNamespaceMetadata := syncPolicy.ManagedNamespaceMetadata
				un.SetLabels(managedNamespaceMetadata.Labels)
				un.SetAnnotations(appendNamespaceSSA(managedNamespaceMetadata.Annotations))
			}

			hasNoResourceTracking := resourceTracking.GetAppInstance(un, appLabelKey, trackingMethod) == nil
			if hasNoResourceTracking {
				err := resourceTracking.SetAppInstance(un, appLabelKey, appName, "", trackingMethod)
				if err != nil {
					return false, err
				}
			}

			return hasManagedMetadata || hasNoResourceTracking, nil
		}

		return isNewNamespace, nil
	}
}

func appendNamespaceSSA(in map[string]string) map[string]string {
	r := map[string]string{}
	for k, v := range in {
		r[k] = v
	}
	r[gitopscommon.AnnotationSyncOptions] = gitopscommon.SyncOptionServerSideApply
	return r
}
