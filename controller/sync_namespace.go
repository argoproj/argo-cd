package controller

import (
	gitopscommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// syncNamespace determine if Argo CD should create and/or manage the namespace
// where the application will be deployed.
func syncNamespace(syncPolicy *v1alpha1.SyncPolicy) func(m *unstructured.Unstructured, l *unstructured.Unstructured) (bool, error) {
	// This function must return true for the managed namespace to be synced.
	return func(managedNs, liveNs *unstructured.Unstructured) (bool, error) {
		if managedNs == nil {
			return false, nil
		}

		isNewNamespace := liveNs == nil
		isManagedNamespace := syncPolicy != nil && syncPolicy.ManagedNamespaceMetadata != nil

		// should only sync the namespace if it doesn't exist in k8s or if
		// syncPolicy is defined to manage the metadata
		if !isManagedNamespace && !isNewNamespace {
			return false, nil
		}

		if isManagedNamespace {
			managedNamespaceMetadata := syncPolicy.ManagedNamespaceMetadata
			managedNs.SetLabels(managedNamespaceMetadata.Labels)
			// managedNamespaceMetadata relies on SSA in order to avoid overriding
			// existing labels and annotations in namespaces
			managedNs.SetAnnotations(appendSSAAnnotation(managedNamespaceMetadata.Annotations))
		}

		// TODO: https://github.com/argoproj/argo-cd/issues/11196
		// err := resourceTracking.SetAppInstance(managedNs, appLabelKey, appName, "", trackingMethod)
		// if err != nil {
		// 	return false, fmt.Errorf("failed to set app instance tracking on the namespace %s: %s", managedNs.GetName(), err)
		// }

		return true, nil
	}
}

// appendSSAAnnotation will set the managed namespace to be synced
// with server-side apply
func appendSSAAnnotation(in map[string]string) map[string]string {
	r := map[string]string{}
	for k, v := range in {
		r[k] = v
	}
	r[gitopscommon.AnnotationSyncOptions] = gitopscommon.SyncOptionServerSideApply
	return r
}
