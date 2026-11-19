package hydrator

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
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
	"github.com/argoproj/argo-cd/v3/util/sourceintegrity"
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

	// EvaluateAppRevisionsChanges checks if any source revisions have changes without generating manifests.
	// The returned string is the resolved revision for the given source.
	EvaluateAppRevisionsChanges(ctx context.Context, app *appv1.Application, source appv1.ApplicationSource, revision string, project *appv1.AppProject, noRevisionCache bool) (bool, string, error)

	// GetRepoObjs returns the repository objects for the given application, source, and revision. It calls the repo-
	// server and gets the manifests (objects).
	GetRepoObjs(ctx context.Context, app *appv1.Application, source appv1.ApplicationSource, revision string, project *appv1.AppProject) ([]*unstructured.Unstructured, *apiclient.ManifestResponse, error)

	// GetWriteCredentials returns the repository credentials for the given repository URL and project. These are to be
	// sent to the commit server to write the hydrated manifests.
	GetWriteCredentials(ctx context.Context, repoURL string, project string) (*appv1.Repository, error)

	// RequestAppRefresh requests a refresh of the application with the given name and namespace. This is used to
	// trigger a refresh after the application has been hydrated and a new commit has been pushed.
	RequestAppRefresh(appName string, appNamespace string) error

	// PersistHydrationStatus persists the application status for the source hydrator.
	PersistHydrationStatus(orig *appv1.Application, newStatus *appv1.SourceHydratorStatus)

	// RemoveHydrationAnnotations removes the hydrate and hydrate-timestamp annotations.
	RemoveHydrationAnnotations(orig *appv1.Application)

	// AddHydrationQueueItem adds a hydration queue item to the queue. This is used to trigger the hydration process for
	// a group of applications which are hydrating to the same repo and target branch.
	AddHydrationQueueItem(key types.HydrationQueueKey)

	// GetHydratorCommitMessageTemplate gets the configured template for rendering commit messages.
	GetHydratorCommitMessageTemplate() (string, error)

	// GetHydratorReadmeMessageTemplate gets the configured template for rendering README messages.
	GetHydratorReadmeMessageTemplate() (string, error)

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

// ProcessAppHydrateQueueItem processes an application hydrate queue item. It checks whether the
// application needs hydration and, if so, enqueues the deduped hydration key.
//
// The hydrate and hydrate-timestamp annotations are removed here only if hydration is not needed,
// otherwise the annotations are removed after hydration process has finished.
//
// The per-app status update that marks the application as Hydrating is deliberately NOT done here.
// It is performed by ProcessHydrationQueueItem, which gathers every application sharing the
// hydration key and updates them together. Because the hydration workqueue dedups by key and never
// hands the same key to two workers concurrently, ProcessHydrationQueueItem holds exclusive
// ownership of the entire app group when it runs — there is no possibility of a worker observing a
// partial view of the group, so the status update is safe under parallel hydration workers
// (https://github.com/argoproj/argo-cd/issues/27926).
//
// It's likely that multiple applications will trigger hydration at the same time. The hydration
// queue key is meant to dedupe these requests.
func (h *Hydrator) ProcessAppHydrateQueueItem(origApp *appv1.Application) {
	app := origApp.DeepCopy()
	if app.Spec.SourceHydrator == nil {
		return
	}

	logCtx := log.WithFields(applog.GetAppLogFields(app))
	logCtx.Debug("Processing app hydrate queue item")

	// Workqueue entry point with no inbound request context, so context.Background() is the root.
	ctx := context.Background()
	needsHydration, reason, resolvedDryRevision := h.appNeedsHydration(ctx, app)
	if resolvedDryRevision != "" && resolvedDryRevision != app.Status.SourceHydrator.LastComparedDryRevision {
		// Update the last compared dry revision to the resolved revision
		// If the app is currently hydrating, we should not have a resolvedDryRevision
		app.Status.SourceHydrator.LastComparedDryRevision = resolvedDryRevision
		logCtx.WithField("lastComparedDryRevision", resolvedDryRevision).Debug("Updated last compared dry revision")
		// Persist the only status change we might do here
		h.dependencies.PersistHydrationStatus(origApp, &app.Status.SourceHydrator)
	}

	// needsRefresh re-enqueues the hydration key for an app that was marked Hydrating on an earlier
	// pass but whose StartedAt has aged past statusRefreshTimeout (typically because the hydration
	// worker crashed or fell behind). CurrentOperation can be nil here for an app that has never
	// been hydrated, so the nil guard is required now that we no longer set CurrentOperation above.
	needsRefresh := app.Status.SourceHydrator.CurrentOperation != nil &&
		app.Status.SourceHydrator.CurrentOperation.Phase == appv1.HydrateOperationPhaseHydrating &&
		metav1.Now().Sub(app.Status.SourceHydrator.CurrentOperation.StartedAt.Time) > h.statusRefreshTimeout
	if needsHydration || needsRefresh {
		logCtx.WithField("reason", reason).Info("Hydrating app")
		h.dependencies.AddHydrationQueueItem(getHydrationQueueKey(app))
	} else {
		logCtx.WithField("reason", reason).Debug("Skipping hydration")
		// Consume the hydrate annotation when hydration is not needed.
		if reason != reasonHydrationOperationAlreadyInProgress {
			// annotations are removed at the end of the running hydration
			h.dependencies.RemoveHydrationAnnotations(origApp)
		}
	}

	logCtx.Debug("Successfully processed app hydrate queue item")
}

