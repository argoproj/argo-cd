package hydrator

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	commitclient "github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v3/controller/hydrator/types"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	applog "github.com/argoproj/argo-cd/v3/util/app/log"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/hydrator"
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
	GetRepoObjs(ctx context.Context, app *appv1.Application, source appv1.ApplicationSource, revision string, project *appv1.AppProject) ([]*unstructured.Unstructured, *apiclient.ManifestResponse, error)

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

	// GetHydratorCommitMessageTemplate gets the configured template for rendering commit messages.
	GetHydratorCommitMessageTemplate() (string, error)

	// GetCommitAuthorName gets the configured commit author name from argocd-cm ConfigMap.
	GetCommitAuthorName() (string, error)

	// GetCommitAuthorEmail gets the configured commit author email from argocd-cm ConfigMap.
	GetCommitAuthorEmail() (string, error)
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
	app := origApp.DeepCopy()
	if app.Spec.SourceHydrator == nil {
		return
	}

	logCtx := log.WithFields(applog.GetAppLogFields(app))
	logCtx.Debug("Processing app hydrate queue item")

	needsHydration, reason := appNeedsHydration(app)
	if needsHydration {
		app.Status.SourceHydrator.CurrentOperation = &appv1.HydrateOperation{
			StartedAt:      metav1.Now(),
			FinishedAt:     nil,
			Phase:          appv1.HydrateOperationPhaseHydrating,
			SourceHydrator: *app.Spec.SourceHydrator,
		}
		h.dependencies.PersistAppHydratorStatus(origApp, &app.Status.SourceHydrator)
	}

	needsRefresh := app.Status.SourceHydrator.CurrentOperation.Phase == appv1.HydrateOperationPhaseHydrating && metav1.Now().Sub(app.Status.SourceHydrator.CurrentOperation.StartedAt.Time) > h.statusRefreshTimeout
	if needsHydration || needsRefresh {
		logCtx.WithField("reason", reason).Info("Hydrating app")
		h.dependencies.AddHydrationQueueItem(getHydrationQueueKey(app))
	} else {
		logCtx.WithField("reason", reason).Debug("Skipping hydration")
	}

	logCtx.Debug("Successfully processed app hydrate queue item")
}

func getHydrationQueueKey(app *appv1.Application) types.HydrationQueueKey {
	key := types.HydrationQueueKey{
		SourceRepoURL:        git.NormalizeGitURLAllowInvalid(app.Spec.SourceHydrator.DrySource.RepoURL),
		SourceTargetRevision: app.Spec.SourceHydrator.DrySource.TargetRevision,
		DestinationBranch:    app.Spec.GetHydrateToSource().TargetRevision,
	}
	return key
}

