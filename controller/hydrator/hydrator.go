package hydrator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	commitclient "github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v3/controller/hydrator/types"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	applog "github.com/argoproj/argo-cd/v3/util/app/log"
	"github.com/argoproj/argo-cd/v3/util/git"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

// RepoGetter is an interface that defines methods for getting repository objects. It's a subset of the DB interface to
// avoid granting access to things we don't need.
type RepoGetter interface {
	// GetRepository returns a repository by its URL and project name.
	GetRepository(ctx context.Context, repoURL, project string) (*appv1.Repository, error)
}

// Dependencies is the interface for the dependencies of the Hydrator. It serves two purposes: 1) it prevents the
// hydrator from having direct access to the app controller, and 2) it allows for easy mocking of dependencies in tests.
// If you add something here, be sure that it is something the app controller needs to provide to the hydrator.
type Dependencies interface {
	// TODO: determine if we actually need to get the app, or if all the stuff we need the app for is done already on
	//       the app controller side.

	// GetProcessableAppProj returns the AppProject for the given application. It should only return projects that are
	// processable by the controller, meaning that the project is not deleted and the application is in a namespace
	// permitted by the project.
	GetProcessableAppProj(app *appv1.Application) (*appv1.AppProject, error)

	// GetProcessableApps returns a list of applications that are processable by the controller.
	GetProcessableApps() (*appv1.ApplicationList, error)

	// GetRepoObjs returns the repository objects for the given application, source, and revision. It calls the repo-
	// server and gets the manifests (objects).
	GetRepoObjs(app *appv1.Application, source appv1.ApplicationSource, revision string, project *appv1.AppProject) ([]*unstructured.Unstructured, *apiclient.ManifestResponse, error)

	// GetWriteCredentials returns the repository credentials for the given repository URL and project. These are to be
	// sent to the commit server to write the hydrated manifests.
	GetWriteCredentials(ctx context.Context, repoURL string, project string) (*appv1.Repository, error)

	// RequestAppRefresh requests a refresh of the application with the given name and namespace. This is used to
	// trigger a refresh after the application has been hydrated and a new commit has been pushed.
	RequestAppRefresh(appName string, appNamespace string) error

	// PersistAppHydratorStatus persists the application status for the source hydrator.
	PersistAppHydratorStatus(orig *appv1.Application, newStatus *appv1.SourceHydratorStatus)

	// AddHydrationQueueItem adds a hydration queue item to the queue. This is used to trigger the hydration process for
	// a group of applications which are hydrating to the same repo and target branch.
	AddHydrationQueueItem(key types.HydrationQueueKey)
}

// Hydrator is the main struct that implements the hydration logic. It uses the Dependencies interface to access the
// app controller's functionality without directly depending on it.
type Hydrator struct {
	dependencies         Dependencies
	statusRefreshTimeout time.Duration
	commitClientset      commitclient.Clientset
	repoClientset        apiclient.Clientset
	repoGetter           RepoGetter
}

// NewHydrator creates a new Hydrator instance with the given dependencies, status refresh timeout, commit clientset,
// repo clientset, and repo getter. The refresh timeout determines how often the hydrator checks if an application
// needs to be hydrated.
func NewHydrator(dependencies Dependencies, statusRefreshTimeout time.Duration, commitClientset commitclient.Clientset, repoClientset apiclient.Clientset, repoGetter RepoGetter) *Hydrator {
	return &Hydrator{
		dependencies:         dependencies,
		statusRefreshTimeout: statusRefreshTimeout,
		commitClientset:      commitClientset,
		repoClientset:        repoClientset,
		repoGetter:           repoGetter,
	}
}

// ProcessAppHydrateQueueItem processes an application hydrate queue item. It checks if the application needs hydration
// and if so, it updates the application's status to indicate that hydration is in progress. It then adds the
// hydration queue item to the queue for further processing.
//
// It's likely that multiple applications will trigger hydration at the same time. The hydration queue key is meant to
// dedupe these requests.
func (h *Hydrator) ProcessAppHydrateQueueItem(origApp *appv1.Application) {
	origApp = origApp.DeepCopy()
	app := origApp.DeepCopy()

	if app.Spec.SourceHydrator == nil {
		return
	}

	logCtx := log.WithFields(applog.GetAppLogFields(app))

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

func getHydrationQueueKey(app *appv1.Application) types.HydrationQueueKey {
	destinationBranch := app.Spec.SourceHydrator.SyncSource.TargetBranch
	if app.Spec.SourceHydrator.HydrateTo != nil {
		destinationBranch = app.Spec.SourceHydrator.HydrateTo.TargetBranch
	}
	key := types.HydrationQueueKey{
		SourceRepoURL:        git.NormalizeGitURLAllowInvalid(app.Spec.SourceHydrator.DrySource.RepoURL),
		SourceTargetRevision: app.Spec.SourceHydrator.DrySource.TargetRevision,
		DestinationBranch:    destinationBranch,
	}
	return key
}

// uniqueHydrationDestination is used to detect duplicate hydrate destinations.
type uniqueHydrationDestination struct {
	// sourceRepoURL must be normalized with git.NormalizeGitURL to ensure that two apps with different URL formats
	// don't end up in two different hydration queue items. Failing to normalize would result in one hydrated commit for
	// each unique URL.
	//nolint:unused // used as part of a map key
	sourceRepoURL string
	//nolint:unused // used as part of a map key
	sourceTargetRevision string
	//nolint:unused // used as part of a map key
	destinationBranch string
	//nolint:unused // used as part of a map key
	destinationPath string
}

// ProcessHydrationQueueItem processes a hydration queue item. It retrieves the relevant applications for the given
// hydration key, hydrates their latest commit, and updates their status accordingly. If the hydration fails, it marks
// the operation as failed and logs the error. If successful, it updates the operation to indicate that hydration was
// successful and requests a refresh of the applications to pick up the new hydrated commit.
func (h *Hydrator) ProcessHydrationQueueItem(hydrationKey types.HydrationQueueKey) (processNext bool) {
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
			app.Status.SourceHydrator.CurrentOperation.Message = fmt.Sprintf("Failed to hydrate revision %q: %v", drySHA, err.Error())
			// We may or may not have gotten far enough in the hydration process to get a non-empty SHA, but set it just
			// in case we did.
			app.Status.SourceHydrator.CurrentOperation.DrySHA = drySHA
			h.dependencies.PersistAppHydratorStatus(origApp, &app.Status.SourceHydrator)
			logCtx = logCtx.WithFields(applog.GetAppLogFields(app))
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
		err := h.dependencies.RequestAppRefresh(app.Name, app.Namespace)
		if err != nil {
			logCtx.WithField("app", app.QualifiedName()).WithError(err).Error("Failed to request app refresh after hydration")
		}
	}
	return
}

