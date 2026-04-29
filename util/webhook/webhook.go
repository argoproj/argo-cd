package webhook

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/argoproj/argo-cd/v3/common"

	bb "github.com/ktrysmt/go-bitbucket"
	"k8s.io/apimachinery/pkg/labels"

	alpha1 "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"

	"github.com/Masterminds/semver/v3"
	bitbucketv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/go-playground/webhooks/v6/azuredevops"
	"github.com/go-playground/webhooks/v6/bitbucket"
	bitbucketserver "github.com/go-playground/webhooks/v6/bitbucket-server"
	"github.com/go-playground/webhooks/v6/github"
	"github.com/go-playground/webhooks/v6/gitlab"
	"github.com/go-playground/webhooks/v6/gogs"
	gogsclient "github.com/gogits/go-gogs-client"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v3/reposerver/cache"
	servercache "github.com/argoproj/argo-cd/v3/server/cache"
	"github.com/argoproj/argo-cd/v3/util/app/path"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/glob"
	"github.com/argoproj/argo-cd/v3/util/guard"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

type settingsSource interface {
	GetAppInstanceLabelKey() (string, error)
	GetTrackingMethod() (string, error)
	GetInstallationID() (string, error)
}

// https://www.rfc-editor.org/rfc/rfc3986#section-3.2.1
// https://github.com/shadow-maint/shadow/blob/master/libmisc/chkname.c#L36
const usernameRegex = `[\w\.][\w\.-]{0,30}[\w\.\$-]?`

const payloadQueueSize = 50000

const panicMsgServer = "panic while processing api-server webhook event"

var (
	webhookManifestCacheWarmDisabled = os.Getenv("ARGOCD_WEBHOOK_MANIFEST_CACHE_WARM_DISABLED") == "true"
	webhookRequestsTotal             = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "argocd_webhook_requests_total",
		Help: "Number of webhook requests received by repo.",
	}, []string{"repo"})

	webhookStoreCacheAttemptsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "argocd_webhook_store_cache_attempts_total",
		Help: "Number of attempts to store previously cached manifests triggered by a webhook event.",
	}, []string{"repo", "successful"})

	webhookHandlersInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "argocd_webhook_handlers_in_flight",
		Help: "Number of webhook HandleEvent calls currently in progress.",
	})
)

var _ settingsSource = &settings.SettingsManager{}

type ArgoCDWebhookHandler struct {
	sync.WaitGroup         // for testing
	repoCache              *cache.Cache
	serverCache            *servercache.Cache
	db                     db.ArgoDB
	ns                     string
	appNs                  []string
	appClientset           appclientset.Interface
	appsLister             alpha1.ApplicationLister
	github                 *github.Webhook
	gitlab                 *gitlab.Webhook
	bitbucket              *bitbucket.Webhook
	bitbucketserver        *bitbucketserver.Webhook
	azuredevops            *azuredevops.Webhook
	gogs                   *gogs.Webhook
	settings               *settings.ArgoCDSettings
	settingsSrc            settingsSource
	queue                  chan any
	maxWebhookPayloadSizeB int64
	ghcrHandler            *GHCRParser
}

func NewHandler(namespace string, applicationNamespaces []string, webhookParallelism int, appClientset appclientset.Interface, appsLister alpha1.ApplicationLister,
	set *settings.ArgoCDSettings, settingsSrc settingsSource, repoCache *cache.Cache,
	serverCache *servercache.Cache, argoDB db.ArgoDB, maxWebhookPayloadSizeB int64,
) *ArgoCDWebhookHandler {
	githubWebhook, err := github.New(github.Options.Secret(set.GetWebhookGitHubSecret()))
	if err != nil {
		log.Warnf("Unable to init the GitHub webhook")
	}
	gitlabWebhook, err := gitlab.New(gitlab.Options.Secret(set.GetWebhookGitLabSecret()))
	if err != nil {
		log.Warnf("Unable to init the GitLab webhook")
	}
	bitbucketWebhook, err := bitbucket.New(bitbucket.Options.UUID(set.GetWebhookBitbucketUUID()))
	if err != nil {
		log.Warnf("Unable to init the Bitbucket webhook")
	}
	bitbucketserverWebhook, err := bitbucketserver.New(bitbucketserver.Options.Secret(set.GetWebhookBitbucketServerSecret()))
	if err != nil {
		log.Warnf("Unable to init the Bitbucket Server webhook")
	}
	gogsWebhook, err := gogs.New(gogs.Options.Secret(set.GetWebhookGogsSecret()))
	if err != nil {
		log.Warnf("Unable to init the Gogs webhook")
	}
	azuredevopsWebhook, err := azuredevops.New(azuredevops.Options.BasicAuth(set.GetWebhookAzureDevOpsUsername(), set.GetWebhookAzureDevOpsPassword()))
	if err != nil {
		log.Warnf("Unable to init the Azure DevOps webhook")
	}
	acdWebhook := ArgoCDWebhookHandler{
		ns:                     namespace,
		appNs:                  applicationNamespaces,
		appClientset:           appClientset,
		github:                 githubWebhook,
		gitlab:                 gitlabWebhook,
		bitbucket:              bitbucketWebhook,
		bitbucketserver:        bitbucketserverWebhook,
		azuredevops:            azuredevopsWebhook,
		gogs:                   gogsWebhook,
		settingsSrc:            settingsSrc,
		repoCache:              repoCache,
		serverCache:            serverCache,
		settings:               set,
		db:                     argoDB,
		queue:                  make(chan any, payloadQueueSize),
		maxWebhookPayloadSizeB: maxWebhookPayloadSizeB,
		appsLister:             appsLister,
		ghcrHandler:            NewGHCRParser(set.GetWebhookGitHubSecret()),
	}

	acdWebhook.startWorkerPool(webhookParallelism)

	return &acdWebhook
}

