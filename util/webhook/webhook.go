package webhook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	bb "github.com/ktrysmt/go-bitbucket"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/workqueue"

	alpha1 "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"

	"github.com/Masterminds/semver/v3"
	bitbucketv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/go-playground/webhooks/v6/azuredevops"
	"github.com/go-playground/webhooks/v6/bitbucket"
	bitbucketserver "github.com/go-playground/webhooks/v6/bitbucket-server"
	"github.com/go-playground/webhooks/v6/github"
	"github.com/go-playground/webhooks/v6/gitlab"
	"github.com/go-playground/webhooks/v6/gogs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v3/reposerver/cache"
	servercache "github.com/argoproj/argo-cd/v3/server/cache"
	applog "github.com/argoproj/argo-cd/v3/util/app/log"
	"github.com/argoproj/argo-cd/v3/util/app/path"
	"github.com/argoproj/argo-cd/v3/util/argo"
	certutil "github.com/argoproj/argo-cd/v3/util/cert"
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

// appRefreshRequest represents a request to refresh an application with optional jitter
type appRefreshRequest struct {
	appName      string
	appNamespace string
	hydrateType  *v1alpha1.HydrateType
}

type ArgoCDWebhookHandler struct {
	sync.WaitGroup                // for testing
	repoCache                     *cache.Cache
	serverCache                   *servercache.Cache
	db                            db.ArgoDB
	ns                            string
	appNs                         []string
	appClientset                  appclientset.Interface
	appsLister                    alpha1.ApplicationLister
	appProjectsLister             alpha1.AppProjectNamespaceLister
	parsers                       []Extractor
	settings                      *settings.ArgoCDSettings
	settingsSrc                   settingsSource
	queue                         chan any
	refreshQueue                  workqueue.TypedDelayingInterface[*appRefreshRequest]
	maxWebhookPayloadSizeB        int64
	webhookRefreshJitter          time.Duration
	webhookRefreshJitterThreshold int
	adoHTTPClientFactory          func(repo *v1alpha1.Repository) *http.Client
}

// ArgoCDParserOptions returns the webhook events the Argo CD API server handles.
func ArgoCDParserOptions() ParserOptions {
	return ParserOptions{
		AzureDevOpsEvents: []azuredevops.Event{azuredevops.GitPushEventType},
		GogsEvents:        []gogs.Event{gogs.PushEvent},
		GitHubEvents:      []github.Event{github.PushEvent, github.PingEvent},
		GitLabEvents:      []gitlab.Event{gitlab.PushEvents, gitlab.TagEvents, gitlab.SystemHookEvents},
		BitbucketEvents:   []bitbucket.Event{bitbucket.RepoPushEvent},
		BitbucketServerEvents: []bitbucketserver.Event{
			bitbucketserver.RepositoryReferenceChangedEvent,
			bitbucketserver.DiagnosticsPingEvent,
		},
		Harbor:    true,
		GHCR:      true,
		DockerHub: true,
	}
}

