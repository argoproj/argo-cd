package controller

import (
	"fmt"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo"
	gitopscommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// syncNamespace determines whether Argo CD should create and/or manage the namespace
// where the application will be deployed.
func syncNamespace(resourceTracking argo.ResourceTracking, appLabelKey string, trackingMethod v1alpha1.TrackingMethod, app *v1alpha1.Application, appNamespace string) func(m *unstructured.Unstructured, l *unstructured.Unstructured) (bool, error) {
	// This function must return true for the managed namespace to be synced.
	return func(managedNs, liveNs *unstructured.Unstructured) (bool, error) {
		if managedNs == nil {
			return false, nil
		}

		syncPolicy := app.Spec.SyncPolicy
		isNewNamespace := liveNs == nil
		isManagedNamespace := syncPolicy != nil && syncPolicy.ManagedNamespaceMetadata != nil

		// should only sync the namespace if it doesn't exist in k8s or if
		// syncPolicy is defined to manage the metadata
		if !isManagedNamespace && !isNewNamespace {
			return false, nil
		}

		if isManagedNamespace {
			appInstanceName := app.InstanceName(appNamespace)
			if liveNs != nil {
				// first check if another application owns the live namespace
				liveAppName := resourceTracking.GetAppName(liveNs, appLabelKey, trackingMethod)
				if liveAppName != "" && liveAppName != appInstanceName {
					log.Errorf("expected namespace %s to be managed by application %s, but it's managed by application %s", liveNs.GetName(), app.Name, liveAppName)
					return false, fmt.Errorf("namespace %s is managed by another application than %s", liveNs.GetName(), app.Name)
				}
			}

			managedNamespaceMetadata := syncPolicy.ManagedNamespaceMetadata
			managedNs.SetLabels(managedNamespaceMetadata.Labels)
			// managedNamespaceMetadata relies on SSA in order to avoid overriding
			// existing labels and annotations in namespaces
			managedNs.SetAnnotations(appendSSAAnnotation(managedNamespaceMetadata.Annotations))

			// set ownership of the namespace to the current application
			err := resourceTracking.SetAppInstance(managedNs, appLabelKey, appInstanceName, "", trackingMethod)
			if err != nil {
				return false, fmt.Errorf("failed to set app instance tracking on the namespace %s: %s", managedNs.GetName(), err)
			}
		}

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