func (a *ArgoCDWebhookHandler) startWorkerPool(webhookParallelism int) {
	compLog := log.WithField("component", "api-server-webhook")
	for range webhookParallelism {
		a.Go(func() {
			for {
				payload, ok := <-a.queue
				if !ok {
					return
				}
				guard.RecoverAndLog(func() { a.HandleEvent(payload) }, compLog, panicMsgServer)
			}
		})
	}
}

func ParseRevision(ref string) string {
	refParts := strings.SplitN(ref, "/", 3)
	return refParts[len(refParts)-1]
}

// affectedRevisionInfo examines a payload from a webhook event, and extracts the repo web URL,
// the revision, and whether, or not this affected origin/HEAD (the default branch of the repository)
func (a *ArgoCDWebhookHandler) affectedRevisionInfo(payloadIf any) (webURLs []string, revision string, change changeInfo, touchedHead bool, changedFiles []string) {
	switch payload := payloadIf.(type) {
	case azuredevops.GitPushEvent:
		// See: https://learn.microsoft.com/en-us/azure/devops/service-hooks/events?view=azure-devops#git.push
		webURLs = append(webURLs, payload.Resource.Repository.RemoteURL)
		if len(payload.Resource.RefUpdates) > 0 {
			revision = ParseRevision(payload.Resource.RefUpdates[0].Name)
			change.shaAfter = ParseRevision(payload.Resource.RefUpdates[0].NewObjectID)
			change.shaBefore = ParseRevision(payload.Resource.RefUpdates[0].OldObjectID)
			touchedHead = payload.Resource.RefUpdates[0].Name == payload.Resource.Repository.DefaultBranch
		}
		// unfortunately, Azure DevOps doesn't provide a list of changed files
	case github.PushPayload:
		// See: https://developer.github.com/v3/activity/events/types/#pushevent
		webURLs = append(webURLs, payload.Repository.HTMLURL)
		revision = ParseRevision(payload.Ref)
		change.shaAfter = ParseRevision(payload.After)
		change.shaBefore = ParseRevision(payload.Before)
		touchedHead = bool(payload.Repository.DefaultBranch == revision)
		for _, commit := range payload.Commits {
			changedFiles = append(changedFiles, commit.Added...)
			changedFiles = append(changedFiles, commit.Modified...)
			changedFiles = append(changedFiles, commit.Removed...)
		}
	case gitlab.PushEventPayload:
		// See: https://docs.gitlab.com/ee/user/project/integrations/webhooks.html
		webURLs = append(webURLs, payload.Project.WebURL)
		revision = ParseRevision(payload.Ref)
		change.shaAfter = ParseRevision(payload.After)
		change.shaBefore = ParseRevision(payload.Before)
		touchedHead = bool(payload.Project.DefaultBranch == revision)
		for _, commit := range payload.Commits {
			changedFiles = append(changedFiles, commit.Added...)
			changedFiles = append(changedFiles, commit.Modified...)
			changedFiles = append(changedFiles, commit.Removed...)
		}
	case gitlab.TagEventPayload:
		// See: https://docs.gitlab.com/ee/user/project/integrations/webhooks.html
		// NOTE: this is untested
		webURLs = append(webURLs, payload.Project.WebURL)
		revision = ParseRevision(payload.Ref)
		change.shaAfter = ParseRevision(payload.After)
		change.shaBefore = ParseRevision(payload.Before)
		touchedHead = bool(payload.Project.DefaultBranch == revision)
		for _, commit := range payload.Commits {
			changedFiles = append(changedFiles, commit.Added...)
			changedFiles = append(changedFiles, commit.Modified...)
			changedFiles = append(changedFiles, commit.Removed...)
		}
	case bitbucket.RepoPushPayload:
		// See: https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Push
		// NOTE: this is untested
		webURLs = append(webURLs, payload.Repository.Links.HTML.Href)
		for _, changes := range payload.Push.Changes {
			revision = changes.New.Name
			change.shaBefore = changes.Old.Target.Hash
			change.shaAfter = changes.New.Target.Hash
			break
		}
		// Not actually sure how to check if the incoming change affected HEAD just by examining the
		// payload alone. To be safe, we just return true and let the controller check for himself.
		touchedHead = true

		// Get DiffSet only for authenticated webhooks.
		// when WebhookBitbucketUUID is set in argocd-secret, then the payload must be signed and
		// signature is validated before payload is parsed.
		if a.settings.GetWebhookBitbucketUUID() != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			argoRepo, err := a.lookupRepository(ctx, webURLs[0])
			if err != nil {
				log.Warnf("error trying to find a matching repo for URL %s: %v", payload.Repository.Links.HTML.Href, err)
				break
			}
			if argoRepo == nil {
				// it could be a public repository with no repo creds stored.
				// initialize with empty bearer token to use the no auth bitbucket client.
				log.Debugf("no bitbucket repository configured for URL %s, initializing with empty bearer token", webURLs[0])
				argoRepo = &v1alpha1.Repository{BearerToken: "", Repo: webURLs[0]}
			}
			apiBaseURL := strings.ReplaceAll(payload.Repository.Links.Self.Href, "/repositories/"+payload.Repository.FullName, "")
			bbClient, err := newBitbucketClient(ctx, argoRepo, apiBaseURL)
			if err != nil {
				log.Warnf("error creating Bitbucket client for repo %s: %v", payload.Repository.Name, err)
				break
			}
			log.Debugf("created bitbucket client with base URL '%s'", apiBaseURL)
			owner, repoSlug, ok := strings.Cut(payload.Repository.FullName, "/")
			if !ok || owner == "" || repoSlug == "" {
				log.Warnf("error parsing bitbucket repository full name %q", payload.Repository.FullName)
				break
			}
			spec := change.shaBefore + ".." + change.shaAfter
			diffStatChangedFiles, err := fetchDiffStatFromBitbucket(ctx, bbClient, owner, repoSlug, spec)
			if err != nil {
				log.Warnf("error fetching changed files using bitbucket diffstat api: %v", err)
			}
			changedFiles = append(changedFiles, diffStatChangedFiles...)
			touchedHead, err = isHeadTouched(ctx, bbClient, owner, repoSlug, revision)
			if err != nil {
				log.Warnf("error fetching bitbucket repo details: %v", err)
				// To be safe, we just return true and let the controller check for himself.
				touchedHead = true
			}
		}

	case bitbucketserver.RepositoryReferenceChangedPayload:
		var httpCloneURL string
		// Webhook module does not parse the inner links
		if payload.Repository.Links != nil {
			clone, ok := payload.Repository.Links["clone"].([]any)
			if ok {
				for _, l := range clone {
					link, ok := l.(map[string]any)
					if !ok {
						continue
					}
					href, ok := link["href"].(string)
					if !ok {
						continue
					}
					switch link["name"] {
					case "http":
						httpCloneURL = href
						webURLs = append(webURLs, href)
					case "ssh":
						webURLs = append(webURLs, href)
					}
				}
			}
		}

		// TODO: bitbucket includes multiple changes as part of a single event.
		// We only pick the first but need to consider how to handle multiple
		for _, refChange := range payload.Changes {
			revision = ParseRevision(refChange.Reference.ID)
			change.shaBefore = refChange.FromHash
			change.shaAfter = refChange.ToHash
			break
		}
		// Default to true as a safe fallback. When WebhookBitbucketServerSecret is set,
		// the actual default branch is determined via the API below and touchedHead is updated.
		touchedHead = true

		// Get changed files only for authenticated webhooks.
		// When WebhookBitbucketServerSecret is set, webhook requests are validated before processing.
		if a.settings.GetWebhookBitbucketServerSecret() != "" && httpCloneURL != "" {
			bbFiles, headTouched, err := a.fetchBBServerChangedFiles(httpCloneURL, payload.Repository.Project.Key, payload.Repository.Slug, change.shaBefore, change.shaAfter, revision)
			if err != nil {
				log.Warnf("error fetching Bitbucket Server changed files: %v", err)
			} else {
				changedFiles = append(changedFiles, bbFiles...)
				touchedHead = headTouched
			}
		}

	case gogsclient.PushPayload:
		revision = ParseRevision(payload.Ref)
		change.shaAfter = ParseRevision(payload.After)
		change.shaBefore = ParseRevision(payload.Before)
		if payload.Repo != nil {
			webURLs = append(webURLs, payload.Repo.HTMLURL)
			touchedHead = payload.Repo.DefaultBranch == revision
		}
		for _, commit := range payload.Commits {
			changedFiles = append(changedFiles, commit.Added...)
			changedFiles = append(changedFiles, commit.Modified...)
			changedFiles = append(changedFiles, commit.Removed...)
		}
	}
	return webURLs, revision, change, touchedHead, changedFiles
}