// ProcessHydrationQueueItem processes a hydration queue item. It retrieves the relevant applications for the given
// hydration key, hydrates their latest commit, and updates their status accordingly. If the hydration fails, it marks
// the operation as failed and logs the error. If successful, it updates the operation to indicate that hydration was
// successful and requests a refresh of the applications to pick up the new hydrated commit.
func (h *Hydrator) ProcessHydrationQueueItem(hydrationKey types.HydrationQueueKey) {
	logCtx := log.WithFields(log.Fields{
		"sourceRepoURL":        hydrationKey.SourceRepoURL,
		"sourceTargetRevision": hydrationKey.SourceTargetRevision,
		"destinationBranch":    hydrationKey.DestinationBranch,
	})

	// Get all applications sharing the same hydration key
	apps, err := h.getAppsForHydrationKey(hydrationKey)
	if err != nil {
		// If we get an error here, we cannot proceed with hydration and we do not know
		// which apps to update with the failure. The best we can do is log an error in
		// the controller and wait for statusRefreshTimeout to retry
		logCtx.WithError(err).Error("failed to get apps for hydration")
		return
	}
	logCtx.WithField("appCount", len(apps))

	// FIXME: we might end up in a race condition here where an HydrationQueueItem is processed
	// before all applications had their CurrentOperation set by ProcessAppHydrateQueueItem.
	// This would cause this method to update "old" CurrentOperation.
	// It should only start hydration if all apps are in the HydrateOperationPhaseHydrating phase.
	raceDetected := false
	for _, app := range apps {
		if app.Status.SourceHydrator.CurrentOperation == nil || app.Status.SourceHydrator.CurrentOperation.Phase != appv1.HydrateOperationPhaseHydrating {
			raceDetected = true
			break
		}
	}
	if raceDetected {
		logCtx.Warn("race condition detected: not all apps are in HydrateOperationPhaseHydrating phase")
	}

	// validate all the applications to make sure they are all correctly configured.
	// All applications sharing the same hydration key must succeed for the hydration to be processed.
	projects, validationErrors := h.validateApplications(apps)
	if len(validationErrors) > 0 {
		// For the applications that have an error, set the specific error in their status.
		// Applications without error will still fail with a generic error since the hydration cannot be partial
		genericError := genericHydrationError(validationErrors)
		for _, app := range apps {
			if err, ok := validationErrors[app.QualifiedName()]; ok {
				logCtx = logCtx.WithFields(applog.GetAppLogFields(app))
				logCtx.Errorf("failed to validate hydration app: %v", err)
				h.setAppHydratorError(app, err)
			} else {
				h.setAppHydratorError(app, genericError)
			}
		}
		return
	}

	// Hydrate all the apps
	drySHA, hydratedSHA, appErrors, err := h.hydrate(logCtx, apps, projects)
	if err != nil {
		// If there is a single error, it affects each applications
		for i := range apps {
			appErrors[apps[i].QualifiedName()] = err
		}
	}
	if drySHA != "" {
		logCtx = logCtx.WithField("drySHA", drySHA)
	}
	if len(appErrors) > 0 {
		// For the applications that have an error, set the specific error in their status.
		// Applications without error will still fail with a generic error since the hydration cannot be partial
		genericError := genericHydrationError(appErrors)
		for _, app := range apps {
			if drySHA != "" {
				// If we have a drySHA, we can set it on the app status
				app.Status.SourceHydrator.CurrentOperation.DrySHA = drySHA
			}
			if err, ok := appErrors[app.QualifiedName()]; ok {
				logCtx = logCtx.WithFields(applog.GetAppLogFields(app))
				logCtx.Errorf("failed to hydrate app: %v", err)
				h.setAppHydratorError(app, err)
			} else {
				h.setAppHydratorError(app, genericError)
			}
		}
		return
	}

	logCtx.Debug("Successfully hydrated apps")
	finishedAt := metav1.Now()
	for _, app := range apps {
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
			logCtx.WithFields(applog.GetAppLogFields(app)).WithError(err).Error("Failed to request app refresh after hydration")
		}
	}
}

// setAppHydratorError updates the CurrentOperation with the error information.
func (h *Hydrator) setAppHydratorError(app *appv1.Application, err error) {
	// if the operation is not in progress, we do not update the status
	if app.Status.SourceHydrator.CurrentOperation.Phase != appv1.HydrateOperationPhaseHydrating {
		return
	}

	origApp := app.DeepCopy()
	app.Status.SourceHydrator.CurrentOperation.Phase = appv1.HydrateOperationPhaseFailed
	failedAt := metav1.Now()
	app.Status.SourceHydrator.CurrentOperation.FinishedAt = &failedAt
	app.Status.SourceHydrator.CurrentOperation.Message = fmt.Sprintf("Failed to hydrate: %v", err.Error())
	h.dependencies.PersistAppHydratorStatus(origApp, &app.Status.SourceHydrator)
}

// getAppsForHydrationKey returns the applications matching the hydration key.
func (h *Hydrator) getAppsForHydrationKey(hydrationKey types.HydrationQueueKey) ([]*appv1.Application, error) {
	// Get all apps
	apps, err := h.dependencies.GetProcessableApps()
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	var relevantApps []*appv1.Application
	for _, app := range apps.Items {
		if app.Spec.SourceHydrator == nil {
			continue
		}
		appKey := getHydrationQueueKey(&app)
		if appKey != hydrationKey {
			continue
		}
		relevantApps = append(relevantApps, &app)
	}
	return relevantApps, nil
}