func (h *Hydrator) hydrateAppsLatestCommit(logCtx *log.Entry, hydrationKey types.HydrationQueueKey) ([]*appv1.Application, string, string, error) {
	relevantApps, err := h.getRelevantAppsForHydration(logCtx, hydrationKey)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get relevant apps for hydration: %w", err)
	}

	dryRevision, hydratedRevision, err := h.hydrate(logCtx, relevantApps)
	if err != nil {
		return relevantApps, dryRevision, "", fmt.Errorf("failed to hydrate apps: %w", err)
	}

	return relevantApps, dryRevision, hydratedRevision, nil
}

func (h *Hydrator) getRelevantAppsForHydration(logCtx *log.Entry, hydrationKey types.HydrationQueueKey) ([]*appv1.Application, error) {
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

		if !git.SameURL(app.Spec.SourceHydrator.DrySource.RepoURL, hydrationKey.SourceRepoURL) ||
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
			sourceRepoURL:        git.NormalizeGitURLAllowInvalid(app.Spec.SourceHydrator.DrySource.RepoURL),
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
			return "", "", fmt.Errorf("failed to get repo objects for app %q: %w", app.QualifiedName(), err)
		}

		// This should be the DRY SHA. We set it here so that after processing the first app, all apps are hydrated
		// using the same SHA.
		targetRevision = resp.Revision

		// Set up a ManifestsRequest
		manifestDetails := make([]*commitclient.HydratedManifestDetails, len(objs))
		for i, obj := range objs {
			objJSON, err := json.Marshal(obj)
			if err != nil {
				return "", "", fmt.Errorf("failed to marshal object: %w", err)
			}
			manifestDetails[i] = &commitclient.HydratedManifestDetails{ManifestJSON: string(objJSON)}
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

	// Get the commit metadata for the target revision.
	revisionMetadata, err := h.getRevisionMetadata(context.Background(), repoURL, project, targetRevision)
	if err != nil {
		return "", "", fmt.Errorf("failed to get revision metadata for %q: %w", targetRevision, err)
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
		Repo:              repo,
		SyncBranch:        syncBranch,
		TargetBranch:      targetBranch,
		DrySha:            targetRevision,
		CommitMessage:     "[Argo CD Bot] hydrate " + targetRevision,
		Paths:             paths,
		DryCommitMetadata: revisionMetadata,
	}

	closer, commitService, err := h.commitClientset.NewCommitServerClient()
	if err != nil {
		return targetRevision, "", fmt.Errorf("failed to create commit service: %w", err)
	}
	defer utilio.Close(closer)
	resp, err := commitService.CommitHydratedManifests(context.Background(), &manifestsRequest)
	if err != nil {
		return targetRevision, "", fmt.Errorf("failed to commit hydrated manifests: %w", err)
	}
	return targetRevision, resp.HydratedSha, nil
}

func (h *Hydrator) getRevisionMetadata(ctx context.Context, repoURL, project, revision string) (*appv1.RevisionMetadata, error) {
	repo, err := h.repoGetter.GetRepository(ctx, repoURL, project)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository %q: %w", repoURL, err)
	}

	closer, repoService, err := h.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create commit service: %w", err)
	}
	defer utilio.Close(closer)

	resp, err := repoService.GetRevisionMetadata(context.Background(), &apiclient.RepoServerRevisionMetadataRequest{
		Repo:     repo,
		Revision: revision,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get revision metadata: %w", err)
	}
	return resp, nil
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

	switch {
	case app.IsHydrateRequested():
		return true, "hydrate requested"
	case app.Status.SourceHydrator.CurrentOperation == nil:
		return true, "no previous hydrate operation"
	case !app.Spec.SourceHydrator.DeepEquals(app.Status.SourceHydrator.CurrentOperation.SourceHydrator):
		return true, "spec.sourceHydrator differs"
	case app.Status.SourceHydrator.CurrentOperation.Phase == appv1.HydrateOperationPhaseFailed && metav1.Now().Sub(app.Status.SourceHydrator.CurrentOperation.FinishedAt.Time) > 2*time.Minute:
		return true, "previous hydrate operation failed more than 2 minutes ago"
	case hydratedAt == nil || hydratedAt.Add(statusHydrateTimeout).Before(time.Now().UTC()):
		return true, "hydration expired"
	}

	return false, ""
}
