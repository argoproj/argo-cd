package hydrator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	commitclient "github.com/argoproj/argo-cd/v2/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v2/controller/utils"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
)

// Dependencies is the interface for the dependencies of the Hydrator. It serves two purposes: 1) it prevents the
// hydrator from having direct access to the app controller, and 2) it allows for easy mocking of dependencies in tests.
// If you add something here, be sure that it is something the app controller needs to provide to the hydrator.
type Dependencies interface {
	// TODO: determine if we actually need to get the app, or if all the stuff we need the app for is done already on
	//       the app controller side.
	GetProcessableAppProj(app *appv1.Application) (*appv1.AppProject, error)
	GetProcessableApps() (*appv1.ApplicationList, error)
	GetRepoObjs(app *appv1.Application, source appv1.ApplicationSource, revision string, project *appv1.AppProject) ([]*unstructured.Unstructured, *apiclient.ManifestResponse, error)
	GetWriteCredentials(ctx context.Context, repoURL string, project string) (*appv1.Repository, error)
	RequestAppRefresh(appName string)
	// TODO: only allow access to the hydrator status
	PersistAppHydratorStatus(orig *appv1.Application, newStatus *appv1.SourceHydratorStatus)
	AddHydrationQueueItem(key HydrationQueueKey)
}

type Hydrator struct {
	dependencies         Dependencies
	statusRefreshTimeout time.Duration
	commitClientset      commitclient.Clientset
}

func NewHydrator(dependencies Dependencies, statusRefreshTimeout time.Duration, commitClientset commitclient.Clientset) *Hydrator {
	return &Hydrator{
		dependencies:         dependencies,
		statusRefreshTimeout: statusRefreshTimeout,
		commitClientset:      commitClientset,
	}
}

func (h *Hydrator) ProcessAppHydrateQueueItem(origApp *appv1.Application) {
	origApp = origApp.DeepCopy()
	app := origApp.DeepCopy()

	if app.Spec.SourceHydrator == nil {
		return
	}

	logCtx := utils.GetAppLog(app)

	logCtx.Debug("Processing app hydrate queue item")

	// TODO: don't reuse statusRefreshTimeout. Create a new timeout for hydration.
	needsHydration, reason := appNeedsHydration(origApp, h.statusRefreshTimeout)
	if !needsHydration {
		return
	}

	logCtx.WithField("reason", reason).Info("Hydrating app")

	app.Status.SourceHydrator.CurrentOperation = &appv1.HydrateOperation{
		StartedAt:      metav1.Now(),
		FinishedAt:     nil,
		Phase:          appv1.HydrateOperationPhaseHydrating,
		SourceHydrator: *app.Spec.SourceHydrator,
	}
	h.dependencies.PersistAppHydratorStatus(origApp, &app.Status.SourceHydrator)
	origApp.Status.SourceHydrator = app.Status.SourceHydrator
	h.dependencies.AddHydrationQueueItem(getHydrationQueueKey(app))

	logCtx.Debug("Successfully processed app hydrate queue item")
}

func getHydrationQueueKey(app *appv1.Application) HydrationQueueKey {
	destinationBranch := app.Spec.SourceHydrator.SyncSource.TargetBranch
	if app.Spec.SourceHydrator.HydrateTo != nil {
		destinationBranch = app.Spec.SourceHydrator.HydrateTo.TargetBranch
	}
	key := HydrationQueueKey{
		SourceRepoURL:        app.Spec.SourceHydrator.DrySource.RepoURL,
		SourceTargetRevision: app.Spec.SourceHydrator.DrySource.TargetRevision,
		DestinationBranch:    destinationBranch,
	}
	return key
}

type HydrationQueueKey struct {
	SourceRepoURL        string
	SourceTargetRevision string
	DestinationBranch    string
}

// uniqueHydrationDestination is used to detect duplicate hydrate destinations.
type uniqueHydrationDestination struct {
	sourceRepoURL        string
	sourceTargetRevision string
	destinationBranch    string
	destinationPath      string
}