type changeInfo struct {
	shaBefore string
	shaAfter  string
}

// HandleEvent handles webhook events for repo push events
func (a *ArgoCDWebhookHandler) HandleEvent(payload any) {
	webhookHandlersInFlight.Inc()
	defer webhookHandlersInFlight.Dec()

	start := time.Now()
	log.Info("Webhook handler started")
	defer func() {
		log.Infof("Webhook handler completed in %v", time.Since(start))
	}()

	if e, ok := payload.(*RegistryEvent); ok {
		a.HandleRegistryEvent(e)
		return
	}
	webURLs, revision, change, touchedHead, changedFiles := a.affectedRevisionInfo(payload)
	// NOTE: the webURL does not include the .git extension
	if len(webURLs) == 0 {
		log.Info("Ignoring webhook event")
		return
	}
	for _, webURL := range webURLs {
		log.Infof("Received push event repo: %s, revision: %s, touchedHead: %v", webURL, revision, touchedHead)
		webhookRequestsTotal.WithLabelValues(git.NormalizeGitURL(webURL)).Inc()
	}

	nsFilter := a.ns
	if len(a.appNs) > 0 {
		// Retrieve app from all namespaces
		nsFilter = ""
	}

	appIf := a.appsLister.Applications(nsFilter)
	apps, err := appIf.List(labels.Everything())
	if err != nil {
		log.Errorf("Failed to list applications: %v", err)
		return
	}

	installationID, err := a.settingsSrc.GetInstallationID()
	if err != nil {
		log.Errorf("Failed to get installation ID: %v", err)
		return
	}
	trackingMethod, err := a.settingsSrc.GetTrackingMethod()
	if err != nil {
		log.Errorf("Failed to get trackingMethod: %v", err)
		return
	}
	appInstanceLabelKey, err := a.settingsSrc.GetAppInstanceLabelKey()
	if err != nil {
		log.Errorf("Failed to get appInstanceLabelKey: %v", err)
		return
	}

	// Skip any application that is neither in the control plane's namespace
	// nor in the list of enabled namespaces.
	var filteredApps []v1alpha1.Application
	for _, app := range apps {
		if app.Namespace == a.ns || glob.MatchStringInList(a.appNs, app.Namespace, glob.REGEXP) {
			filteredApps = append(filteredApps, *app)
		}
	}

	for _, webURL := range webURLs {
		repoRegexp, err := GetWebURLRegex(webURL)
		if err != nil {
			log.Errorf("Failed to get repoRegexp: %s", err)
			continue
		}

		cacheWarmDisabled := webhookManifestCacheWarmDisabled
		if !cacheWarmDisabled {
			repo, err := a.lookupRepository(context.Background(), webURL)
			if err != nil {
				log.Debugf("Failed to look up repository for %s: %v", webURL, err)
			} else if repo != nil && repo.WebhookManifestCacheWarmDisabled {
				cacheWarmDisabled = true
			}
		}

		// iterate over apps and check if any files specified in their sources have changed
		for _, app := range filteredApps {
			// get all sources, including sync source and dry source if source hydrator is configured
			sources := app.Spec.GetSources()
			if app.Spec.SourceHydrator != nil {
				// we already have sync source, so add dry source if source hydrator is configured
				sources = append(sources, app.Spec.SourceHydrator.GetDrySource())
			}

			// iterate over all sources and check if any files specified in refresh paths have changed
			for _, source := range sources {
				if sourceRevisionHasChanged(source, revision, touchedHead) && sourceUsesURL(source, webURL, repoRegexp) {
					refreshPaths := path.GetSourceRefreshPaths(&app, source)
					if path.AppFilesHaveChanged(refreshPaths, changedFiles) {
						hydrate := false
						if app.Spec.SourceHydrator != nil {
							drySource := app.Spec.SourceHydrator.GetDrySource()
							if (&source).Equals(&drySource) {
								hydrate = true
							}
						}

						// refresh paths have changed, so we need to refresh the app
						log.Infof("refreshing app '%s' from webhook", app.Name)
						if hydrate {
							// log if we need to hydrate the app
							log.Infof("webhook trigger refresh app to hydrate '%s'", app.Name)
						}
						namespacedAppInterface := a.appClientset.ArgoprojV1alpha1().Applications(app.Namespace)
						if _, err := argo.RefreshApp(namespacedAppInterface, app.Name, v1alpha1.RefreshTypeNormal, hydrate); err != nil {
							log.Errorf("Failed to refresh app '%s': %v", app.Name, err)
						}
						break // we don't need to check other sources
					} else if change.shaBefore != "" && change.shaAfter != "" && !cacheWarmDisabled {
						// update the cached manifests with the new revision cache key
						if err := a.storePreviouslyCachedManifests(&app, change, trackingMethod, appInstanceLabelKey, installationID, source); err != nil {
							log.Debugf("Failed to store cached manifests of previous revision for app '%s': %v", app.Name, err)
							webhookStoreCacheAttemptsTotal.WithLabelValues(git.NormalizeGitURL(webURL), "false").Inc()
						} else {
							webhookStoreCacheAttemptsTotal.WithLabelValues(git.NormalizeGitURL(webURL), "true").Inc()
						}
					}
				}
			}
		}
	}
}