func getHydrationQueueKey(app *appv1.Application) types.HydrationQueueKey {
	hydrateToSource := app.Spec.GetHydrateToSource()
	destinationRepoURL := git.NormalizeGitURLAllowInvalid(hydrateToSource.RepoURL)
	if hydrateToSource.IsOCI() {
		// The git normalization strips a ".git" suffix, which is a legitimate part of an OCI repository name.
		destinationRepoURL = strings.ToLower(strings.TrimSpace(hydrateToSource.RepoURL))
	}
	key := types.HydrationQueueKey{
		SourceRepoURL:        git.NormalizeGitURLAllowInvalid(app.Spec.SourceHydrator.DrySource.RepoURL),
		SourceTargetRevision: app.Spec.SourceHydrator.DrySource.TargetRevision,
		DestinationRepoURL:   destinationRepoURL,
		DestinationBranch:    hydrateToSource.TargetRevision,
	}
	return key
}

// ProcessHydrationQueueItem processes a hydration queue item. It retrieves the relevant applications for the given
// hydration key, marks every app in the group as Hydrating, generates and commits their manifests, and updates each
// app's status accordingly. If the hydration fails, it marks the operation as failed and logs the error. If successful,
// it updates the operation to indicate that hydration was successful and requests a refresh of the applications to pick
// up the new hydrated commit.
//
// The hydration workqueue is a rate-limiting queue keyed by hydration key, which guarantees the same key is never
// handed to two workers at once. So at the start of this function we hold exclusive ownership over the entire app
// group sharing this key. That ownership is what makes the per-app status updates safe even when multiple hydration
// workers are running in parallel (https://github.com/argoproj/argo-cd/issues/27926): there is no possibility of a
// worker observing a partial view of the group, so we can mark every app Hydrating up front and keep their statuses in
// lockstep with the single commit produced by hydrate().
func (h *Hydrator) ProcessHydrationQueueItem(hydrationKey types.HydrationQueueKey) {
	logCtx := log.WithFields(log.Fields{
		"sourceRepoURL":        hydrationKey.SourceRepoURL,
		"sourceTargetRevision": hydrationKey.SourceTargetRevision,
		"destinationRepoURL":   hydrationKey.DestinationRepoURL,
		"destinationBranch":    hydrationKey.DestinationBranch,
	})

	// Get all applications sharing the same hydration key
	apps, conflictingApps, err := h.getAppsForHydrationKey(hydrationKey)
	if err != nil {
		// If we get an error here, we cannot proceed with hydration and we do not know
		// which apps to update with the failure. The best we can do is log an error in
		// the controller and wait for statusRefreshTimeout to retry
		logCtx.WithError(err).Error("failed to get apps for hydration")
		return
	}
	logCtx.WithField("appCount", len(apps))

	// Atomically mark every app in this group as Hydrating before doing any work. The workqueue's
	// per-key dedup means no other worker can be touching this group concurrently, so it is safe to
	// do the status writes here rather than in ProcessAppHydrateQueueItem
	// (https://github.com/argoproj/argo-cd/issues/27926).
	h.markAppsHydrating(apps)

	// validate all the applications to make sure they are all correctly configured.
	// All applications sharing the same hydration key must succeed for the hydration to be processed.
	projects, validationErrors := h.validateApplications(apps)
	if len(validationErrors) == 0 {
		validationErrors = h.validateTagOwnership(apps, conflictingApps)
	}
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
			h.dependencies.RemoveHydrationAnnotations(app)
		}
		return
	}

	// Hydrate all the apps. ProcessHydrationQueueItem is a workqueue entry point with no inbound
	// request context, so context.Background() is the root of this operation's context tree.
	ctx := context.Background()
	drySHA, hydratedSHA, appErrors, err := h.hydrate(ctx, logCtx, apps, projects)
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
				// markAppsHydrating ran before hydrate(), so CurrentOperation is always populated here.
				app.Status.SourceHydrator.CurrentOperation.DrySHA = drySHA
				app.Status.SourceHydrator.LastComparedDryRevision = drySHA
			}
			if err, ok := appErrors[app.QualifiedName()]; ok {
				logCtx = logCtx.WithFields(applog.GetAppLogFields(app))
				logCtx.Errorf("failed to hydrate app: %v", err)
				h.setAppHydratorError(app, err)
			} else {
				h.setAppHydratorError(app, genericError)
			}
			h.dependencies.RemoveHydrationAnnotations(app)
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
		app.Status.SourceHydrator.LastComparedDryRevision = drySHA
		app.Status.SourceHydrator.LastSuccessfulOperation = &appv1.SuccessfulHydrateOperation{
			DrySHA:         drySHA,
			HydratedSHA:    hydratedSHA,
			SourceHydrator: app.Status.SourceHydrator.CurrentOperation.SourceHydrator,
		}
		h.dependencies.PersistHydrationStatus(origApp, &app.Status.SourceHydrator)
		h.dependencies.RemoveHydrationAnnotations(origApp)

		// Request a refresh since we pushed a new commit.
		err := h.dependencies.RequestAppRefresh(app.Name, app.Namespace)
		if err != nil {
			logCtx.WithFields(applog.GetAppLogFields(app)).WithError(err).Error("Failed to request app refresh after hydration")
		}
	}
}