func (h *Hydrator) ProcessHydrationQueueItem(hydrationKey HydrationQueueKey) (processNext bool) {
	logCtx := log.WithFields(log.Fields{
		"sourceRepoURL":        hydrationKey.SourceRepoURL,
		"sourceTargetRevision": hydrationKey.SourceTargetRevision,
		"destinationBranch":    hydrationKey.DestinationBranch,
	})

	relevantApps, drySHA, hydratedSHA, err := h.hydrateAppsLatestCommit(logCtx, hydrationKey)
	if drySHA != "" {
		logCtx = logCtx.WithField("drySHA", drySHA)
	}
	if err != nil {
		logCtx.WithField("appCount", len(relevantApps)).WithError(err).Error("Failed to hydrate apps")
		for _, app := range relevantApps {
			origApp := app.DeepCopy()
			app.Status.SourceHydrator.CurrentOperation.Phase = appv1.HydrateOperationPhaseFailed
			failedAt := metav1.Now()
			app.Status.SourceHydrator.CurrentOperation.FinishedAt = &failedAt
			app.Status.SourceHydrator.CurrentOperation.Message = fmt.Sprintf("Failed to hydrated revision %s: %v", drySHA, err.Error())
			h.dependencies.PersistAppHydratorStatus(origApp, &app.Status.SourceHydrator)
			logCtx = logCtx.WithField("app", app.QualifiedName())
			logCtx.Errorf("Failed to hydrate app: %v", err)
		}
		return
	}
	logCtx.WithField("appCount", len(relevantApps)).Debug("Successfully hydrated apps")
	finishedAt := metav1.Now()
	for _, app := range relevantApps {
		origApp := app.DeepCopy()
		operation := &appv1.HydrateOperation{
			StartedAt:      app.Status.SourceHydrator.CurrentOperation.StartedAt,
			FinishedAt:     &finishedAt,
			Phase:          appv1.HydrateOperationPhaseHydrated,
			Message:        "",
			DrySHA:         drySHA,
			HydratedSHA:    hydratedSHA,
			SourceHydrator: app.Status.SourceHydrator.CurrentOperation.SourceHydrator,
		}
		app.Status.SourceHydrator.CurrentOperation = operation
		app.Status.SourceHydrator.LastSuccessfulOperation = &appv1.SuccessfulHydrateOperation{
			DrySHA:         drySHA,
			HydratedSHA:    hydratedSHA,
			SourceHydrator: app.Status.SourceHydrator.CurrentOperation.SourceHydrator,
		}
		h.dependencies.PersistAppHydratorStatus(origApp, &app.Status.SourceHydrator)
		// Request a refresh since we pushed a new commit.
		h.dependencies.RequestAppRefresh(app.QualifiedName())
	}
	return
}

func (h *Hydrator) hydrateAppsLatestCommit(logCtx *log.Entry, hydrationKey HydrationQueueKey) ([]*appv1.Application, string, string, error) {
	relevantApps, err := h.getRelevantAppsForHydration(logCtx, hydrationKey)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get relevant apps for hydration: %w", err)
	}

	hydratedRevision, dryRevision, err := h.hydrate(logCtx, relevantApps)
	if err != nil {
		return relevantApps, dryRevision, "", fmt.Errorf("failed to hydrate apps: %w", err)
	}

	return relevantApps, dryRevision, hydratedRevision, nil
}

func (h *Hydrator) getRelevantAppsForHydration(logCtx *log.Entry, hydrationKey HydrationQueueKey) ([]*appv1.Application, error) {
	// Get all apps
	apps, err := h.dependencies.GetProcessableApps()
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	var relevantApps []*appv1.Application
	uniqueDestinations := make(map[uniqueHydrationDestination]bool, len(apps.Items))
	for _, app := range apps.Items {
		if app.Spec.SourceHydrator == nil {
			continue
		}

		if app.Spec.SourceHydrator.DrySource.RepoURL != hydrationKey.SourceRepoURL ||
			app.Spec.SourceHydrator.DrySource.TargetRevision != hydrationKey.SourceTargetRevision {
			continue
		}
		destinationBranch := app.Spec.SourceHydrator.SyncSource.TargetBranch
		if app.Spec.SourceHydrator.HydrateTo != nil {
			destinationBranch = app.Spec.SourceHydrator.HydrateTo.TargetBranch
		}
		if destinationBranch != hydrationKey.DestinationBranch {
			continue
		}

		var proj *appv1.AppProject
		proj, err = h.dependencies.GetProcessableAppProj(&app)
		if err != nil {
			return nil, fmt.Errorf("failed to get project %q for app %q: %w", app.Spec.Project, app.QualifiedName(), err)
		}
		permitted := proj.IsSourcePermitted(app.Spec.GetSource())
		if !permitted {
			// Log and skip. We don't want to fail the entire operation because of one app.
			logCtx.Warnf("App %q is not permitted to use source %q", app.QualifiedName(), app.Spec.Source.String())
			continue
		}

		uniqueDestinationKey := uniqueHydrationDestination{
			sourceRepoURL:        app.Spec.SourceHydrator.DrySource.RepoURL,
			sourceTargetRevision: app.Spec.SourceHydrator.DrySource.TargetRevision,
			destinationBranch:    destinationBranch,
			destinationPath:      app.Spec.SourceHydrator.SyncSource.Path,
		}
		// TODO: test the dupe detection
		if _, ok := uniqueDestinations[uniqueDestinationKey]; ok {
			return nil, fmt.Errorf("multiple app hydrators use the same destination: %v", uniqueDestinationKey)
		}
		uniqueDestinations[uniqueDestinationKey] = true

		relevantApps = append(relevantApps, &app)
	}
	return relevantApps, nil
}