// GetWebURLRegex compiles a regex that will match any targetRevision referring to the same repo as
// the given webURL. webURL is expected to be a URL from an SCM webhook payload pointing to the web
// page for the repo.
func GetWebURLRegex(webURL string) (*regexp.Regexp, error) {
	// 1. Optional: protocol (`http`, `https`, or `ssh`) followed by `://`
	// 2. Optional: username followed by `@`
	// 3. Optional: `ssh` or `altssh` subdomain
	// 4. Required: hostname parsed from `webURL`
	// 5. Optional: `:` followed by port number
	// 6. Required: `:` or `/`
	// 7. Required: path parsed from `webURL`
	// 8. Optional: `.git` extension
	return getURLRegex(webURL, `(?i)^((https?|ssh)://)?(%[1]s@)?((alt)?ssh\.)?%[2]s(:\d+)?[:/]%[3]s(\.git)?$`)
}

// GetAPIURLRegex compiles a regex that will match any targetRevision referring to the same repo as
// the given apiURL.
func GetAPIURLRegex(apiURL string) (*regexp.Regexp, error) {
	// 1. Optional: protocol (`http` or `https`) followed by `://`
	// 2. Optional: username followed by `@`
	// 3. Required: hostname parsed from `webURL`
	// 4. Optional: `:` followed by port number
	// 5. Optional: `/`
	return getURLRegex(apiURL, `(?i)^(https?://)?(%[1]s@)?%[2]s(:\d+)?/?$`)
}

