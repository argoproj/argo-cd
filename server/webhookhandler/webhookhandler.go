package webhookhandler

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	azureDevOps "github.com/go-playground/webhooks/v6/azuredevops"
	"github.com/go-playground/webhooks/v6/bitbucket"
	bitbucketServer "github.com/go-playground/webhooks/v6/bitbucket-server"
	gitHub "github.com/go-playground/webhooks/v6/github"
	gitLab "github.com/go-playground/webhooks/v6/gitlab"
	gogsClient "github.com/gogits/go-gogs-client"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appClientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v2/reposerver/cache"
	serverCache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/util/app/path"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/glob"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/argoproj/argo-cd/v2/util/webhook"
)

type ApplicationWebhookPayloadHandler struct {
	db                db.ArgoDB
	ns                string
	appNs             []string
	appClientset      appClientset.Interface
	repoCache         *cache.Cache
	serverCache       *serverCache.Cache
	argoCdSettingsMgr *settings.SettingsManager
}

type changeInfo struct {
	shaBefore string
	shaAfter  string
}

// affectedRevisionInfo examines a payload from a webhook event, and extracts the repo web URL, the
// revision, and whether or not this affected origin/HEAD (the default branch of the repository).
func affectedRevisionInfo(payloadIf interface{}) (webURLs []string, revision string, change changeInfo, touchedHead bool, changedFiles []string) {
	switch payload := payloadIf.(type) {
	case azureDevOps.GitPushEvent:
		// See: https://learn.microsoft.com/en-us/azure/devops/service-hooks/events?view=azure-devops#git.push
		webURLs = append(webURLs, payload.Resource.Repository.RemoteURL)
		revision = webhook.ParseRevision(payload.Resource.RefUpdates[0].Name)
		change.shaAfter = webhook.ParseRevision(payload.Resource.RefUpdates[0].NewObjectID)
		change.shaBefore = webhook.ParseRevision(payload.Resource.RefUpdates[0].OldObjectID)
		touchedHead = payload.Resource.RefUpdates[0].Name == payload.Resource.Repository.DefaultBranch
		// Unfortunately, Azure DevOps doesn't provide a list of changed files.
	case gitHub.PushPayload:
		// See: https://developer.github.com/v3/activity/events/types/#pushevent
		webURLs = append(webURLs, payload.Repository.HTMLURL)
		revision = webhook.ParseRevision(payload.Ref)
		change.shaAfter = webhook.ParseRevision(payload.After)
		change.shaBefore = webhook.ParseRevision(payload.Before)
		touchedHead = bool(payload.Repository.DefaultBranch == revision)

		for _, commit := range payload.Commits {
			changedFiles = append(changedFiles, commit.Added...)
			changedFiles = append(changedFiles, commit.Modified...)
			changedFiles = append(changedFiles, commit.Removed...)
		}
	case gitLab.PushEventPayload:
		// See: https://docs.gitlab.com/ee/user/project/integrations/webhooks.html
		webURLs = append(webURLs, payload.Project.WebURL)
		revision = webhook.ParseRevision(payload.Ref)
		change.shaAfter = webhook.ParseRevision(payload.After)
		change.shaBefore = webhook.ParseRevision(payload.Before)
		touchedHead = bool(payload.Project.DefaultBranch == revision)

		for _, commit := range payload.Commits {
			changedFiles = append(changedFiles, commit.Added...)
			changedFiles = append(changedFiles, commit.Modified...)
			changedFiles = append(changedFiles, commit.Removed...)
		}
	case gitLab.TagEventPayload:
		// See: https://docs.gitlab.com/ee/user/project/integrations/webhooks.html
		// NOTE: This is untested.
		webURLs = append(webURLs, payload.Project.WebURL)
		revision = webhook.ParseRevision(payload.Ref)
		change.shaAfter = webhook.ParseRevision(payload.After)
		change.shaBefore = webhook.ParseRevision(payload.Before)
		touchedHead = bool(payload.Project.DefaultBranch == revision)

		for _, commit := range payload.Commits {
			changedFiles = append(changedFiles, commit.Added...)
			changedFiles = append(changedFiles, commit.Modified...)
			changedFiles = append(changedFiles, commit.Removed...)
		}
	case bitbucket.RepoPushPayload:
		// See: https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Push
		// NOTE: This is untested.
		webURLs = append(webURLs, payload.Repository.Links.HTML.Href)

		// TODO: Bitbucket includes multiple changes as part of a single event. We only pick the first
		// but need to consider how to handle multiple.
		for _, change := range payload.Push.Changes {
			revision = change.New.Name

			break
		}

		// Not actually sure how to check if the incoming change affected HEAD just by examining the
		// payload alone. To be safe, we just return true and let the controller check for itself.
		touchedHead = true
	case bitbucketServer.RepositoryReferenceChangedPayload:
		// Webhook module does not parse the inner links.
		if payload.Repository.Links != nil {
			for _, l := range payload.Repository.Links["clone"].([]interface{}) {
				link := l.(map[string]interface{})

				if link["name"] == "http" || link["name"] == "ssh" {
					webURLs = append(webURLs, link["href"].(string))
				}
			}
		}

		// TODO: Bitbucket includes multiple changes as part of a single event. We only pick the first
		// but need to consider how to handle multiple.
		for _, change := range payload.Changes {
			revision = webhook.ParseRevision(change.Reference.ID)

			break
		}

		// Not actually sure how to check if the incoming change affected HEAD just by examining the
		// payload alone. To be safe, we just return true and let the controller check for itself.
		touchedHead = true

		// Bitbucket does not include a list of changed files anywhere in its payload so we cannot
		// update changedFiles for this type of payload.
	case gogsClient.PushPayload:
		webURLs = append(webURLs, payload.Repo.HTMLURL)
		revision = webhook.ParseRevision(payload.Ref)
		change.shaAfter = webhook.ParseRevision(payload.After)
		change.shaBefore = webhook.ParseRevision(payload.Before)
		touchedHead = bool(payload.Repo.DefaultBranch == revision)

		for _, commit := range payload.Commits {
			changedFiles = append(changedFiles, commit.Added...)
			changedFiles = append(changedFiles, commit.Modified...)
			changedFiles = append(changedFiles, commit.Removed...)
		}
	}

	return webURLs, revision, change, touchedHead, changedFiles
}

