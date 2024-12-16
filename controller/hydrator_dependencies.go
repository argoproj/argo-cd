package controller

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v2/controller/hydrator"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

/**
This file implements the hydrator.Dependencies interface for the ApplicationController.

Hydration logic does not belong in this file. The methods here should be "bookkeeping" methods that keep hydration work
in the hydrator and app controller work in the app controller. The only purpose of this file is to provide the hydrator
safe, minimal access to certain app controller functionality to avoid duplicate code.
*/

func (ctrl *ApplicationController) GetProcessableAppProj(app *appv1.Application) (*appv1.AppProject, error) {
	return ctrl.getAppProj(app)
}

// GetProcessableApps returns a list of applications that are processable by the controller.
func (ctrl *ApplicationController) GetProcessableApps() (*appv1.ApplicationList, error) {
	// getAppList already filters out applications that are not processable by the controller.
	return ctrl.getAppList(metav1.ListOptions{})
}

func (ctrl *ApplicationController) GetRepoObjs(app *appv1.Application, source appv1.ApplicationSource, revision string, project *appv1.AppProject) ([]*unstructured.Unstructured, *apiclient.ManifestResponse, error) {
	sources := []appv1.ApplicationSource{source}
	revisions := []string{revision}

	appLabelKey, err := ctrl.settingsMgr.GetAppInstanceLabelKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get app instance label key: %w", err)
	}

	// FIXME: use cache and revision cache
	objs, resp, _, err := ctrl.appStateManager.GetRepoObjs(app, sources, appLabelKey, revisions, true, true, false, project, false, false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get repo objects: %w", err)
	}

	if len(resp) != 1 {
		return nil, nil, fmt.Errorf("expected one manifest response, got %d", len(resp))
	}

	return objs, resp[0], nil
}

func (ctrl *ApplicationController) GetWriteCredentials(ctx context.Context, repoURL string, project string) (*appv1.Repository, error) {
	return ctrl.db.GetWriteRepository(ctx, repoURL, project)
}

func (ctrl *ApplicationController) RequestAppRefresh(appName string) {
	ctrl.requestAppRefresh(appName, CompareWithLatest.Pointer(), nil)
}

func (ctrl *ApplicationController) PersistAppHydratorStatus(orig *appv1.Application, newStatus *appv1.SourceHydratorStatus) {
	status := orig.Status.DeepCopy()
	status.SourceHydrator = *newStatus
	ctrl.persistAppStatus(orig, status)
}

func (ctrl *ApplicationController) AddHydrationQueueItem(key hydrator.HydrationQueueKey) {
	ctrl.hydrationQueue.AddRateLimited(key)
}