func getURLRegex(originalURL string, regexpFormat string) (*regexp.Regexp, error) {
	urlObj, err := url.Parse(originalURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL '%s'", originalURL)
	}

	regexEscapedHostname := regexp.QuoteMeta(urlObj.Hostname())
	const urlPathSeparator = "/"
	regexEscapedPath := regexp.QuoteMeta(strings.TrimPrefix(urlObj.EscapedPath(), urlPathSeparator))
	regexpStr := fmt.Sprintf(regexpFormat, usernameRegex, regexEscapedHostname, regexEscapedPath)
	repoRegexp, err := regexp.Compile(regexpStr)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regexp for URL '%s'", originalURL)
	}

	return repoRegexp, nil
}

func (a *ArgoCDWebhookHandler) storePreviouslyCachedManifests(app *v1alpha1.Application, change changeInfo, trackingMethod string, appInstanceLabelKey string, installationID string, source v1alpha1.ApplicationSource) error {
	destCluster, err := argo.GetDestinationCluster(context.Background(), app.Spec.Destination, a.db)
	if err != nil {
		return fmt.Errorf("error validating destination: %w", err)
	}

	var clusterInfo v1alpha1.ClusterInfo
	err = a.serverCache.GetClusterInfo(destCluster.Server, &clusterInfo)
	if err != nil {
		return fmt.Errorf("error getting cluster info: %w", err)
	}

	var sources v1alpha1.ApplicationSources
	if app.Spec.HasMultipleSources() {
		sources = app.Spec.GetSources()
	} else {
		sources = append(sources, app.Spec.GetSource())
	}

	refSources, err := argo.GetRefSources(context.Background(), sources, app.Spec.Project, a.db.GetRepository, []string{})
	if err != nil {
		return fmt.Errorf("error getting ref sources: %w", err)
	}

	cache.LogDebugManifestCacheKeyFields("moving manifests cache", "webhook app revision changed", change.shaBefore, &source, refSources, &clusterInfo, app.Spec.Destination.Namespace, trackingMethod, appInstanceLabelKey, app.Name, nil)

	if err := a.repoCache.SetNewRevisionManifests(change.shaAfter, change.shaBefore, &source, refSources, refSources, &clusterInfo, app.Spec.Destination.Namespace, trackingMethod, appInstanceLabelKey, app.Name, nil, nil, installationID); err != nil {
		return fmt.Errorf("error setting new revision manifests: %w", err)
	}

	return nil
}

// lookupRepository returns a repository with its credentials for a given URL. If there are no matching repository secret found,
// then nil repository is returned.
func (a *ArgoCDWebhookHandler) lookupRepository(ctx context.Context, repoURL string) (*v1alpha1.Repository, error) {
	repositories, err := a.db.ListRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing repositories: %w", err)
	}
	var repository *v1alpha1.Repository
	for _, repo := range repositories {
		if git.SameURL(repo.Repo, repoURL) {
			log.Debugf("found a matching repository for URL %s", repoURL)
			return repo, nil
		}
	}
	return repository, nil
}

func sourceRevisionHasChanged(source v1alpha1.ApplicationSource, revision string, touchedHead bool) bool {
	targetRev := ParseRevision(source.TargetRevision)
	if targetRev == "HEAD" || targetRev == "" { // revision is head
		return touchedHead
	}
	targetRevisionHasPrefixList := []string{"refs/heads/", "refs/tags/"}
	for _, prefix := range targetRevisionHasPrefixList {
		if strings.HasPrefix(source.TargetRevision, prefix) {
			return compareRevisions(revision, targetRev)
		}
	}

	return compareRevisions(revision, source.TargetRevision)
}

func compareRevisions(revision string, targetRevision string) bool {
	if revision == targetRevision {
		return true
	}

	// If basic equality checking fails, it might be that the target revision is
	// a semver version constraint
	constraint, err := semver.NewConstraint(targetRevision)
	if err != nil {
		// The target revision is not a constraint
		return false
	}

	version, err := semver.NewVersion(revision)
	if err != nil {
		// The new revision is not a valid semver version, so it can't match the constraint.
		return false
	}

	return constraint.Check(version)
}

func sourceUsesURL(source v1alpha1.ApplicationSource, webURL string, repoRegexp *regexp.Regexp) bool {
	if !repoRegexp.MatchString(source.RepoURL) {
		log.Debugf("%s does not match %s", source.RepoURL, repoRegexp.String())
		return false
	}

	log.Debugf("%s uses repoURL %s", source.RepoURL, webURL)
	return true
}