func sourceRevisionHasChanged(source v1alpha1.ApplicationSource, revision string, touchedHead bool) bool {
	targetRev := webhook.ParseRevision(source.TargetRevision)

	if targetRev == "HEAD" || targetRev == "" { // revision is head
		return touchedHead
	}

	targetRevisionHasPrefixList := []string{"refs/heads/", "refs/tags/"}

	for _, prefix := range targetRevisionHasPrefixList {
		if strings.HasPrefix(source.TargetRevision, prefix) {
			return revision == targetRev
		}
	}

	return source.TargetRevision == revision
}

func sourceUsesURL(source v1alpha1.ApplicationSource, webURL string, repoRegexp *regexp.Regexp) bool {
	if !repoRegexp.MatchString(source.RepoURL) {
		log.Debugf("%s does not match %s", source.RepoURL, repoRegexp.String())

		return false
	}

	log.Debugf("%s uses repoURL %s", source.RepoURL, webURL)

	return true
}

func (handler *ApplicationWebhookPayloadHandler) HandlePayload(payload interface{}) {
	webURLs, revision, change, touchedHead, changedFiles := affectedRevisionInfo(payload)

	// NOTE: The webURL does not include the .git extension.

	if len(webURLs) == 0 {
		log.Info("Ignoring webhook event")

		return
	}

	for _, webURL := range webURLs {
		log.Infof("Received push event repo: %s, revision: %s, touchedHead: %v", webURL, revision, touchedHead)
	}

	nsFilter := handler.ns

	if len(handler.appNs) > 0 {
		// Retrieve app from all namespaces
		nsFilter = ""
	}

	appIf := handler.appClientset.ArgoprojV1alpha1().Applications(nsFilter)
	apps, err := appIf.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Warnf("Failed to list applications: %v", err)

		return
	}

	trackingMethod, err := handler.argoCdSettingsMgr.GetTrackingMethod()
	if err != nil {
		log.Warnf("Failed to get trackingMethod: %v", err)

		return
	}

	appInstanceLabelKey, err := handler.argoCdSettingsMgr.GetAppInstanceLabelKey()
	if err != nil {
		log.Warnf("Failed to get appInstanceLabelKey: %v", err)

		return
	}

	// Skip any application that is neither in the control plane's namespace
	// nor in the list of enabled namespaces.
	var filteredApps []v1alpha1.Application

	for _, app := range apps.Items {
		if app.Namespace == handler.ns || glob.MatchStringInList(handler.appNs, app.Namespace, glob.REGEXP) {
			filteredApps = append(filteredApps, app)
		}
	}

	for _, webURL := range webURLs {
		repoRegexp, err := webhook.GetWebUrlRegex(webURL)
		if err != nil {
			log.Warnf("Failed to get repoRegexp: %s", err)

			continue
		}

		for _, app := range filteredApps {
			for _, source := range app.Spec.GetSources() {
				if sourceRevisionHasChanged(source, revision, touchedHead) && sourceUsesURL(source, webURL, repoRegexp) {
					refreshPaths := path.GetAppRefreshPaths(&app)

					if path.AppFilesHaveChanged(refreshPaths, changedFiles) {
						namespacedAppInterface := handler.appClientset.ArgoprojV1alpha1().Applications(app.ObjectMeta.Namespace)
						_, err = argo.RefreshApp(namespacedAppInterface, app.ObjectMeta.Name, v1alpha1.RefreshTypeNormal)
						if err != nil {
							log.Warnf("Failed to refresh app '%s' for controller reprocessing: %v", app.ObjectMeta.Name, err)

							continue
						}

						// No need to refresh multiple times if multiple sources match.
						break
					} else if change.shaBefore != "" && change.shaAfter != "" {
						if err := handler.storePreviouslyCachedManifests(&app, change, trackingMethod, appInstanceLabelKey); err != nil {
							log.Warnf("Failed to store cached manifests of previous revision for app '%s': %v", app.Name, err)
						}
					}
				}
			}
		}
	}
}