func NewHandler(namespace string, applicationNamespaces []string, webhookParallelism int, webhookRefreshWorkers int, appClientset appclientset.Interface, appsLister alpha1.ApplicationLister, set *settings.ArgoCDSettings, settingsSrc settingsSource, repoCache *cache.Cache, serverCache *servercache.Cache, argoDB db.ArgoDB, maxWebhookPayloadSizeB int64, webhookRefreshJitter time.Duration, webhookRefreshJitterThreshold int, appProjectsLister alpha1.AppProjectNamespaceLister) *ArgoCDWebhookHandler {
	parsers := NewParsers(set, ArgoCDParserOptions())
	log.Debugf("webhookRefreshJitter=%v", webhookRefreshJitter)
	log.Debugf("webhookRefreshJitterThreshold=%d", webhookRefreshJitterThreshold)

	acdWebhook := ArgoCDWebhookHandler{
		ns:                            namespace,
		appNs:                         applicationNamespaces,
		appClientset:                  appClientset,
		parsers:                       parsers,
		settingsSrc:                   settingsSrc,
		repoCache:                     repoCache,
		serverCache:                   serverCache,
		settings:                      set,
		db:                            argoDB,
		queue:                         make(chan any, payloadQueueSize),
		refreshQueue:                  workqueue.NewTypedDelayingQueue[*appRefreshRequest](),
		maxWebhookPayloadSizeB:        maxWebhookPayloadSizeB,
		appsLister:                    appsLister,
		appProjectsLister:             appProjectsLister,
		webhookRefreshJitter:          webhookRefreshJitter,
		webhookRefreshJitterThreshold: webhookRefreshJitterThreshold,
		adoHTTPClientFactory:          newADOHTTPClientCache().get,
	}

	// Payloads are processed after the HTTP request has completed, so the request context
	// cannot be propagated to the workers.
	handleEvent := func(payload any) { acdWebhook.HandleEvent(context.Background(), payload) }
	StartWorkers(&acdWebhook.WaitGroup, webhookParallelism, acdWebhook.queue, handleEvent, "api-server-webhook", panicMsgServer)
	acdWebhook.startRefreshWorkers(webhookRefreshWorkers)

	return &acdWebhook
}

// startRefreshWorkers starts worker goroutines to process app refresh requests from the refresh queue
func (a *ArgoCDWebhookHandler) startRefreshWorkers(count int) {
	for range count {
		a.Go(func() {
			for {
				req, shutdown := a.refreshQueue.Get()
				if shutdown {
					return
				}
				guard.RecoverAndLog(func() { a.processAppRefresh(req) }, log.WithField("component", "api-server-webhook-refresh"), panicMsgServer)
				a.refreshQueue.Done(req)
			}
		})
	}
}

// processAppRefresh processes a single app refresh request
func (a *ArgoCDWebhookHandler) processAppRefresh(req *appRefreshRequest) {
	namespacedAppInterface := a.appClientset.ArgoprojV1alpha1().Applications(req.appNamespace)
	_, err := argo.RefreshApp(namespacedAppInterface, req.appName, v1alpha1.RefreshTypeNormal, req.hydrateType)
	if err != nil {
		if req.hydrateType != nil {
			log.Warnf("Failed to hydrate app '%s' for controller reprocessing: %v", req.appName, err)
		} else {
			log.Warnf("Failed to refresh app '%s' for controller reprocessing: %v", req.appName, err)
		}
		return
	}

	if req.hydrateType != nil {
		log.Infof("Requested app '%s' hydration", req.appName)
	} else {
		log.Infof("Requested app '%s' refresh", req.appName)
	}
}

func ParseRevision(ref string) string {
	refParts := strings.SplitN(ref, "/", 3)
	return refParts[len(refParts)-1]
}