// newBitbucketClient creates a new bitbucket client for the given repository and uses the provided apiURL to connect
// to the bitbucket server. If the repository uses basic auth, then a basic auth client is created or if bearer token
// is provided, then oauth based client is created.
func newBitbucketClient(_ context.Context, repository *v1alpha1.Repository, apiBaseURL string) (*bb.Client, error) {
	var bbClient *bb.Client
	var err error
	if repository.Username != "" && repository.Password != "" {
		log.Debugf("fetched user/password for repository URL '%s', initializing basic auth client", repository.Repo)
		if repository.Username == "x-token-auth" {
			bbClient, err = bb.NewOAuthbearerToken(repository.Password)
			if err != nil {
				return nil, fmt.Errorf("error creating BitBucket Cloud client with oauth bearer token: %w", err)
			}
		} else {
			bbClient, err = bb.NewBasicAuth(repository.Username, repository.Password)
			if err != nil {
				return nil, fmt.Errorf("error creating BitBucket Cloud client with basic auth: %w", err)
			}
		}
	} else {
		if repository.BearerToken != "" {
			log.Debugf("fetched bearer token for repository URL '%s', initializing bearer token auth based client", repository.Repo)
		} else {
			log.Debugf("no credentials available for repository URL '%s', initializing no auth client", repository.Repo)
		}
		bbClient, err = bb.NewOAuthbearerToken(repository.BearerToken)
		if err != nil {
			return nil, fmt.Errorf("error creating BitBucket Cloud client with oauth bearer token: %w", err)
		}
	}
	// parse and set the target URL of the Bitbucket server in the client
	repoBaseURL, err := url.Parse(apiBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bitbucket api base URL '%s'", apiBaseURL)
	}
	bbClient.SetApiBaseURL(*repoBaseURL)
	return bbClient, nil
}

// fetchDiffStatFromBitbucket gets the list of files changed between two commits, by making a diffstat api callback to the
// bitbucket server from where the webhook orignated.
func fetchDiffStatFromBitbucket(_ context.Context, bbClient *bb.Client, owner, repoSlug, spec string) ([]string, error) {
	// Getting the files changed from diff API:
	// https://developer.atlassian.com/cloud/bitbucket/rest/api-group-commits/#api-repositories-workspace-repo-slug-diffstat-spec-get

	// invoke the diffstat api call to get the list of changed files between two commit shas
	log.Debugf("invoking diffstat call with parameters: [Owner:%s, RepoSlug:%s, Spec:%s]", owner, repoSlug, spec)
	diffStatResp, err := bbClient.Repositories.Diff.GetDiffStat(&bb.DiffStatOptions{
		Owner:    owner,
		RepoSlug: repoSlug,
		Spec:     spec,
		Renames:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("error getting the diffstat: %w", err)
	}
	changedFiles := make([]string, len(diffStatResp.DiffStats))
	for i, value := range diffStatResp.DiffStats {
		changedFilePath := value.New["path"]
		if changedFilePath != nil {
			changedFiles[i] = changedFilePath.(string)
		}
	}
	log.Debugf("changed files for spec %s: %v", spec, changedFiles)
	return changedFiles, nil
}

// isHeadTouched returns true if the repository's main branch is modified, false otherwise
func isHeadTouched(ctx context.Context, bbClient *bb.Client, owner, repoSlug, revision string) (bool, error) {
	bbRepoOptions := &bb.RepositoryOptions{
		Owner:    owner,
		RepoSlug: repoSlug,
	}
	bbRepo, err := bbClient.Repositories.Repository.Get(bbRepoOptions.WithContext(ctx))
	if err != nil {
		return false, err
	}
	return bbRepo.Mainbranch.Name == revision, nil
}

// bbServerAPITimeout is the timeout for outbound Bitbucket Server REST API calls made during
// webhook processing.
const bbServerAPITimeout = 10 * time.Second

// fetchBBServerChangedFiles calls the Bitbucket Server REST API to obtain the set of changed
// files and whether the push touched the repository's default branch. 
func (a *ArgoCDWebhookHandler) fetchBBServerChangedFiles(httpCloneURL, projectKey, repoSlug, fromHash, toHash, revision string) ([]string, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), bbServerAPITimeout)
	defer cancel()

	argoRepo, err := a.lookupRepository(ctx, httpCloneURL)
	if err != nil {
		return nil, true, fmt.Errorf("error looking up repository for URL %s: %w", httpCloneURL, err)
	}
	if argoRepo == nil {
		log.Debugf("no Bitbucket Server repository configured for URL %s, skipping changed files fetch", httpCloneURL)
		return nil, true, nil
	}
	serverURL, err := extractBBServerBaseURL(httpCloneURL)
	if err != nil {
		return nil, true, fmt.Errorf("error extracting Bitbucket Server base URL from %s: %w", httpCloneURL, err)
	}
	bbClient := newBitbucketServerClient(ctx, argoRepo, serverURL)
	log.Debugf("created Bitbucket Server client with base URL '%s'", serverURL)

	changedFiles, err := fetchChangesFromBitbucketServer(bbClient, projectKey, repoSlug, fromHash, toHash)
	if err != nil {
		log.Warnf("error fetching changed files from Bitbucket Server: %v", err)
		changedFiles = nil
	}
	// Default to true (safe fallback) if the API call fails.
	touchedHead := true
	headTouched, err := isBBServerHeadTouched(bbClient, projectKey, repoSlug, revision)
	if err != nil {
		log.Warnf("error fetching Bitbucket Server default branch: %v", err)
	} else {
		touchedHead = headTouched
	}
	return changedFiles, touchedHead, nil
}

