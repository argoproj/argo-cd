package status

import (
	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func BuildResourceStatus(statusMap map[string]argov1alpha1.ResourceStatus, apps []argov1alpha1.Application) map[string]argov1alpha1.ResourceStatus {
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