// affectedRevisionInfo examines a payload from a webhook event, and extracts the repo web URL,
// the revision, and whether, or not this affected origin/HEAD (the default branch of the repository)
func (a *ArgoCDWebhookHandler) affectedRevisionInfo(ctx context.Context, payloadIf any) (webURLs []string, revision string, change changeInfo, touchedHead bool, changedFiles []string) {
	info := ParsePushEvent(payloadIf)
	if info == nil {
		return nil, "", changeInfo{}, false, nil
	}
	// Azure DevOps and Bitbucket payloads don't say which files changed, and Bitbucket
	// payloads don't say whether the default branch moved, so ask the provider API, but
	// only for authenticated webhooks.
	switch payload := payloadIf.(type) {
	case azuredevops.GitPushEvent:
		// When the Azure DevOps webhook username or password is set in argocd-secret,
		// basic auth is validated before the payload is parsed.
		if a.azureDevOpsWebhookAuthConfigured() {
			info.ChangedFiles = append(info.ChangedFiles, a.lookupAndFetchADOChangedFiles(ctx, payload.Resource.Repository.RemoteURL, payload.Resource.Repository.ID, info.SHABefore, info.SHAAfter)...)
		} else {
			log.Debugf("skipping Azure DevOps changed files lookup for URL %s because Azure DevOps webhook authentication is not configured", payload.Resource.Repository.RemoteURL)
		}
	case bitbucket.RepoPushPayload:
		// when WebhookBitbucketUUID is set in argocd-secret, then the payload must be signed and
		// signature is validated before payload is parsed.
		if a.settings.GetWebhookBitbucketUUID() != "" {
			a.addBitbucketChanges(info, payload)
		}
	case bitbucketserver.RepositoryReferenceChangedPayload:
		// When WebhookBitbucketServerSecret is set, webhook requests are validated before processing.
		httpCloneURL, _ := bitbucketServerCloneURLs(payload.Repository)
		if a.settings.GetWebhookBitbucketServerSecret() != "" && httpCloneURL != "" {
			bbFiles, headTouched, err := a.fetchBBServerChangedFiles(httpCloneURL, payload.Repository.Project.Key, payload.Repository.Slug, info.SHABefore, info.SHAAfter, info.Revision)
			if err != nil {
				log.Warnf("error fetching Bitbucket Server changed files: %v", err)
			} else {
				info.ChangedFiles = append(info.ChangedFiles, bbFiles...)
				info.TouchedHead = headTouched
			}
		}
	}
	return info.WebURLs, info.Revision, changeInfo{shaBefore: info.SHABefore, shaAfter: info.SHAAfter}, info.TouchedHead, info.ChangedFiles
}

// addBitbucketChanges fills in the changed files and whether the default branch
// moved for a Bitbucket Cloud push, using the Bitbucket API. On any API error it
// leaves info as parsed from the payload.
func (a *ArgoCDWebhookHandler) addBitbucketChanges(info *PushEventInfo, payload bitbucket.RepoPushPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	argoRepo, err := a.lookupRepository(ctx, info.WebURLs[0])
	if err != nil {
		log.Warnf("error trying to find a matching repo for URL %s: %v", payload.Repository.Links.HTML.Href, err)
		return
	}
	if argoRepo == nil {
		// it could be a public repository with no repo creds stored.
		// initialize with empty bearer token to use the no auth bitbucket client.
		log.Debugf("no bitbucket repository configured for URL %s, initializing with empty bearer token", info.WebURLs[0])
		argoRepo = &v1alpha1.Repository{BearerToken: "", Repo: info.WebURLs[0]}
	}
	apiBaseURL := strings.ReplaceAll(payload.Repository.Links.Self.Href, "/repositories/"+payload.Repository.FullName, "")
	bbClient, err := newBitbucketClient(ctx, argoRepo, apiBaseURL)
	if err != nil {
		log.Warnf("error creating Bitbucket client for repo %s: %v", payload.Repository.Name, err)
		return
	}
	log.Debugf("created bitbucket client with base URL '%s'", apiBaseURL)
	owner, repoSlug, ok := strings.Cut(payload.Repository.FullName, "/")
	if !ok || owner == "" || repoSlug == "" {
		log.Warnf("error parsing bitbucket repository full name %q", payload.Repository.FullName)
		return
	}
	spec := info.SHABefore + ".." + info.SHAAfter
	diffStatChangedFiles, err := fetchDiffStatFromBitbucket(ctx, bbClient, owner, repoSlug, spec)
	if err != nil {
		log.Warnf("error fetching changed files using bitbucket diffstat api: %v", err)
	}
	info.ChangedFiles = append(info.ChangedFiles, diffStatChangedFiles...)
	info.TouchedHead, err = isHeadTouched(ctx, bbClient, owner, repoSlug, info.Revision)
	if err != nil {
		log.Warnf("error fetching bitbucket repo details: %v", err)
		// To be safe, we just return true and let the controller check for himself.
		info.TouchedHead = true
	}
}

type changeInfo struct {
	shaBefore string
	shaAfter  string
}