// markAppsHydrating stamps every app in the group with a Hydrating CurrentOperation and persists
// the change. It is called from ProcessHydrationQueueItem, where the hydration workqueue's per-key
// dedup gives the caller exclusive ownership of the app group — there is no possibility of another
// worker mutating these apps' hydration phase concurrently
// (https://github.com/argoproj/argo-cd/issues/27926).
//
// Apps already in the Hydrating phase are left alone. That happens on a needsRefresh re-entry
// (statusRefreshTimeout elapsed while the previous attempt was in-flight) and also on the very
// first re-entry for a brand-new run where ProcessAppHydrateQueueItem fires twice before the
// hydration worker picks the key up. Preserving the original StartedAt keeps the operation's
// elapsed time meaningful for observers, and avoids needlessly thrashing the persisted status.
func (h *Hydrator) markAppsHydrating(apps []*appv1.Application) {
	now := metav1.Now()
	for _, app := range apps {
		if app.Status.SourceHydrator.CurrentOperation != nil &&
			app.Status.SourceHydrator.CurrentOperation.Phase == appv1.HydrateOperationPhaseHydrating {
			continue
		}
		origApp := app.DeepCopy()
		app.Status.SourceHydrator.CurrentOperation = &appv1.HydrateOperation{
			StartedAt:      now,
			FinishedAt:     nil,
			Phase:          appv1.HydrateOperationPhaseHydrating,
			SourceHydrator: *app.Spec.SourceHydrator,
		}
		h.dependencies.PersistHydrationStatus(origApp, &app.Status.SourceHydrator)
	}
}