func (handler *ApplicationWebhookPayloadHandler) storePreviouslyCachedManifests(app *v1alpha1.Application, change changeInfo, trackingMethod string, appInstanceLabelKey string) error {
	err := argo.ValidateDestination(context.Background(), &app.Spec.Destination, handler.db)
	if err != nil {
		return fmt.Errorf("error validating destination: %w", err)
	}

	var clusterInfo v1alpha1.ClusterInfo

	err = handler.serverCache.GetClusterInfo(app.Spec.Destination.Server, &clusterInfo)
	if err != nil {
		return fmt.Errorf("error getting cluster info: %w", err)
	}

	var sources v1alpha1.ApplicationSources

	if app.Spec.HasMultipleSources() {
		sources = app.Spec.GetSources()
	} else {
		sources = append(sources, app.Spec.GetSource())
	}

	refSources, err := argo.GetRefSources(context.Background(), sources, app.Spec.Project, handler.db.GetRepository, []string{}, false)
	if err != nil {
		return fmt.Errorf("error getting ref sources: %w", err)
	}

	source := app.Spec.GetSource()

	cache.LogDebugManifestCacheKeyFields("moving manifests cache", "webhook app revision changed", change.shaBefore, &source, refSources, &clusterInfo, app.Spec.Destination.Namespace, trackingMethod, appInstanceLabelKey, app.Name, nil)

	if err := handler.repoCache.SetNewRevisionManifests(change.shaAfter, change.shaBefore, &source, refSources, &clusterInfo, app.Spec.Destination.Namespace, trackingMethod, appInstanceLabelKey, app.Name, nil); err != nil {
		return fmt.Errorf("error setting new revision manifests: %w", err)
	}

	return nil
}

func NewWebhook(
	parallelism int,
	maxPayloadSize int64,
	argoCdSettingsMgr *settings.SettingsManager,
	db db.ArgoDB,
	ns string,
	appNs []string,
	appClientset appClientset.Interface,
	repoCache *cache.Cache,
	serverCache *serverCache.Cache,
) (*webhook.Webhook, error) {
	payloadHandler := &ApplicationWebhookPayloadHandler{
		db:                db,
		ns:                ns,
		appNs:             appNs,
		appClientset:      appClientset,
		repoCache:         repoCache,
		serverCache:       serverCache,
		argoCdSettingsMgr: argoCdSettingsMgr,
	}

	webhook, err := webhook.NewWebhook(
		parallelism,
		maxPayloadSize,
		argoCdSettingsMgr,
		payloadHandler,
	)
	if err != nil {
		return nil, err
	}

	return webhook, nil
}