// validateApplications checks that all applications are valid for hydration.
func (h *Hydrator) validateApplications(apps []*appv1.Application) (map[string]*appv1.AppProject, map[string]error) {
	projects := make(map[string]*appv1.AppProject)
	errors := make(map[string]error)
	uniquePaths := make(map[string]string, len(apps))

	for _, app := range apps {
		// Get the project for the app and validate if the app is allowed to use the source.
		// We can't short-circuit this even if we have seen this project before, because we need to verify that this
		// particular app is allowed to use this project.
		proj, err := h.dependencies.GetProcessableAppProj(app)
		if err != nil {
			errors[app.QualifiedName()] = fmt.Errorf("failed to get project %q: %w", app.Spec.Project, err)
			continue
		}
		permitted := proj.IsSourcePermitted(app.Spec.GetSource())
		if !permitted {
			errors[app.QualifiedName()] = fmt.Errorf("application repo %s is not permitted in project '%s'", app.Spec.GetSource().RepoURL, proj.Name)
			continue
		}
		projects[app.Spec.Project] = proj

		// Disallow hydrating to the repository root.
		// Hydrating to root would overwrite or delete files at the top level of the repo,
		// which can break other applications or shared configuration.
		// Every hydrated app must write into a subdirectory instead.
		destPath := app.Spec.SourceHydrator.SyncSource.Path
		if IsRootPath(destPath) {
			errors[app.QualifiedName()] = fmt.Errorf("app is configured to hydrate to the repository root (branch %q, path %q) which is not allowed", app.Spec.GetHydrateToSource().TargetRevision, destPath)
			continue
		}

		// TODO: test the dupe detection
		// TODO: normalize the path to avoid "path/.." from being treated as different from "."
		if appName, ok := uniquePaths[destPath]; ok {
			errors[app.QualifiedName()] = fmt.Errorf("app %s hydrator use the same destination: %v", appName, app.Spec.SourceHydrator.SyncSource.Path)
			errors[appName] = fmt.Errorf("app %s hydrator use the same destination: %v", app.QualifiedName(), app.Spec.SourceHydrator.SyncSource.Path)
			continue
		}
		uniquePaths[destPath] = app.QualifiedName()
	}

	// If there are any errors, return nil for projects to avoid possible partial processing.
	if len(errors) > 0 {
		projects = nil
	}

	return projects, errors
}