// setAppHydratorError updates the CurrentOperation with the error information. It is only called
// from ProcessHydrationQueueItem after markAppsHydrating has ensured every app has a Hydrating
// CurrentOperation, so no nil guard is needed here.
func (h *Hydrator) setAppHydratorError(app *appv1.Application, err error) {
	origApp := app.DeepCopy()
	app.Status.SourceHydrator.CurrentOperation.Phase = appv1.HydrateOperationPhaseFailed
	failedAt := metav1.Now()
	app.Status.SourceHydrator.CurrentOperation.FinishedAt = &failedAt
	app.Status.SourceHydrator.CurrentOperation.Message = fmt.Sprintf("Failed to hydrate: %v", err.Error())
	h.dependencies.PersistHydrationStatus(origApp, &app.Status.SourceHydrator)
}

// getAppsForHydrationKey returns the applications matching the hydration key. It also returns the applications which
// hydrate to the same OCI repository and tag from a different dry source. Pushing a tag replaces the whole artifact,
// so unlike a git branch a tag cannot be shared by applications which are not hydrated together.
func (h *Hydrator) getAppsForHydrationKey(hydrationKey types.HydrationQueueKey) (relevantApps []*appv1.Application, conflictingApps []*appv1.Application, err error) {
	// Get all apps
	apps, err := h.dependencies.GetProcessableApps()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list apps: %w", err)
	}

	for _, app := range apps.Items {
		if app.Spec.SourceHydrator == nil {
			continue
		}
		appKey := getHydrationQueueKey(&app)
		switch {
		case appKey == hydrationKey:
			relevantApps = append(relevantApps, &app)
		case app.Spec.SourceHydrator.IsSyncSourceOCI() &&
			appKey.DestinationRepoURL == hydrationKey.DestinationRepoURL &&
			appKey.DestinationBranch == hydrationKey.DestinationBranch:
			conflictingApps = append(conflictingApps, &app)
		}
	}
	return relevantApps, conflictingApps, nil
}