// extractBBServerBaseURL extracts the scheme and host from a Bitbucket Server clone URL.
// For example, "https://bitbucketserver/scm/project/repo.git" returns "https://bitbucketserver".
func extractBBServerBaseURL(cloneURL string) (string, error) {
	u, err := url.Parse(cloneURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse Bitbucket Server clone URL '%s': %w", cloneURL, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid Bitbucket Server clone URL '%s': missing scheme or host", cloneURL)
	}
	// Bitbucket Server clone URLs always contain "/scm/" as the separator between the
	// server base path and the project/repo path. Everything before "/scm/" is the
	// server base URL, which may include a subpath when Bitbucket is deployed under a
	// context path (e.g. https://mycompany.com/bitbucket/scm/... → https://mycompany.com/bitbucket).
	if idx := strings.Index(u.Path, "/scm/"); idx >= 0 {
		return u.Scheme + "://" + u.Host + u.Path[:idx], nil
	}
	return u.Scheme + "://" + u.Host, nil
}

// newBitbucketServerClient creates a new Bitbucket Server API client for the given repository credentials and server URL.
func newBitbucketServerClient(ctx context.Context, repository *v1alpha1.Repository, serverURL string) *bitbucketv1.APIClient {
	// Ensure the base path ends with /rest for the Bitbucket Server REST API
	if !strings.HasSuffix(serverURL, "/rest") {
		serverURL = strings.TrimSuffix(serverURL, "/") + "/rest"
	}
	bitbucketConfig := bitbucketv1.NewConfiguration(serverURL)
	if repository != nil {
		if repository.Username != "" && repository.Password != "" {
			ctx = context.WithValue(ctx, bitbucketv1.ContextBasicAuth, bitbucketv1.BasicAuth{
				UserName: repository.Username,
				Password: repository.Password,
			})
		} else if repository.BearerToken != "" {
			ctx = context.WithValue(ctx, bitbucketv1.ContextAccessToken, repository.BearerToken)
		}
	}
	return bitbucketv1.NewAPIClient(ctx, bitbucketConfig)
}

// fetchChangesFromBitbucketServer retrieves the list of files changed between fromHash and toHash
// by calling the Bitbucket Server changes REST API.
func fetchChangesFromBitbucketServer(client *bitbucketv1.APIClient, projectKey, repoSlug, fromHash, toHash string) ([]string, error) {
	log.Debugf("invoking Bitbucket Server changes call: [ProjectKey:%s, RepoSlug:%s, since:%s, until:%s]", projectKey, repoSlug, fromHash, toHash)
	var changedFiles []string
	start := 0
	for {
		opts := map[string]any{
			"since": fromHash,
			"until": toHash,
			"start": start,
		}
		resp, err := client.DefaultApi.GetChanges(projectKey, repoSlug, opts)
		if err != nil {
			return nil, fmt.Errorf("error fetching changes from Bitbucket Server: %w", err)
		}
		values, ok := resp.Values["values"].([]any)
		if !ok {
			break
		}
		for _, v := range values {
			entry, ok := v.(map[string]any)
			if !ok {
				continue
			}
			pathObj, ok := entry["path"].(map[string]any)
			if !ok {
				continue
			}
			filePath, ok := pathObj["toString"].(string)
			if !ok || filePath == "" {
				continue
			}
			changedFiles = append(changedFiles, filePath)
		}
		isLastPage, _ := resp.Values["isLastPage"].(bool)
		if isLastPage {
			break
		}
		nextStart, ok := resp.Values["nextPageStart"].(float64)
		if !ok {
			break
		}
		start = int(nextStart)
	}
	log.Debugf("Bitbucket Server changed files: %v", changedFiles)
	return changedFiles, nil
}

// isBBServerHeadTouched returns true if the push updated the repository's default branch.
func isBBServerHeadTouched(client *bitbucketv1.APIClient, projectKey, repoSlug, revision string) (bool, error) {
	resp, err := client.DefaultApi.GetDefaultBranch(projectKey, repoSlug)
	if err != nil {
		return false, err
	}
	branch, err := bitbucketv1.GetBranchResponse(resp)
	if err != nil {
		return false, err
	}
	return branch.DisplayID == revision, nil
}