// HandleEvent handles webhook events for repo push events
func (a *ArgoCDWebhookHandler) HandleEvent(ctx context.Context, payload any) {
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
	webURLs, revision, change, touchedHead, changedFiles := a.affectedRevisionInfo(ctx, payload)
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
				log.Errorf("Failed to look up repository for %s: %v", webURL, err)
			} else if repo != nil && repo.WebhookManifestCacheWarmDisabled {
				cacheWarmDisabled = true
			}
		}

		appCount := 0
		// iterate over apps and check if any files specified in their sources have changed
		for _, app := range filteredApps {
			logCtx := log.WithFields(applog.GetAppLogFields(&app))
			// get all sources, including sync source and dry source if source hydrator is configured
			sources := app.Spec.GetSources()
			if app.Spec.SourceHydrator != nil {
				// we already have sync source, so add dry source if source hydrator is configured
				sources = append(sources, app.Spec.SourceHydrator.GetDrySource())
			}

			// iterate over all sources and check if any files specified in refresh paths have changed
			for _, source := range sources {
				if RevisionHasChanged(source.TargetRevision, revision, touchedHead) && RepoURLMatches(source.RepoURL, repoRegexp) {
					refreshPaths := path.GetSourceRefreshPaths(&app, source)
					if path.AppFilesHaveChanged(refreshPaths, changedFiles) {
						var hydrateType *v1alpha1.HydrateType
						if app.Spec.SourceHydrator != nil {
							drySource := app.Spec.SourceHydrator.GetDrySource()
							if (&source).Equals(&drySource) {
								ht := v1alpha1.HydrateTypeNormal
								hydrateType = &ht
							}
						}

						// refresh paths have changed, so we need to refresh the app
						logCtx.Info("refreshing app from webhook")
						if hydrateType != nil {
							logCtx.Infof("webhook trigger refresh app to hydrate")
						}
						appCount++
						req := &appRefreshRequest{
							appName:      app.Name,
							appNamespace: app.Namespace,
							hydrateType:  hydrateType,
						}
						// Apply jitter only if more than threshold apps are affected
						if appCount > a.webhookRefreshJitterThreshold && a.webhookRefreshJitter != 0 {
							jitter := time.Duration(float64(a.webhookRefreshJitter) * rand.Float64())
							a.refreshQueue.AddAfter(req, jitter)
						} else {
							a.refreshQueue.Add(req)
						}
						break // we don't need to check other sources
					} else if change.shaBefore != "" && change.shaAfter != "" && !cacheWarmDisabled {
						// update the cached manifests with the new revision cache key
						if err := a.storePreviouslyCachedManifests(logCtx, &app, change, trackingMethod, appInstanceLabelKey, installationID, source); err != nil {
							// Errors while updating the cache are non-fatal since the manifest will simply be regenerated on
							// the next reconciliation.
							logCtx.Warnf("Failed to store cached manifests of previous revision: %v", err)
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

func (a *ArgoCDWebhookHandler) storePreviouslyCachedManifests(logCtx *log.Entry, app *v1alpha1.Application, change changeInfo, trackingMethod string, appInstanceLabelKey string, installationID string, source v1alpha1.ApplicationSource) error {
	destCluster, err := argo.GetDestinationCluster(context.Background(), app.Spec.Destination, a.db)
	if err != nil {
		return fmt.Errorf("error validating destination: %w", err)
	}

	var sourceIntegrity *v1alpha1.SourceIntegrity

	if app.Spec.Project != "" {
		proj, err := a.appProjectsLister.Get(app.Spec.Project)
		if err != nil {
			return err
		}
		sourceIntegrity = proj.EffectiveSourceIntegrity()
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

	if len(refSources) != 0 {
		// TODO: need to support multi source (calculate refSourceCommitSHAs for SetNewRevisionManifests)
		return errors.New("moving manifest cache is currently not supported for multi-source applications")
	}

	oldManifestKey := cache.NewManifestKey(
		change.shaBefore,
		&source,
		refSources,
		app.Spec.Destination.Namespace,
		trackingMethod,
		appInstanceLabelKey,
		app.InstanceName(a.ns),
		installationID,
		sourceIntegrity,
		&clusterInfo,
		nil,
	)
	newManifestKey := oldManifestKey
	newManifestKey.Revision = change.shaAfter

	logEntry := logCtx.WithFields(log.Fields{
		"old_cacheKey": oldManifestKey.String(),
		"new_cacheKey": newManifestKey.String(),
	})

	if err := a.repoCache.SetNewRevisionManifests(oldManifestKey, newManifestKey); err != nil {
		webhookStoreCacheAttemptsTotal.WithLabelValues(git.NormalizeGitURL(source.RepoURL), "false").Inc()
		if errors.Is(err, cache.ErrCacheMiss) {
			logEntry.Info("manifest cache miss while moving manifests cache to the new revision")
			return nil
		}
		return fmt.Errorf("error setting new revision manifests: %w", err)
	}

	webhookStoreCacheAttemptsTotal.WithLabelValues(git.NormalizeGitURL(source.RepoURL), "true").Inc()
	logEntry.Info("manifests cache moved")
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

// lookupRepositoryWithCredsTemplate returns a repository with its credentials for a given URL, falling
// back to the best matching credentials template when no repository secret matches. If neither is
// found, then nil repository is returned.
func (a *ArgoCDWebhookHandler) lookupRepositoryWithCredsTemplate(ctx context.Context, repoURL string) (*v1alpha1.Repository, error) {
	repo, err := a.lookupRepository(ctx, repoURL)
	if err != nil || repo != nil {
		return repo, err
	}
	creds, err := a.db.GetRepositoryCredentials(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("error getting repository credentials for %q: %w", repoURL, err)
	}
	if creds == nil {
		return nil, nil
	}
	log.Debugf("found a matching credentials template for URL %s", repoURL)
	repo = &v1alpha1.Repository{Repo: repoURL}
	repo.CopyCredentialsFrom(creds)
	return repo, nil
}

// RevisionHasChanged reports whether a webhook payload for revision affects targetRevision.
//
// targetRevision is the configured target: "HEAD", a branch or tag name, a fully qualified
// ref such as "refs/heads/main", or a semver constraint such as ">=1.0.0". revision is the
// ref carried by the payload, already reduced by ParseRevision. A targetRevision of "HEAD"
// or "" tracks the repository's default branch, so it is affected exactly when touchedHead
// is true; anything else is compared with CompareRevisions.
func RevisionHasChanged(targetRevision string, revision string, touchedHead bool) bool {
	targetRev := ParseRevision(targetRevision)
	if targetRev == "HEAD" || targetRev == "" { // revision is head
		return touchedHead
	}
	targetRevisionHasPrefixList := []string{"refs/heads/", "refs/tags/"}
	for _, prefix := range targetRevisionHasPrefixList {
		if strings.HasPrefix(targetRevision, prefix) {
			return CompareRevisions(revision, targetRev)
		}
	}

	return CompareRevisions(revision, targetRevision)
}

func CompareRevisions(revision string, targetRevision string) bool {
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

// RepoURLMatches reports whether repoURL refers to the same repository as the webhook payload that
// repoRegexp was built from. A non-match is the normal case, since every configured repository is
// checked against every incoming event, so it is logged at debug level.
func RepoURLMatches(repoURL string, repoRegexp *regexp.Regexp) bool {
	if !repoRegexp.MatchString(repoURL) {
		log.Debugf("%s does not match %s", repoURL, repoRegexp.String())
		return false
	}

	log.Debugf("%s matches %s", repoURL, repoRegexp.String())
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
// bitbucket server from where the webhook originated.
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

// bbServerChangesPageLimit is the page size used when fetching changed files from the Bitbucket Server
// changes API. It minimizes the round trips. If the server admin has configured a lower hard max, the server
// silently caps each page at that value and the pagination loop still terminates correctly via isLastPage.
const bbServerChangesPageLimit = 1000

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
			"limit": bbServerChangesPageLimit,
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

const (
	// adoDiffsAPIVersion pins the Azure DevOps REST API version used for the Diffs endpoint, so the
	// response contract does not change with the server-side default. Moving to a newer version
	// requires a code change.
	// See: https://learn.microsoft.com/en-us/rest/api/azure/devops/git/diffs/get?view=azure-devops-rest-7.1
	adoDiffsAPIVersion = "7.1"

	// adoDiffsMaxPageSize is the maximum number of file changes requested per call to the Diffs API.
	// The endpoint defaults to 100; this value is aligned with the documented maximum of the related
	// Pull Request Iteration Changes endpoint. Pushes exceeding this limit fall back to a full refresh.
	// See: https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-request-iteration-changes/get?view=azure-devops-rest-7.1
	adoDiffsMaxPageSize = 2000

	// adoNullSHA is the all-zeros object ID used by Azure DevOps to signal a non-existent commit.
	// It appears as newObjectId on branch deletion events and as oldObjectId on branch creation events.
	// Diffing against this SHA is not meaningful and the Diffs API returns 404 for such requests.
	adoNullSHA = "0000000000000000000000000000000000000000"

	// adoAPITimeout is the timeout for outbound Azure DevOps REST API calls made during webhook processing.
	adoAPITimeout = 10 * time.Second

	// adoRespDrainLimit bounds how much of an unread response body is discarded so that the
	// connection can be returned to the pool of a cached client.
	adoRespDrainLimit = 512 * 1024

	// adoHTTPClientCacheLimit bounds the number of cached HTTP clients. Clients are keyed by the
	// transport settings of a repository, which usually resolve to a single entry per Azure DevOps
	// host, so the limit is only reached after many TLS or proxy configuration changes without a
	// restart. Beyond it, clients are created per request without keep-alives.
	adoHTTPClientCacheLimit = 64
)

var errNoADOCredentials = errors.New("no supported Azure DevOps REST API credentials configured")

type adoDiffsResponse struct {
	Changes            []adoDiffChange `json:"changes"`
	AllChangesIncluded bool            `json:"allChangesIncluded"`
}

type adoDiffChange struct {
	Item       adoDiffItem `json:"item"`
	ChangeType string      `json:"changeType"`
}

type adoDiffItem struct {
	Path     string `json:"path"`
	IsFolder bool   `json:"isFolder"`
}

func (a *ArgoCDWebhookHandler) azureDevOpsWebhookAuthConfigured() bool {
	// The Azure DevOps webhook parser requires basic auth when either value is configured.
	return a.settings.GetWebhookAzureDevOpsUsername() != "" || a.settings.GetWebhookAzureDevOpsPassword() != ""
}

// lookupAndFetchADOChangedFiles returns the files changed by an Azure DevOps push event, or nil when
// they cannot be determined, in which case all matching applications are refreshed.
// The repository lookup acts as the SSRF guard: the Diffs API is only called for repositories, or
// credentials templates, configured in Argo CD.
func (a *ArgoCDWebhookHandler) lookupAndFetchADOChangedFiles(ctx context.Context, repoURL, repoID, shaBefore, shaAfter string) []string {
	if shaBefore == "" || shaAfter == "" || shaBefore == adoNullSHA || shaAfter == adoNullSHA {
		log.Debugf("skipping Azure DevOps changed files lookup for URL %s because the commit range is empty or includes the null SHA used for branch creation or deletion", repoURL)
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, adoAPITimeout)
	defer cancel()
	argoRepo, err := a.lookupRepositoryWithCredsTemplate(ctx, repoURL)
	if err != nil {
		log.Warnf("error finding repository for Azure DevOps webhook URL %s: %v", repoURL, err)
		return nil
	}
	if argoRepo == nil {
		log.Debugf("no repository or credentials template configured for Azure DevOps webhook URL %s, skipping changed files lookup", repoURL)
		return nil
	}
	changedFiles, err := fetchChangedFilesFromADO(ctx, a.adoHTTPClientFactory(argoRepo), argoRepo, repoID, shaBefore, shaAfter)
	if err != nil {
		if errors.Is(err, errNoADOCredentials) {
			log.Warnf("no supported Azure DevOps REST API credentials configured for repository %s; falling back to full refresh", repoURL)
			return nil
		}
		log.Warnf("error fetching changed files from Azure DevOps diffs API: %v", err)
		return nil
	}
	return changedFiles
}

// parseADOBaseURL extracts the base URL used to construct Azure DevOps REST API requests.
// It supports both the modern format (https://dev.azure.com/{org}/{project}/_git/{repo})
// and the legacy format (https://{org}.visualstudio.com/{project}/_git/{repo}).
// SSH URLs (e.g. git@ssh.dev.azure.com:v3/{org}/{project}/{repo}) are not supported
// because SSH keys cannot be used to authenticate the Azure DevOps REST API.
func parseADOBaseURL(repoURL string) (string, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse Azure DevOps URL %q: %w", repoURL, err)
	}
	parts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 3)
	// Legacy format: https://{org}.visualstudio.com/{project}/_git/{repo}
	// The org is in the host; the first path segment is the project.
	if strings.HasSuffix(u.Host, ".visualstudio.com") {
		if len(parts) < 1 || parts[0] == "" {
			return "", fmt.Errorf("cannot extract project from Azure DevOps URL %q", repoURL)
		}
		return fmt.Sprintf("%s://%s/%s", u.Scheme, u.Host, parts[0]), nil
	}
	// Modern format: https://dev.azure.com/{org}/{project}/_git/{repo}
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("cannot extract organization and project from Azure DevOps URL %q", repoURL)
	}
	return fmt.Sprintf("%s://%s/%s/%s", u.Scheme, u.Host, parts[0], parts[1]), nil
}

// setADOAuthHeader sets the Authorization header on the request based on the repository credentials,
// with the same precedence used for Git operations over HTTPS: a bearer token takes precedence over
// basic auth, and basic auth only needs a password, since Azure DevOps ignores the username when
// authenticating with a personal access token.
// Returns true if credentials were applied, false if none are configured.
func setADOAuthHeader(req *http.Request, repo *v1alpha1.Repository) bool {
	if repo.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+repo.BearerToken)
		return true
	}
	if repo.Password != "" {
		req.SetBasicAuth(repo.Username, repo.Password)
		return true
	}
	return false
}

// adoHTTPClientCache reuses HTTP clients, and therefore their connections, across Azure DevOps
// REST API calls made for repositories that share the same transport settings.
type adoHTTPClientCache struct {
	mu      sync.Mutex
	clients map[string]*http.Client
}

func newADOHTTPClientCache() *adoHTTPClientCache {
	return &adoHTTPClientCache{clients: map[string]*http.Client{}}
}

// get returns an HTTP client honoring the TLS and proxy settings of the repository. Credentials are
// not part of the client and are set on each request, so repositories with different credentials
// can share a client.
func (c *adoHTTPClientCache) get(repo *v1alpha1.Repository) *http.Client {
	key := adoHTTPClientCacheKey(repo)
	c.mu.Lock()
	defer c.mu.Unlock()
	if client, ok := c.clients[key]; ok {
		return client
	}
	client := newADOHTTPClient(repo)
	if len(c.clients) >= adoHTTPClientCacheLimit {
		log.Debugf("Azure DevOps HTTP client cache is full, creating an uncached client for %s", repo.Repo)
		return client
	}
	// Repository HTTP clients disable keep-alives because they are created per request, which would
	// otherwise leave idle connections behind. A cached client is reused, so keep-alives let it
	// reuse connections, and idle ones are closed by the transport's idle timeout.
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.DisableKeepAlives = false
	}
	c.clients[key] = client
	return client
}

// adoHTTPClientCacheKey identifies the settings newADOHTTPClient builds a client from, including
// the custom server certificates, so that TLS and proxy configuration changes take effect without
// a restart.
func adoHTTPClientCacheKey(repo *v1alpha1.Repository) string {
	var host string
	var serverCerts []string
	if u, err := url.Parse(repo.Repo); err == nil {
		host = u.Host
		if !repo.IsInsecure() {
			// Errors are ignored, as newADOHTTPClient falls back to the system roots on error.
			serverCerts, _ = certutil.GetCertificateForConnect(host)
		}
	}
	h := sha256.New()
	parts := append([]string{host, strconv.FormatBool(repo.IsInsecure()), repo.Proxy, repo.NoProxy, repo.TLSClientCertData, repo.TLSClientCertKey}, serverCerts...)
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func newADOHTTPClient(repo *v1alpha1.Repository) *http.Client {
	return git.GetRepoHTTPClient(repo.Repo, repo.IsInsecure(), repo.GetGitCreds(&git.NoopCredsStore{}), repo.Proxy, repo.NoProxy)
}

// fetchChangedFilesFromADO retrieves the list of files changed between two commits by calling
// the Azure DevOps Diffs REST API.
// See: https://learn.microsoft.com/en-us/rest/api/azure/devops/git/diffs/get?view=azure-devops-rest-7.1
func fetchChangedFilesFromADO(ctx context.Context, httpClient *http.Client, repo *v1alpha1.Repository, repoID, shaBefore, shaAfter string) ([]string, error) {
	baseURL, err := parseADOBaseURL(repo.Repo)
	if err != nil {
		return nil, err
	}
	params := url.Values{
		"baseVersion":       {shaBefore},
		"baseVersionType":   {"commit"},
		"targetVersion":     {shaAfter},
		"targetVersionType": {"commit"},
		"$top":              {strconv.Itoa(adoDiffsMaxPageSize)},
		"api-version":       {adoDiffsAPIVersion},
	}
	apiURL := fmt.Sprintf("%s/_apis/git/repositories/%s/diffs/commits?%s",
		baseURL, url.PathEscape(repoID), params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("error creating Azure DevOps diffs API request: %w", err)
	}
	if !setADOAuthHeader(req, repo) {
		return nil, errNoADOCredentials
	}
	log.Debugf("fetching changed files from Azure DevOps diffs API: %s", apiURL)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling Azure DevOps diffs API: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, adoRespDrainLimit))
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("azure DevOps diffs API returned unexpected status %d", resp.StatusCode)
	}
	var diffsResp adoDiffsResponse
	if err := json.NewDecoder(resp.Body).Decode(&diffsResp); err != nil {
		return nil, fmt.Errorf("error decoding Azure DevOps diffs API response: %w", err)
	}
	if !diffsResp.AllChangesIncluded {
		return nil, fmt.Errorf("azure DevOps diffs API response was truncated (more than %d files changed); falling back to full refresh", adoDiffsMaxPageSize)
	}
	changedFiles := make([]string, 0, len(diffsResp.Changes))
	for _, c := range diffsResp.Changes {
		if !c.Item.IsFolder && c.Item.Path != "" {
			changedFiles = append(changedFiles, strings.TrimPrefix(c.Item.Path, "/"))
		}
	}
	log.Debugf("Azure DevOps diffs API returned %d changed files between %s..%s", len(changedFiles), shaBefore, shaAfter)
	return changedFiles, nil
}

func (a *ArgoCDWebhookHandler) Handler(w http.ResponseWriter, r *http.Request) {
	HandleRequest(w, r, a.parsers, a.maxWebhookPayloadSizeB, a.queue)
}

// Shutdown gracefully shuts down the webhook handler by closing queues and waiting for workers
func (a *ArgoCDWebhookHandler) Shutdown() {
	close(a.queue)
	a.refreshQueue.ShutDownWithDrain()
	a.Wait()
}