// validateApplications checks that all applications are valid for hydration.
func (h *Hydrator) validateApplications(apps []*appv1.Application) (map[string]*appv1.AppProject, map[string]error) {
	projects := make(map[string]*appv1.AppProject)
	errors := make(map[string]error)
	type hydrationDestKey struct {
		repoURL string
		path    string
	}
	uniquePaths := make(map[hydrationDestKey]string, len(apps))

	for _, app := range apps {
		if problems := hydrator.ValidateSourceHydrator(app.Spec.SourceHydrator); len(problems) > 0 {
			errors[app.QualifiedName()] = fmt.Errorf("invalid source hydrator configuration: %s", strings.Join(problems, "; "))
			continue
		}

		// Get the project for the app and validate if the app is allowed to use the source.
		// We can't short-circuit this even if we have seen this project before, because we need to verify that this
		// particular app is allowed to use this project.
		proj, err := h.dependencies.GetProcessableAppProj(app)
		if err != nil {
			errors[app.QualifiedName()] = fmt.Errorf("failed to get project %q: %w", app.Spec.Project, err)
			continue
		}
		drySource := app.Spec.SourceHydrator.GetDrySource()
		if !proj.IsSourcePermitted(drySource) {
			errors[app.QualifiedName()] = fmt.Errorf("application repo %s is not permitted in project '%s'", git.SanitizeRepoURL(drySource.RepoURL), proj.Name)
			continue
		}
		projects[app.Spec.Project] = proj

		hydrateToSource := app.Spec.GetHydrateToSource()
		destPath := hydrateToSource.Path
		if IsRootPath(destPath) {
			if !hydrateToSource.IsOCI() {
				// Disallow hydrating to the repository root.
				// Hydrating to root would overwrite or delete files at the top level of the repo,
				// which can break other applications or shared configuration.
				// Every hydrated app must write into a subdirectory instead.
				errors[app.QualifiedName()] = fmt.Errorf("app is configured to hydrate to the repository root (branch %q, path %q) which is not allowed", hydrateToSource.TargetRevision, destPath)
				continue
			}
			// An OCI artifact holds nothing but the hydrated manifests, so the root is a valid destination as long as
			// no other application needs a place in the same artifact.
			if len(apps) > 1 {
				errors[app.QualifiedName()] = fmt.Errorf("app is configured to hydrate to the root of the OCI artifact (tag %q, path %q) which is only allowed when no other application hydrates to the same tag", hydrateToSource.TargetRevision, destPath)
				continue
			}
		}

		if !proj.IsSourcePermitted(hydrateToSource) {
			errors[app.QualifiedName()] = fmt.Errorf("destination repo %s is not permitted in project '%s'", git.SanitizeRepoURL(hydrateToSource.RepoURL), proj.Name)
			continue
		}

		// Clean the path so that equivalent paths like "app", "app/" and "./app" are detected as duplicates.
		destKey := hydrationDestKey{repoURL: hydrateToSource.RepoURL, path: filepath.Clean(destPath)}
		if appName, ok := uniquePaths[destKey]; ok {
			errors[app.QualifiedName()] = fmt.Errorf("app %s hydrator uses the same destination: repo=%s, path=%s", appName, git.SanitizeRepoURL(destKey.repoURL), destKey.path)
			errors[appName] = fmt.Errorf("app %s hydrator uses the same destination: repo=%s, path=%s", app.QualifiedName(), git.SanitizeRepoURL(destKey.repoURL), destKey.path)
			continue
		}
		uniquePaths[destKey] = app.QualifiedName()
	}

	// If there are any errors, return nil for projects to avoid possible partial processing.
	if len(errors) > 0 {
		projects = nil
	}

	return projects, errors
}

// validateTagOwnership checks that no application outside the group hydrates to the OCI tag the group hydrates to.
// conflictingApps are the candidates returned by getAppsForHydrationKey. An application whose project does not permit
// the destination can never push to it, so it is not allowed to block the applications which legitimately own the tag.
func (h *Hydrator) validateTagOwnership(apps []*appv1.Application, conflictingApps []*appv1.Application) map[string]error {
	errors := make(map[string]error)
	if len(apps) == 0 {
		return errors
	}
	for _, conflict := range conflictingApps {
		proj, err := h.dependencies.GetProcessableAppProj(conflict)
		if err != nil || !proj.IsSourcePermitted(conflict.Spec.GetHydrateToSource()) {
			continue
		}
		hydrateToSource := apps[0].Spec.GetHydrateToSource()
		for _, app := range apps {
			errors[app.QualifiedName()] = fmt.Errorf("OCI tag %q of %s is also the hydration destination of application %s, which uses a different dry source repository or revision; a tag can only be hydrated from one dry source", hydrateToSource.TargetRevision, git.SanitizeRepoURL(hydrateToSource.RepoURL), conflict.QualifiedName())
		}
		break
	}
	return errors
}

