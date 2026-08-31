package status

import (
	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// BuildResourceStatus builds the resource status entries for the live Applications owned by the
// ApplicationSet. Applications that are owned but no longer generated (for example left behind
// after a generator change under the create-only or create-update applicationsSync policies, or
// not yet deleted under the delete-capable policies) are marked as Orphaned.
func BuildResourceStatus(statusMap map[string]argov1alpha1.ResourceStatus, apps []argov1alpha1.Application, generatedApps []argov1alpha1.Application) map[string]argov1alpha1.ResourceStatus {
	generated := make(map[string]bool, len(generatedApps))
	for _, app := range generatedApps {
		generated[app.Name] = true
	}

	appMap := map[string]argov1alpha1.Application{}
	for _, app := range apps {
		appMap[app.Name] = app

		gvk := app.GroupVersionKind()
		var status argov1alpha1.ResourceStatus
		status.Group = gvk.Group
		status.Version = gvk.Version
		status.Kind = gvk.Kind
		status.Name = app.Name
		status.Namespace = app.Namespace
		status.Status = app.Status.Sync.Status
		status.Health = &argov1alpha1.HealthStatus{Status: app.Status.Health.Status}
		// An Application that is already being deleted is being reaped, not orphaned; without this
		// the delete-capable policies would briefly report routine scale-downs as orphans.
		status.Orphaned = !generated[app.Name] && app.DeletionTimestamp == nil

		statusMap[app.Name] = status
	}
	cleanupDeletedApplicationStatuses(statusMap, appMap)

	return statusMap
}

func GetResourceStatusMap(appset *argov1alpha1.ApplicationSet) map[string]argov1alpha1.ResourceStatus {
	statusMap := map[string]argov1alpha1.ResourceStatus{}
	for _, status := range appset.Status.Resources {
		statusMap[status.Name] = status
	}
	return statusMap
}

func cleanupDeletedApplicationStatuses(statusMap map[string]argov1alpha1.ResourceStatus, apps map[string]argov1alpha1.Application) {
	for name := range statusMap {
		if _, ok := apps[name]; !ok {
			delete(statusMap, name)
		}
	}
}
