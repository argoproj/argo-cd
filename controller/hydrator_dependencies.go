package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/controller/hydrator/types"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	argoutil "github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/hydrator"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

const (
	// HydratorMetadataFile is the filename for hydrator metadata
	HydratorMetadataFile = "hydrator.metadata"
)

// hydratedCommitValidation represents a cached validation result for an app
type hydratedCommitValidation struct {
	syncSHA    string // The sync branch SHA that was validated
	rootDrySHA string // DrySHA from root metadata (expected/fresh)
	pathDrySHA string // DrySHA from path metadata (actual)
	isValid    bool   // Whether path matches root (fresh=true, stale=false)
}

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

func (ctrl *ApplicationController) GetRepoObjs(ctx context.Context, origApp *appv1.Application, drySource appv1.ApplicationSource, revision string, project *appv1.AppProject) ([]*unstructured.Unstructured, *apiclient.ManifestResponse, error) {
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
	objs, resp, _, err := ctrl.appStateManager.GetRepoObjs(ctx, app, drySources, appLabelKey, dryRevisions, true, true, false, project, false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get repo objects: %w", err)
	}
	trackingMethod, err := ctrl.settingsMgr.GetTrackingMethod()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get tracking method: %w", err)
	}
	for _, obj := range objs {
		if err := argoutil.NewResourceTracking().RemoveAppInstance(obj, trackingMethod); err != nil {
			return nil, nil, fmt.Errorf("failed to remove the app instance value: %w", err)
		}
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

func (ctrl *ApplicationController) GetHydratorCommitMessageTemplate() (string, error) {
	sourceHydratorCommitMessageKey, err := ctrl.settingsMgr.GetSourceHydratorCommitMessageTemplate()
	if err != nil {
		return "", fmt.Errorf("failed to get sourceHydrator commit message template key: %w", err)
	}

	return sourceHydratorCommitMessageKey, nil
}

func (ctrl *ApplicationController) GetCommitAuthorName() (string, error) {
	authorName, err := ctrl.settingsMgr.GetCommitAuthorName()
	if err != nil {
		return "", fmt.Errorf("failed to get commit author name: %w", err)
	}
	return authorName, nil
}

func (ctrl *ApplicationController) GetCommitAuthorEmail() (string, error) {
	authorEmail, err := ctrl.settingsMgr.GetCommitAuthorEmail()
	if err != nil {
		return "", fmt.Errorf("failed to get commit author email: %w", err)
	}
	return authorEmail, nil
}

// ValidateHydratedCommitFreshness checks if the hydrated commit at the sync source
// was produced for the current dry source SHA by comparing hydrator.metadata files.
//
// Strategy:
//   - Root hydrator.metadata contains the "expected" drySHA (from latest hydration)
//   - Path-specific hydrator.metadata contains the "actual" drySHA for that path
//   - If they match, the path is fresh; if they differ, the path is stale
//
// Missing metadata files are treated as stale (isValid=false) rather than errors,
// as this is an expected state for an hydrator source configuration.
//
// Returns:
//   - isValid: true if path metadata matches root (fresh), false if mismatch (stale) or missing files
//   - rootDrySHA: the drySHA from root metadata (expected/fresh value)
//   - pathDrySHA: the drySHA from path metadata (actual value)
//   - err: error only for unexpected failures (repo access, client creation, etc.)
func (ctrl *ApplicationController) ValidateHydratedCommitFreshness(
	ctx context.Context,
	app *appv1.Application,
	syncBranchSHA string,
) (isValid bool, rootDrySHA string, pathDrySHA string, err error) {
	if app.Spec.SourceHydrator == nil {
		return true, "", "", nil
	}

	syncSource := app.Spec.SourceHydrator.GetSyncSource()

	// Get repository credentials
	repo, err := ctrl.db.GetRepository(ctx, syncSource.RepoURL, app.Spec.Project)
	if err != nil {
		return false, "", "", fmt.Errorf("failed to get repository: %w", err)
	}

	// Fetch hydrator.metadata files from sync branch
	closer, repoClient, err := ctrl.repoClientset.NewRepoServerClient()
	if err != nil {
		return false, "", "", fmt.Errorf("failed to create repo client: %w", err)
	}
	defer utilio.Close(closer)

	// Use GetGitFiles to fetch all hydrator.metadata files
	resp, err := repoClient.GetGitFiles(ctx, &apiclient.GitFilesRequest{
		Repo:                      repo,
		Revision:                  syncBranchSHA,
		Path:                      "**/" + HydratorMetadataFile,
		NewGitFileGlobbingEnabled: true,
	})
	if err != nil {
		return false, "", "", fmt.Errorf("failed to fetch %s: %w", HydratorMetadataFile, err)
	}

	// Parse root metadata (contains the expected/fresh drySHA)
	rootMetadata, rootExists := resp.Map[HydratorMetadataFile]
	if !rootExists {
		// Missing root metadata is expected during initial hydration - treat as stale, not error
		return false, "", "", nil
	}

	var rootMeta hydrator.HydratorCommitMetadata
	if err := json.Unmarshal(rootMetadata, &rootMeta); err != nil {
		// Malformed metadata is unexpected - return error
		return false, "", "", fmt.Errorf("failed to parse root metadata: %w", err)
	}

	// Parse path-specific metadata (contains the actual drySHA for this path)
	pathMetadataKey := filepath.Join(syncSource.Path, HydratorMetadataFile)
	pathMetadata, pathExists := resp.Map[pathMetadataKey]
	if !pathExists {
		// Missing path metadata is expected when path hasn't been hydrated yet - treat as stale, not error
		return false, rootMeta.DrySHA, "", nil
	}

	var pathMeta hydrator.HydratorCommitMetadata
	if err := json.Unmarshal(pathMetadata, &pathMeta); err != nil {
		// Malformed metadata is unexpected - return error
		return false, rootMeta.DrySHA, "", fmt.Errorf("failed to parse path metadata: %w", err)
	}

	// Compare: if path drySHA matches root drySHA, the path is fresh
	isValid = pathMeta.DrySHA == rootMeta.DrySHA

	return isValid, rootMeta.DrySHA, pathMeta.DrySHA, nil
}

// shouldBlockAutoSyncForHydrator checks if auto-sync should be blocked for a hydrator app
// due to stale hydrated files. It validates that the path-specific hydrator.metadata drySHA
// matches the root hydrator.metadata drySHA. Uses in-memory cache to avoid repeated repo
// server calls during reconcile loops.
//
// Returns true if auto-sync should be blocked (stale or validation error), false otherwise.
func (ctrl *ApplicationController) shouldBlockAutoSyncForHydrator(app *appv1.Application, syncRevision string, logCtx *log.Entry) bool {
	if app.Spec.SourceHydrator == nil {
		return false
	}

	cacheKey := fmt.Sprintf("%s/%s", app.Namespace, app.Name)

	// Check in-memory cache first
	ctrl.hydratedCacheLock.RLock()
	cached, exists := ctrl.hydratedCommitCache[cacheKey]
	ctrl.hydratedCacheLock.RUnlock()

	// Use cached result if it's for the same sync SHA
	if exists && cached.syncSHA == syncRevision {
		if !cached.isValid {
			logCtx.Infof("Skipping auto-sync: hydrated files at %s are stale (path drySHA: %s, root drySHA: %s)",
				syncRevision, cached.pathDrySHA, cached.rootDrySHA)
			return true
		}
		logCtx.Debugf("Using cached validation for sync commit %s (fresh)", syncRevision)
		return false
	}

	// Cache miss or different SHA - validate now
	isValid, rootDrySHA, pathDrySHA, err := ctrl.ValidateHydratedCommitFreshness(
		context.TODO(),
		app,
		syncRevision,
	)
	if err != nil {
		// Unexpected error - don't cache, block auto-sync
		logCtx.WithError(err).Warn("Skipping auto-sync: failed to validate hydrated commit freshness")
		return true
	}

	// Cache successful validation result (both fresh and stale states)
	ctrl.hydratedCacheLock.Lock()
	ctrl.hydratedCommitCache[cacheKey] = &hydratedCommitValidation{
		syncSHA:    syncRevision,
		rootDrySHA: rootDrySHA,
		pathDrySHA: pathDrySHA,
		isValid:    isValid,
	}
	ctrl.hydratedCacheLock.Unlock()

	if !isValid {
		logCtx.Infof("Skipping auto-sync: hydrated files at %s are stale (path drySHA: %s, root drySHA: %s)",
			syncRevision, pathDrySHA, rootDrySHA)
		return true
	}

	logCtx.Debugf("Validated sync commit %s is fresh (drySHA: %s)", syncRevision, rootDrySHA)
	return false
}