func (h *Hydrator) hydrate(ctx context.Context, logCtx *log.Entry, apps []*appv1.Application, projects map[string]*appv1.AppProject) (string, string, map[string]error, error) {
	errors := make(map[string]error)
	if len(apps) == 0 {
		return "", "", nil, nil
	}

	// These values are the same for all apps being hydrated together, so just get them from the first app.
	hydrateToSource := apps[0].Spec.GetHydrateToSource()
	destinationRepoURL := hydrateToSource.RepoURL
	// For an OCI repository, the "branch" is the tag the hydrated manifests are pushed to.
	targetBranch := hydrateToSource.TargetRevision
	// FIXME: As a convenience, the commit server will create the syncBranch if it does not exist. If the
	// targetBranch does not exist, it will create it based on the syncBranch. On the next line, we take
	// the `syncBranch` from the first app and assume that they're all configured the same. Instead, if any
	// app has a different syncBranch, we should send the commit server an empty string and allow it to
	// create the targetBranch as an orphan since we can't reliable determine a reasonable base.
	syncBranch := apps[0].Spec.SourceHydrator.SyncSource.TargetBranch
	if hydrateToSource.IsOCI() {
		// A tag has no base to be created from.
		syncBranch = ""
	}
	drySourceRepoURL := apps[0].Spec.SourceHydrator.DrySource.RepoURL

	// Get a static SHA revision from the first app so that all apps are hydrated from the same revision.
	targetRevision, pathDetails, err := h.getManifests(ctx, apps[0], "", projects[apps[0].Spec.Project])
	if err != nil {
		errors[apps[0].QualifiedName()] = fmt.Errorf("failed to get manifests: %w", err)
		return "", "", errors, nil
	}
	paths := []*commitclient.PathDetails{pathDetails}
	logCtx = logCtx.WithFields(log.Fields{"drySha": targetRevision})
	// De-dupe, if the drySha was already hydrated log a debug and return using the data from the last successful hydration run.
	// For git, we only inspect one app. If apps have been added/removed, that will be handled on the next DRY commit.
	if lastSuccessful := apps[0].Status.SourceHydrator.LastSuccessfulOperation; lastSuccessful != nil && targetRevision == lastSuccessful.DrySHA {
		// Pushing an OCI tag replaces the whole artifact, and the tag is expected to exist as soon as it is
		// configured. So one application which was added to the tag or reconfigured since it was last hydrated
		// (e.g. a new path, a new tag, or a move from git to OCI) means the manifests must be pushed even though
		// the DRY commit is the same.
		if !hydrateToSource.IsOCI() || allHydratedTo(apps, targetRevision, lastSuccessful.HydratedSHA) {
			logCtx.Debug("Skipping hydration since the DRY commit was already hydrated")
			return targetRevision, lastSuccessful.HydratedSHA, nil, nil
		}
	}

	// NB: use a distinct name for the errgroup-derived context. errgroup cancels it as soon as
	// Wait() returns, so it must NOT clobber the operation ctx used by the calls after Wait()
	// (getRevisionMetadata/GetWriteCredentials/CommitHydratedManifests) - otherwise those run on a
	// canceled context and hydration fails.
	eg, egCtx := errgroup.WithContext(ctx)
	var mu sync.Mutex

	for _, app := range apps[1:] {
		eg.Go(func() error {
			// Use goroutine-local variables here. Assigning to the function-scoped pathDetails/err
			// from multiple errgroup goroutines is a data race (and can append the wrong path under
			// the mutex). See https://github.com/argoproj/argo-cd/issues/27926.
			_, pathDetails, err := h.getManifests(egCtx, app, targetRevision, projects[app.Spec.Project])
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
	revisionMetadata, err := h.getRevisionMetadata(ctx, drySourceRepoURL, project, targetRevision)
	if err != nil {
		return targetRevision, "", errors, fmt.Errorf("failed to get revision metadata for %q: %w", targetRevision, err)
	}

	repo, err := h.dependencies.GetWriteCredentials(ctx, destinationRepoURL, project)
	if err != nil {
		return targetRevision, "", errors, fmt.Errorf("failed to get hydrator credentials: %w", err)
	}
	if repo == nil {
		// Try without credentials.
		repo = &appv1.Repository{
			Repo: destinationRepoURL,
		}
		logCtx.Warn("no credentials found for repo, continuing without credentials")
	}
	// get the commit message template
	commitMessageTemplate, err := h.dependencies.GetHydratorCommitMessageTemplate()
	if err != nil {
		return targetRevision, "", errors, fmt.Errorf("failed to get hydrated commit message template: %w", err)
	}
	commitMessage, errMsg := getTemplatedCommitMessage(drySourceRepoURL, targetRevision, commitMessageTemplate, revisionMetadata)
	if errMsg != nil {
		return targetRevision, "", errors, fmt.Errorf("failed to get hydrator commit templated message: %w", errMsg)
	}

	// get the readme message template
	readmeTemplate, err := h.dependencies.GetHydratorReadmeMessageTemplate()
	if err != nil {
		return targetRevision, "", errors, fmt.Errorf("failed to get hydrated readme message template: %w", err)
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
		ReadmeMessage:     readmeTemplate,
		AuthorName:        authorName,
		AuthorEmail:       authorEmail,
		DryRepoURL:        drySourceRepoURL,
	}

	closer, commitService, err := h.commitClientset.NewCommitServerClient()
	if err != nil {
		return targetRevision, "", errors, fmt.Errorf("failed to create commit service: %w", err)
	}
	defer utilio.Close(closer)
	resp, err := commitService.CommitHydratedManifests(ctx, &manifestsRequest)
	if err != nil {
		return targetRevision, "", errors, fmt.Errorf("failed to commit hydrated manifests: %w", err)
	}
	return targetRevision, resp.HydratedSha, errors, nil
}

// allHydratedTo returns true if the last successful hydration of every application was a hydration of the given DRY
// commit to the given revision, with the source hydrator configuration the application has now.
func allHydratedTo(apps []*appv1.Application, drySHA, hydratedSHA string) bool {
	for _, app := range apps {
		lastSuccessful := app.Status.SourceHydrator.LastSuccessfulOperation
		if lastSuccessful == nil ||
			lastSuccessful.DrySHA != drySHA ||
			lastSuccessful.HydratedSHA != hydratedSHA ||
			!lastSuccessful.SourceHydrator.DeepEquals(*app.Spec.SourceHydrator) {
			return false
		}
	}
	return true
}

// getManifests gets the manifests for the given application and target revision. It returns the resolved revision
// (a git SHA), and path details for the commit server.
//
// If the given target revision is empty, it uses the target revision from the app dry source spec.
func (h *Hydrator) getManifests(ctx context.Context, app *appv1.Application, targetRevision string, project *appv1.AppProject) (revision string, pathDetails *commitclient.PathDetails, err error) {
	drySource := app.Spec.SourceHydrator.GetDrySource()
	if targetRevision == "" {
		targetRevision = drySource.TargetRevision
	}

	objs, resp, err := h.dependencies.GetRepoObjs(ctx, app, drySource, targetRevision, project)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get repo objects for app %q: %w", app.QualifiedName(), err)
	}

	if si := project.EffectiveSourceIntegrity(); sourceintegrity.HasCriteria(si, drySource) {
		if resp.SourceIntegrityResult == nil {
			return "", nil, fmt.Errorf("source integrity verification required but not performed for app %q dry revision %q", app.QualifiedName(), resp.Revision)
		}
		if err := resp.SourceIntegrityResult.AsError(); err != nil {
			return "", nil, fmt.Errorf("source integrity verification failed for app %q dry revision %q: %w", app.QualifiedName(), resp.Revision, err)
		}
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
		return nil, fmt.Errorf("failed to get repository %q: %w", git.SanitizeRepoURL(repoURL), err)
	}

	closer, repoService, err := h.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create commit service: %w", err)
	}
	defer utilio.Close(closer)

	resp, err := repoService.GetRevisionMetadata(ctx, &apiclient.RepoServerRevisionMetadataRequest{
		Repo:     repo,
		Revision: revision,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get revision metadata: %w", err)
	}
	return resp, nil
}

// newRevisionHasChanges checks if the dry source has a new revision that differs from the last compared dry revision.
// Returns true if the new revision may contain changes that would affect the hydrated manifests, the resolved
// revision from evaluation (empty if evaluation was skipped), and any error encountered.
func (h *Hydrator) newRevisionHasChanges(ctx context.Context, app *appv1.Application, noRevisionCache bool) (bool, string, error) {
	if app.Status.SourceHydrator.LastComparedDryRevision == "" {
		log.WithFields(applog.GetAppLogFields(app)).Debug("No LastComparedDryRevision, hydration needed")
		return true, "", nil
	}

	project, err := h.dependencies.GetProcessableAppProj(app)
	if err != nil {
		return false, "", fmt.Errorf("failed to get project for revision check: %w", err)
	}

	drySource := app.Spec.SourceHydrator.GetDrySource()
	hasChanges, resolvedRev, err := h.dependencies.EvaluateAppRevisionsChanges(ctx, app, drySource, drySource.TargetRevision, project, noRevisionCache)
	if err != nil {
		return false, "", fmt.Errorf("failed to evaluate app revisions changes: %w", err)
	}

	return hasChanges, resolvedRev, nil
}

const reasonHydrationOperationAlreadyInProgress = "hydration operation already in progress"

// appNeedsHydration answers if application needs manifests hydrated. The third return value is the resolved dry
// source revision from revision evaluation (empty if evaluation was skipped or failed).
func (h *Hydrator) appNeedsHydration(ctx context.Context, app *appv1.Application) (needsHydration bool, reason string, resolvedDryRevision string) {
	requested, hydrateType := app.IsHydrateRequested()
	noRevisionCache := requested

	switch {
	case app.Spec.SourceHydrator == nil:
		return false, "source hydrator not configured", ""
	case app.Status.SourceHydrator.CurrentOperation == nil:
		return true, "no previous hydrate operation", ""
	case app.Status.SourceHydrator.CurrentOperation.Phase == appv1.HydrateOperationPhaseHydrating:
		return false, reasonHydrationOperationAlreadyInProgress, ""
	case requested && hydrateType == appv1.HydrateTypeHard:
		return true, "hard hydrate requested", ""
	case !app.Spec.SourceHydrator.DeepEquals(app.Status.SourceHydrator.CurrentOperation.SourceHydrator):
		return true, "spec.sourceHydrator differs", ""
	case app.Status.SourceHydrator.CurrentOperation.Phase == appv1.HydrateOperationPhaseFailed:
		finishedAt := app.Status.SourceHydrator.CurrentOperation.FinishedAt
		withinCooldown := finishedAt == nil || metav1.Now().Sub(finishedAt.Time) <= 2*time.Minute
		switch {
		case requested:
			// Manual/API refresh or dry-source webhook explicitly asks to hydrate;
			// retry now (hard requests are handled by the earlier case).
			return true, "retrying previous failed hydration", ""
		case !withinCooldown:
			return true, "previous hydrate operation failed more than 2 minutes ago", ""
		case app.Status.SourceHydrator.LastComparedDryRevision == "":
			// No baseline to diff against; revision evaluation would always report
			// "needs hydration", causing a tight retry loop. Wait out the cooldown.
			return false, "previous hydrate operation failed", ""
		}
		// Within cooldown with a baseline: fall through to newRevisionHasChanges so a
		// new dry commit retries immediately while an unchanged revision stays idle.
	}

	// Check for new revision changes
	hasChanges, resolvedRev, err := h.newRevisionHasChanges(ctx, app, noRevisionCache)
	if err != nil {
		log.WithFields(applog.GetAppLogFields(app)).WithError(err).Warn("Failed to check for new revision changes")
		return false, "cannot determine if hydration is needed", resolvedRev
	}
	if hasChanges {
		return true, "new revision may have changes", resolvedRev
	}

	return false, "hydration not needed", resolvedRev
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