func (a *ArgoCDWebhookHandler) Handler(w http.ResponseWriter, r *http.Request) {
	var payload any
	var err error

	r.Body = http.MaxBytesReader(w, r.Body, a.maxWebhookPayloadSizeB)

	if event, handled, err := a.processRegistryWebhook(r); handled {
		if err != nil {
			if errors.Is(err, ErrHMACVerificationFailed) {
				log.WithField(common.SecurityField, common.SecurityHigh).Infof("Registry webhook HMAC verification failed")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			log.Infof("Registry webhook processing failed: %s", err)
			http.Error(w, "Registry webhook processing failed", http.StatusBadRequest)
			return
		}
		if event == nil {
			w.WriteHeader(http.StatusOK)
			return
		}
		select {
		case a.queue <- event:
		default:
			log.Info("Queue is full, discarding registry webhook payload")
			http.Error(w, "Queue is full, discarding registry webhook payload", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Registry event received. Processing triggered."))
		return
	}

	payload, err = a.processSCMWebhook(r)
	if err == nil && payload == nil {
		http.Error(w, "Unknown webhook event", http.StatusBadRequest)
		return
	}
	if err != nil {
		// If the error is due to a large payload, return a more user-friendly error message
		if isParsingPayloadError(err) {
			log.WithField(common.SecurityField, common.SecurityHigh).Warnf("Webhook processing failed: payload too large or corrupted (limit %v MB): %v", a.maxWebhookPayloadSizeB/1024/1024, err)
			http.Error(w, fmt.Sprintf("Webhook processing failed: payload must be valid JSON under %v MB", a.maxWebhookPayloadSizeB/1024/1024), http.StatusBadRequest)
			return
		}

		status := http.StatusBadRequest
		if r.Method != http.MethodPost {
			status = http.StatusMethodNotAllowed
		}
		log.Infof("Webhook processing failed: %v", err)
		http.Error(w, "Webhook processing failed", status)
		return
	}

	if payload != nil {
		select {
		case a.queue <- payload:
		default:
			log.Info("Queue is full, discarding webhook payload")
			http.Error(w, "Queue is full, discarding webhook payload", http.StatusServiceUnavailable)
			return
		}
	}
}

// isParsingPayloadError returns a bool if the error is parsing payload error
func isParsingPayloadError(err error) bool {
	return errors.Is(err, github.ErrParsingPayload) ||
		errors.Is(err, gitlab.ErrParsingPayload) ||
		errors.Is(err, gogs.ErrParsingPayload) ||
		errors.Is(err, bitbucket.ErrParsingPayload) ||
		errors.Is(err, bitbucketserver.ErrParsingPayload) ||
		errors.Is(err, azuredevops.ErrParsingPayload)
}

// processRegistryWebhook routes an incoming request to the appropriate registry
// handler. It returns the parsed event, a boolean indicating whether any handler
// claimed the request, and any error from parsing. When handled is false, the
// caller should fall through to SCM webhook processing.
func (a *ArgoCDWebhookHandler) processRegistryWebhook(r *http.Request) (*RegistryEvent, bool, error) {
	if a.ghcrHandler.CanHandle(r) {
		event, err := a.ghcrHandler.ProcessWebhook(r)
		return event, true, err
		// TODO: add dockerhub, ecr handler cases in future
	}
	return nil, false, nil
}

// processSCMWebhook processes an SCM webhook
func (a *ArgoCDWebhookHandler) processSCMWebhook(r *http.Request) (any, error) {
	var payload any
	var err error
	switch {
	case r.Header.Get("X-Vss-Activityid") != "":
		payload, err = a.azuredevops.Parse(r, azuredevops.GitPushEventType)
		if errors.Is(err, azuredevops.ErrBasicAuthVerificationFailed) {
			log.WithField(common.SecurityField, common.SecurityHigh).Infof("Azure DevOps webhook basic auth verification failed")
		}
	// Gogs needs to be checked before GitHub since it carries both Gogs and (incompatible) GitHub headers
	case r.Header.Get("X-Gogs-Event") != "":
		payload, err = a.gogs.Parse(r, gogs.PushEvent)
		if errors.Is(err, gogs.ErrHMACVerificationFailed) {
			log.WithField(common.SecurityField, common.SecurityHigh).Infof("Gogs webhook HMAC verification failed")
		}
	case r.Header.Get("X-GitHub-Event") != "":
		payload, err = a.github.Parse(r, github.PushEvent, github.PingEvent)
		if errors.Is(err, github.ErrHMACVerificationFailed) {
			log.WithField(common.SecurityField, common.SecurityHigh).Infof("GitHub webhook HMAC verification failed")
		}
	case r.Header.Get("X-Gitlab-Event") != "":
		payload, err = a.gitlab.Parse(r, gitlab.PushEvents, gitlab.TagEvents, gitlab.SystemHookEvents)
		if errors.Is(err, gitlab.ErrGitLabTokenVerificationFailed) {
			log.WithField(common.SecurityField, common.SecurityHigh).Infof("GitLab webhook token verification failed")
		}
	case r.Header.Get("X-Hook-UUID") != "":
		payload, err = a.bitbucket.Parse(r, bitbucket.RepoPushEvent)
		if errors.Is(err, bitbucket.ErrUUIDVerificationFailed) {
			log.WithField(common.SecurityField, common.SecurityHigh).Infof("BitBucket webhook UUID verification failed")
		}
	case r.Header.Get("X-Event-Key") != "":
		payload, err = a.bitbucketserver.Parse(r, bitbucketserver.RepositoryReferenceChangedEvent, bitbucketserver.DiagnosticsPingEvent)
		if errors.Is(err, bitbucketserver.ErrHMACVerificationFailed) {
			log.WithField(common.SecurityField, common.SecurityHigh).Infof("BitBucket webhook HMAC verification failed")
		}
	default:
		log.Debug("Ignoring unknown webhook event")
		return nil, nil
	}

	return payload, err
}
