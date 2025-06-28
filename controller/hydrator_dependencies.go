package controller

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/controller/hydrator/types"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	argoutil "github.com/argoproj/argo-cd/v3/util/argo"

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

func (ctrl *ApplicationController) GetRepoObjs(origApp *appv1.Application, drySource appv1.ApplicationSource, revision string, project *appv1.AppProject) ([]*unstructured.Unstructured, *apiclient.ManifestResponse, error) {
	drySources := []appv1.ApplicationSource{drySource}
	dryRevisions := []string{revision}

	appLabelKey, err := ctrl.settingsMgr.GetAppInstanceLabelKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get app instance label key: %w", err)
	}

	app := origApp.DeepCopy()
	// Remove the manifest generate path annotation, because the feature will misbehave for apps using source hydrator.
	// Setting this annotation causes GetRepoObjs to compare the dry source commit to the most recent synced commit. The
	// problem is that the most recent synced commit is likely on the hydrated branch, not the dry branch. The
	// comparison will throw an error and break hydration.
	//
	// The long-term solution will probably be to persist the synced _dry_ revision and use that for the comparison.
	delete(app.Annotations, appv1.AnnotationKeyManifestGeneratePaths)

	// FIXME: use cache and revision cache
	objs, resp, _, err := ctrl.appStateManager.GetRepoObjs(app, drySources, appLabelKey, dryRevisions, true, true, false, project, false, false)
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

func (ctrl *ApplicationController) RequestAppRefresh(appName string, appNamespace string) error {
	// We request a refresh by setting the annotation instead of by adding it to the refresh queue, because there is no
	// guarantee that the hydrator is running on the same controller shard as is processing the application.

	// This function is called for each app after a hydrate operation is completed so that the app controller can pick
	// up the newly-hydrated changes. So we set hydrate=false to avoid a hydrate loop.
	_, err := argoutil.RefreshApp(ctrl.applicationClientset.ArgoprojV1alpha1().Applications(appNamespace), appName, appv1.RefreshTypeNormal, false)
	if err != nil {
		return fmt.Errorf("failed to request app refresh: %w", err)
	}
	return nil
}

func (ctrl *ApplicationController) PersistAppHydratorStatus(orig *appv1.Application, newStatus *appv1.SourceHydratorStatus) {
	status := orig.Status.DeepCopy()
	status.SourceHydrator = *newStatus
	ctrl.persistAppStatus(orig, status)
}

func (ctrl *ApplicationController) AddHydrationQueueItem(key types.HydrationQueueKey) {
	ctrl.hydrationQueue.AddRateLimited(key)
}