func (h *Hydrator) hydrate(logCtx *log.Entry, apps []*appv1.Application, projects map[string]*appv1.AppProject) (string, string, map[string]error, error) {
	errors := make(map[string]error)
	if len(apps) == 0 {
		return "", "", nil, nil
	}

	// These values are the same for all apps being hydrated together, so just get them from the first app.
	repoURL := apps[0].Spec.GetHydrateToSource().RepoURL
	targetBranch := apps[0].Spec.GetHydrateToSource().TargetRevision
	// FIXME: As a convenience, the commit server will create the syncBranch if it does not exist. If the
	// targetBranch does not exist, it will create it based on the syncBranch. On the next line, we take
	// the `syncBranch` from the first app and assume that they're all configured the same. Instead, if any
	// app has a different syncBranch, we should send the commit server an empty string and allow it to
	// create the targetBranch as an orphan since we can't reliable determine a reasonable base.
	syncBranch := apps[0].Spec.SourceHydrator.SyncSource.TargetBranch

	// Get a static SHA revision from the first app so that all apps are hydrated from the same revision.
	targetRevision, pathDetails, err := h.getManifests(context.Background(), apps[0], "", projects[apps[0].Spec.Project])
	if err != nil {
		errors[apps[0].QualifiedName()] = fmt.Errorf("failed to get manifests: %w", err)
		return "", "", errors, nil
	}
	paths := []*commitclient.PathDetails{pathDetails}
	logCtx = logCtx.WithFields(log.Fields{"drySha": targetRevision})
	// De-dupe, if the drySha was already hydrated log a debug and return using the data from the last successful hydration run.
	// We only inspect one app. If apps have been added/removed, that will be handled on the next DRY commit.
	if apps[0].Status.SourceHydrator.LastSuccessfulOperation != nil && targetRevision == apps[0].Status.SourceHydrator.LastSuccessfulOperation.DrySHA {
		logCtx.Debug("Skipping hydration since the DRY commit was already hydrated")
		return targetRevision, apps[0].Status.SourceHydrator.LastSuccessfulOperation.HydratedSHA, nil, nil
	}

	eg, ctx := errgroup.WithContext(context.Background())
	var mu sync.Mutex

	for _, app := range apps[1:] {
		eg.Go(func() error {
			_, pathDetails, err = h.getManifests(ctx, app, targetRevision, projects[app.Spec.Project])
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors[app.QualifiedName()] = fmt.Errorf("failed to get manifests: %w", err)
				return errors[app.QualifiedName()]
			}
			paths = append(paths, pathDetails)
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return targetRevision, "", errors, nil
	}

	// If all the apps are under the same project, use that project. Otherwise, use an empty string to indicate that we
	// need global creds.
	project := ""
	if len(projects) == 1 {
		for p := range projects {
			project = p
			break
		}
	}

	// Get the commit metadata for the target revision.
	revisionMetadata, err := h.getRevisionMetadata(context.Background(), repoURL, project, targetRevision)
	if err != nil {
		return targetRevision, "", errors, fmt.Errorf("failed to get revision metadata for %q: %w", targetRevision, err)
	}

	repo, err := h.dependencies.GetWriteCredentials(context.Background(), repoURL, project)
	if err != nil {
		return targetRevision, "", errors, fmt.Errorf("failed to get hydrator credentials: %w", err)
	}
	if repo == nil {
		// Try without credentials.
		repo = &appv1.Repository{
			Repo: repoURL,
		}
		logCtx.Warn("no credentials found for repo, continuing without credentials")
	}
	// get the commit message template
	commitMessageTemplate, err := h.dependencies.GetHydratorCommitMessageTemplate()
	if err != nil {
		return targetRevision, "", errors, fmt.Errorf("failed to get hydrated commit message template: %w", err)
	}
	commitMessage, errMsg := getTemplatedCommitMessage(repoURL, targetRevision, commitMessageTemplate, revisionMetadata)
	if errMsg != nil {
		return targetRevision, "", errors, fmt.Errorf("failed to get hydrator commit templated message: %w", errMsg)
	}

	// get commit author configuration from argocd-cm
	authorName, err := h.dependencies.GetCommitAuthorName()
	if err != nil {
		return targetRevision, "", errors, fmt.Errorf("failed to get commit author name: %w", err)
	}
	authorEmail, err := h.dependencies.GetCommitAuthorEmail()
	if err != nil {
		return targetRevision, "", errors, fmt.Errorf("failed to get commit author email: %w", err)
	}

	manifestsRequest := commitclient.CommitHydratedManifestsRequest{
		Repo:              repo,
		SyncBranch:        syncBranch,
		TargetBranch:      targetBranch,
		DrySha:            targetRevision,
		CommitMessage:     commitMessage,
		Paths:             paths,
		DryCommitMetadata: revisionMetadata,
		AuthorName:        authorName,
		AuthorEmail:       authorEmail,
	}

	closer, commitService, err := h.commitClientset.NewCommitServerClient()
	if err != nil {
		return targetRevision, "", errors, fmt.Errorf("failed to create commit service: %w", err)
	}
	defer utilio.Close(closer)
	resp, err := commitService.CommitHydratedManifests(context.Background(), &manifestsRequest)
	if err != nil {
		return targetRevision, "", errors, fmt.Errorf("failed to commit hydrated manifests: %w", err)
	}
	return targetRevision, resp.HydratedSha, errors, nil
}

// getManifests gets the manifests for the given application and target revision. It returns the resolved revision
// (a git SHA), and path details for the commit server.
//
// If the given target revision is empty, it uses the target revision from the app dry source spec.
func (h *Hydrator) getManifests(ctx context.Context, app *appv1.Application, targetRevision string, project *appv1.AppProject) (revision string, pathDetails *commitclient.PathDetails, err error) {
	drySource := appv1.ApplicationSource{
		RepoURL:        app.Spec.SourceHydrator.DrySource.RepoURL,
		Path:           app.Spec.SourceHydrator.DrySource.Path,
		TargetRevision: app.Spec.SourceHydrator.DrySource.TargetRevision,
		Helm:           app.Spec.SourceHydrator.DrySource.Helm,
		Kustomize:      app.Spec.SourceHydrator.DrySource.Kustomize,
		Directory:      app.Spec.SourceHydrator.DrySource.Directory,
		Plugin:         app.Spec.SourceHydrator.DrySource.Plugin,
	}
	if targetRevision == "" {
		targetRevision = app.Spec.SourceHydrator.DrySource.TargetRevision
	}

	// TODO: enable signature verification
	objs, resp, err := h.dependencies.GetRepoObjs(ctx, app, drySource, targetRevision, project)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get repo objects for app %q: %w", app.QualifiedName(), err)
	}

	// Set up a ManifestsRequest
	manifestDetails := make([]*commitclient.HydratedManifestDetails, len(objs))
	for i, obj := range objs {
		objJSON, err := json.Marshal(obj)
		if err != nil {
			return "", nil, fmt.Errorf("failed to marshal object: %w", err)
		}
		manifestDetails[i] = &commitclient.HydratedManifestDetails{ManifestJSON: string(objJSON)}
	}

	return resp.Revision, &commitclient.PathDetails{
		Path:      app.Spec.SourceHydrator.SyncSource.Path,
		Manifests: manifestDetails,
		Commands:  resp.Commands,
	}, nil
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
func appNeedsHydration(app *appv1.Application) (needsHydration bool, reason string) {
	switch {
	case app.Spec.SourceHydrator == nil:
		return false, "source hydrator not configured"
	case app.Status.SourceHydrator.CurrentOperation == nil:
		return true, "no previous hydrate operation"
	case app.Status.SourceHydrator.CurrentOperation.Phase == appv1.HydrateOperationPhaseHydrating:
		return false, "hydration operation already in progress"
	case app.IsHydrateRequested():
		return true, "hydrate requested"
	case !app.Spec.SourceHydrator.DeepEquals(app.Status.SourceHydrator.CurrentOperation.SourceHydrator):
		return true, "spec.sourceHydrator differs"
	case app.Status.SourceHydrator.CurrentOperation.Phase == appv1.HydrateOperationPhaseFailed && metav1.Now().Sub(app.Status.SourceHydrator.CurrentOperation.FinishedAt.Time) > 2*time.Minute:
		return true, "previous hydrate operation failed more than 2 minutes ago"
	}

	return false, "hydration not needed"
}

// getTemplatedCommitMessage gets the multi-line commit message based on the template defined in the configmap. It is a two step process:
// 1. Get the metadata template engine would use to render the template
// 2. Pass the output of Step 1 and Step 2 to template Render
func getTemplatedCommitMessage(repoURL, revision, commitMessageTemplate string, dryCommitMetadata *appv1.RevisionMetadata) (string, error) {
	hydratorCommitMetadata, err := hydrator.GetCommitMetadata(repoURL, revision, dryCommitMetadata)
	if err != nil {
		return "", fmt.Errorf("failed to get hydrated commit message: %w", err)
	}
	templatedCommitMsg, err := hydrator.Render(commitMessageTemplate, hydratorCommitMetadata)
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", commitMessageTemplate, err)
	}
	return templatedCommitMsg, nil
}

// genericHydrationError returns an error that summarizes the hydration errors for all applications.
func genericHydrationError(validationErrors map[string]error) error {
	if len(validationErrors) == 0 {
		return nil
	}

	keys := slices.Sorted(maps.Keys(validationErrors))
	remainder := "has an error"
	if len(keys) > 1 {
		remainder = fmt.Sprintf("and %d more have errors", len(keys)-1)
	}
	return fmt.Errorf("cannot hydrate because application %s %s", keys[0], remainder)
}

// IsRootPath returns whether the path references a root path
func IsRootPath(path string) bool {
	clean := filepath.Clean(path)
	return clean == "" || clean == "." || clean == string(filepath.Separator)
}
