package status

import (
	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func BuildResourceStatus(statusMap map[string]argov1alpha1.ResourceStatus, apps []argov1alpha1.Application) map[string]argov1alpha1.ResourceStatus {
	appMap := map[string]argov1alpha1.Application{}
	for _, app := range apps {
		appCopy := app
		appMap[app.Name] = app

		gvk := app.GroupVersionKind()
		// Create status if it does not exist
		status, ok := statusMap[app.Name]
		if !ok {
			status = argov1alpha1.ResourceStatus{
				Group:     gvk.Group,
				Version:   gvk.Version,
				Kind:      gvk.Kind,
				Name:      app.Name,
				Namespace: app.Namespace,
				Status:    app.Status.Sync.Status,
				Health:    &appCopy.Status.Health,
			}
		}

		status.Group = gvk.Group
		status.Version = gvk.Version
		status.Kind = gvk.Kind
		status.Name = app.Name
		status.Namespace = app.Namespace
		status.Status = app.Status.Sync.Status
		status.Health = &appCopy.Status.Health

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