func (h *Hydrator) hydrate(logCtx *log.Entry, apps []*appv1.Application) (string, string, error) {
	if len(apps) == 0 {
		return "", "", nil
	}
	repoURL := apps[0].Spec.SourceHydrator.DrySource.RepoURL
	syncBranch := apps[0].Spec.SourceHydrator.SyncSource.TargetBranch
	targetBranch := apps[0].Spec.GetHydrateToSource().TargetRevision
	var paths []*commitclient.PathDetails
	projects := make(map[string]bool, len(apps))
	var targetRevision string
	// TODO: parallelize this loop
	for _, app := range apps {
		project, err := h.dependencies.GetProcessableAppProj(app)
		if err != nil {
			return "", "", fmt.Errorf("failed to get project: %w", err)
		}
		projects[project.Name] = true
		drySource := appv1.ApplicationSource{
			RepoURL:        app.Spec.SourceHydrator.DrySource.RepoURL,
			Path:           app.Spec.SourceHydrator.DrySource.Path,
			TargetRevision: app.Spec.SourceHydrator.DrySource.TargetRevision,
		}
		if targetRevision == "" {
			targetRevision = app.Spec.SourceHydrator.DrySource.TargetRevision
		}

		// TODO: enable signature verification
		objs, resp, err := h.dependencies.GetRepoObjs(app, drySource, targetRevision, project)
		if err != nil {
			return "", "", fmt.Errorf("failed to get repo objects: %w", err)
		}

		targetRevision = resp.Revision

		// Set up a ManifestsRequest
		manifestDetails := make([]*commitclient.HydratedManifestDetails, len(objs))
		for i, obj := range objs {
			objJson, err := json.Marshal(obj)
			if err != nil {
				return "", "", fmt.Errorf("failed to marshal object: %w", err)
			}
			manifestDetails[i] = &commitclient.HydratedManifestDetails{ManifestJSON: string(objJson)}
		}

		paths = append(paths, &commitclient.PathDetails{
			Path:      app.Spec.SourceHydrator.SyncSource.Path,
			Manifests: manifestDetails,
			Commands:  resp.Commands,
		})
	}

	// If all the apps are under the same project, use that project. Otherwise, use an empty string to indicate that we
	// need global creds.
	project := ""
	if len(projects) == 1 {
		for p := range projects {
			project = p
		}
	}

	repo, err := h.dependencies.GetWriteCredentials(context.Background(), repoURL, project)
	if err != nil {
		return "", "", fmt.Errorf("failed to get hydrator credentials: %w", err)
	}
	if repo == nil {
		// Try without credentials.
		repo = &appv1.Repository{
			Repo: repoURL,
		}
		logCtx.Warn("no credentials found for repo, continuing without credentials")
	}

	manifestsRequest := commitclient.CommitHydratedManifestsRequest{
		Repo:          repo,
		SyncBranch:    syncBranch,
		TargetBranch:  targetBranch,
		DrySha:        targetRevision,
		CommitMessage: "[Argo CD Bot] hydrate " + targetRevision,
		Paths:         paths,
	}

	closer, commitService, err := h.commitClientset.NewCommitServerClient()
	if err != nil {
		return "", "", fmt.Errorf("failed to create commit service: %w", err)
	}
	defer argoio.Close(closer)
	resp, err := commitService.CommitHydratedManifests(context.Background(), &manifestsRequest)
	if err != nil {
		return "", "", fmt.Errorf("failed to commit hydrated manifests: %w", err)
	}
	return targetRevision, resp.HydratedSha, nil
}

// appNeedsHydration answers if application needs manifests hydrated.
func appNeedsHydration(app *appv1.Application, statusHydrateTimeout time.Duration) (needsHydration bool, reason string) {
	if app.Spec.SourceHydrator == nil {
		return false, "source hydrator not configured"
	}

	var hydratedAt *metav1.Time
	if app.Status.SourceHydrator.CurrentOperation != nil {
		hydratedAt = &app.Status.SourceHydrator.CurrentOperation.StartedAt
	}

	if app.IsHydrateRequested() {
		return true, "hydrate requested"
	} else if app.Status.SourceHydrator.CurrentOperation == nil {
		return true, "no previous hydrate operation"
	} else if !app.Spec.SourceHydrator.DeepEquals(app.Status.SourceHydrator.CurrentOperation.SourceHydrator) {
		return true, "spec.sourceHydrator differs"
	} else if app.Status.SourceHydrator.CurrentOperation.Phase == appv1.HydrateOperationPhaseFailed && metav1.Now().Sub(app.Status.SourceHydrator.CurrentOperation.FinishedAt.Time) > 2*time.Minute {
		return true, "previous hydrate operation failed more than 2 minutes ago"
	} else if hydratedAt == nil || hydratedAt.Add(statusHydrateTimeout).Before(time.Now().UTC()) {
		return true, "hydration expired"
	}

	return false, ""
}
