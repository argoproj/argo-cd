package webhook

import (
	"context"
	"errors"
	"fmt"
	"github.com/argoproj/argo-cd/v2/util/glob"
	"html"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	gogsclient "github.com/gogits/go-gogs-client"
	log "github.com/sirupsen/logrus"
	"gopkg.in/go-playground/webhooks.v5/bitbucket"
	bitbucketserver "gopkg.in/go-playground/webhooks.v5/bitbucket-server"
	"gopkg.in/go-playground/webhooks.v5/github"
	"gopkg.in/go-playground/webhooks.v5/gitlab"
	"gopkg.in/go-playground/webhooks.v5/gogs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v2/reposerver/cache"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/security"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

type settingsSource interface {
	GetAppInstanceLabelKey() (string, error)
	GetTrackingMethod() (string, error)
}

// https://www.rfc-editor.org/rfc/rfc3986#section-3.2.1
// https://github.com/shadow-maint/shadow/blob/master/libmisc/chkname.c#L36
const usernameRegex = `[a-zA-Z0-9_\.][a-zA-Z0-9_\.-]{0,30}[a-zA-Z0-9_\.\$-]?`

var _ settingsSource = &settings.SettingsManager{}

type ArgoCDWebhookHandler struct {
	repoCache       *cache.Cache
	serverCache     *servercache.Cache
	db              db.ArgoDB
	ns              string
	appNs           []string
	appClientset    appclientset.Interface
	github          *github.Webhook
	gitlab          *gitlab.Webhook
	bitbucket       *bitbucket.Webhook
	bitbucketserver *bitbucketserver.Webhook
	gogs            *gogs.Webhook
	settingsSrc     settingsSource
}

func NewHandler(namespace string, applicationNamespaces []string, appClientset appclientset.Interface, set *settings.ArgoCDSettings, settingsSrc settingsSource, repoCache *cache.Cache, serverCache *servercache.Cache, argoDB db.ArgoDB) *ArgoCDWebhookHandler {
	githubWebhook, err := github.New(github.Options.Secret(set.WebhookGitHubSecret))
	if err != nil {
		log.Warnf("Unable to init the GitHub webhook")
	}
	gitlabWebhook, err := gitlab.New(gitlab.Options.Secret(set.WebhookGitLabSecret))
	if err != nil {
		log.Warnf("Unable to init the GitLab webhook")
	}
	bitbucketWebhook, err := bitbucket.New(bitbucket.Options.UUID(set.WebhookBitbucketUUID))
	if err != nil {
		log.Warnf("Unable to init the Bitbucket webhook")
	}
	bitbucketserverWebhook, err := bitbucketserver.New(bitbucketserver.Options.Secret(set.WebhookBitbucketServerSecret))
	if err != nil {
		log.Warnf("Unable to init the Bitbucket Server webhook")
	}
	gogsWebhook, err := gogs.New(gogs.Options.Secret(set.WebhookGogsSecret))
	if err != nil {
		log.Warnf("Unable to init the Gogs webhook")
	}

	acdWebhook := ArgoCDWebhookHandler{
		ns:              namespace,
		appNs:           applicationNamespaces,
		appClientset:    appClientset,
		github:          githubWebhook,
		gitlab:          gitlabWebhook,
		bitbucket:       bitbucketWebhook,
		bitbucketserver: bitbucketserverWebhook,
		gogs:            gogsWebhook,
		settingsSrc:     settingsSrc,
		repoCache:       repoCache,
		serverCache:     serverCache,
		db:              argoDB,
	}

	return &acdWebhook
}

func parseRevision(ref string) string {
	refParts := strings.SplitN(ref, "/", 3)
	return refParts[len(refParts)-1]
}

// affectedRevisionInfo examines a payload from a webhook event, and extracts the repo web URL,
// the revision, and whether or not this affected origin/HEAD (the default branch of the repository)
func affectedRevisionInfo(payloadIf interface{}) (webURLs []string, revision string, change changeInfo, touchedHead bool, changedFiles []string) {
	switch payload := payloadIf.(type) {
	case github.PushPayload:
		// See: https://developer.github.com/v3/activity/events/types/#pushevent
		webURLs = append(webURLs, payload.Repository.HTMLURL)
		revision = parseRevision(payload.Ref)
		change.shaAfter = parseRevision(payload.After)
		change.shaBefore = parseRevision(payload.Before)
		touchedHead = bool(payload.Repository.DefaultBranch == revision)
		for _, commit := range payload.Commits {
			changedFiles = append(changedFiles, commit.Added...)
			changedFiles = append(changedFiles, commit.Modified...)
			changedFiles = append(changedFiles, commit.Removed...)
		}
	case gitlab.PushEventPayload:
		// See: https://docs.gitlab.com/ee/user/project/integrations/webhooks.html
		webURLs = append(webURLs, payload.Project.WebURL)
		revision = parseRevision(payload.Ref)
		change.shaAfter = parseRevision(payload.After)
		change.shaBefore = parseRevision(payload.Before)
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
		revision = parseRevision(payload.Ref)
		change.shaAfter = parseRevision(payload.After)
		change.shaBefore = parseRevision(payload.Before)
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
		// TODO: bitbucket includes multiple changes as part of a single event.
		// We only pick the first but need to consider how to handle multiple
		for _, change := range payload.Push.Changes {
			revision = change.New.Name
			break
		}
		// Not actually sure how to check if the incoming change affected HEAD just by examining the
		// payload alone. To be safe, we just return true and let the controller check for himself.
		touchedHead = true

	// Bitbucket does not include a list of changed files anywhere in it's payload
	// so we cannot update changedFiles for this type of payload
	case bitbucketserver.RepositoryReferenceChangedPayload:

		// Webhook module does not parse the inner links
		if payload.Repository.Links != nil {
			for _, l := range payload.Repository.Links["clone"].([]interface{}) {
				link := l.(map[string]interface{})
				if link["name"] == "http" {
					webURLs = append(webURLs, link["href"].(string))
				}
				if link["name"] == "ssh" {
					webURLs = append(webURLs, link["href"].(string))
				}
			}
		}

		// TODO: bitbucket includes multiple changes as part of a single event.
		// We only pick the first but need to consider how to handle multiple
		for _, change := range payload.Changes {
			revision = parseRevision(change.Reference.ID)
			break
		}
		// Not actually sure how to check if the incoming change affected HEAD just by examining the
		// payload alone. To be safe, we just return true and let the controller check for himself.
		touchedHead = true

		// Bitbucket does not include a list of changed files anywhere in it's payload
		// so we cannot update changedFiles for this type of payload

	case gogsclient.PushPayload:
		webURLs = append(webURLs, payload.Repo.HTMLURL)
		revision = parseRevision(payload.Ref)
		change.shaAfter = parseRevision(payload.After)
		change.shaBefore = parseRevision(payload.Before)
		touchedHead = bool(payload.Repo.DefaultBranch == revision)
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
func (a *ArgoCDWebhookHandler) HandleEvent(payload interface{}) {
	webURLs, revision, change, touchedHead, changedFiles := affectedRevisionInfo(payload)
	// NOTE: the webURL does not include the .git extension
	if len(webURLs) == 0 {
		log.Info("Ignoring webhook event")
		return
	}
	for _, webURL := range webURLs {
		log.Infof("Received push event repo: %s, revision: %s, touchedHead: %v", webURL, revision, touchedHead)
	}

	nsFilter := a.ns
	if len(a.appNs) > 0 {
		// Retrieve app from all namespaces
		nsFilter = ""
	}

	appIf := a.appClientset.ArgoprojV1alpha1().Applications(nsFilter)
	apps, err := appIf.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Warnf("Failed to list applications: %v", err)
		return
	}

	trackingMethod, err := a.settingsSrc.GetTrackingMethod()
	if err != nil {
		log.Warnf("Failed to get trackingMethod: %v", err)
		return
	}
	appInstanceLabelKey, err := a.settingsSrc.GetAppInstanceLabelKey()
	if err != nil {
		log.Warnf("Failed to get appInstanceLabelKey: %v", err)
		return
	}

	// Skip any application that is neither in the control plane's namespace
	// nor in the list of enabled namespaces.
	var filteredApps []v1alpha1.Application
	for _, app := range apps.Items {
		if app.Namespace == a.ns || glob.MatchStringInList(a.appNs, app.Namespace, false) {
			filteredApps = append(filteredApps, app)
		}
	}

	for _, webURL := range webURLs {
		repoRegexp, err := getWebUrlRegex(webURL)
		if err != nil {
			log.Warnf("Failed to get repoRegexp: %s", err)
			continue
		}
		for _, app := range filteredApps {

			for _, source := range app.Spec.GetSources() {
				if sourceRevisionHasChanged(source, revision, touchedHead) && sourceUsesURL(source, webURL, repoRegexp) {
					if appFilesHaveChanged(&app, changedFiles) {
						_, err = argo.RefreshApp(appIf, app.ObjectMeta.Name, v1alpha1.RefreshTypeNormal)
						if err != nil {
							log.Warnf("Failed to refresh app '%s' for controller reprocessing: %v", app.ObjectMeta.Name, err)
							continue
						}
						// No need to refresh multiple times if multiple sources match.
						break
					} else if change.shaBefore != "" && change.shaAfter != "" {
						if err := a.storePreviouslyCachedManifests(&app, change, trackingMethod, appInstanceLabelKey); err != nil {
							log.Warnf("Failed to store cached manifests of previous revision for app '%s': %v", app.Name, err)
						}
					}
				}
			}
		}
	}
}

// getWebUrlRegex compiles a regex that will match any targetRevision referring to the same repo as the given webURL.
// webURL is expected to be a URL from an SCM webhook payload pointing to the web page for the repo.
func getWebUrlRegex(webURL string) (*regexp.Regexp, error) {
	urlObj, err := url.Parse(webURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repoURL '%s'", webURL)
	}

	regexEscapedHostname := regexp.QuoteMeta(urlObj.Hostname())
	regexEscapedPath := regexp.QuoteMeta(urlObj.Path[1:])
	regexpStr := fmt.Sprintf(`(?i)^(http://|https://|%s@|ssh://(%s@)?)%s(:[0-9]+|)[:/]%s(\.git)?$`,
		usernameRegex, usernameRegex, regexEscapedHostname, regexEscapedPath)
	repoRegexp, err := regexp.Compile(regexpStr)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regexp for repoURL '%s'", webURL)
	}

	return repoRegexp, nil
}

func (a *ArgoCDWebhookHandler) storePreviouslyCachedManifests(app *v1alpha1.Application, change changeInfo, trackingMethod string, appInstanceLabelKey string) error {
	err := argo.ValidateDestination(context.Background(), &app.Spec.Destination, a.db)
	if err != nil {
		return fmt.Errorf("error validating destination: %w", err)
	}

	var clusterInfo v1alpha1.ClusterInfo
	err = a.serverCache.GetClusterInfo(app.Spec.Destination.Server, &clusterInfo)
	if err != nil {
		return fmt.Errorf("error getting cluster info: %w", err)
	}

	refSources, err := argo.GetRefSources(context.Background(), app.Spec, a.db)
	if err != nil {
		return fmt.Errorf("error getting ref sources: %w", err)
	}
	source := app.Spec.GetSource()
	cache.LogDebugManifestCacheKeyFields("getting manifests cache", "webhook app revision changed", change.shaBefore, &source, refSources, &clusterInfo, app.Spec.Destination.Namespace, trackingMethod, appInstanceLabelKey, app.Name, nil)

	var cachedManifests cache.CachedManifestResponse
	if err := a.repoCache.GetManifests(change.shaBefore, &source, refSources, &clusterInfo, app.Spec.Destination.Namespace, trackingMethod, appInstanceLabelKey, app.Name, &cachedManifests, nil); err != nil {
		return err
	}

	cache.LogDebugManifestCacheKeyFields("setting manifests cache", "webhook app revision changed", change.shaAfter, &source, refSources, &clusterInfo, app.Spec.Destination.Namespace, trackingMethod, appInstanceLabelKey, app.Name, nil)

	if err = a.repoCache.SetManifests(change.shaAfter, &source, refSources, &clusterInfo, app.Spec.Destination.Namespace, trackingMethod, appInstanceLabelKey, app.Name, &cachedManifests, nil); err != nil {
		return err
	}
	return nil
}

func getAppRefreshPaths(app *v1alpha1.Application) []string {
	var paths []string
	if val, ok := app.Annotations[v1alpha1.AnnotationKeyManifestGeneratePaths]; ok && val != "" {
		for _, item := range strings.Split(val, ";") {
			if item == "" {
				continue
			}
			if filepath.IsAbs(item) {
				paths = append(paths, item[1:])
			} else {
				for _, source := range app.Spec.GetSources() {
					paths = append(paths, filepath.Clean(filepath.Join(source.Path, item)))
				}
			}
		}
	}
	return paths
}

func appFilesHaveChanged(app *v1alpha1.Application, changedFiles []string) bool {
	// an empty slice of changed files means that the payload didn't include a list
	// of changed files and w have to assume that a refresh is required
	if len(changedFiles) == 0 {
		return true
	}

	// Check to see if the app has requested refreshes only on a specific prefix
	refreshPaths := getAppRefreshPaths(app)

	if len(refreshPaths) == 0 {
		// Apps without a given refreshed paths always be refreshed, regardless of changed files
		// this is the "default" behavior
		return true
	}

	// At last one changed file must be under refresh path
	for _, f := range changedFiles {
		f = ensureAbsPath(f)
		for _, item := range refreshPaths {
			item = ensureAbsPath(item)
			changed := false
			if f == item {
				changed = true
			} else if _, err := security.EnforceToCurrentRoot(item, f); err == nil {
				changed = true
			}
			if changed {
				log.WithField("application", app.Name).Debugf("Application uses files that have changed")
				return true
			}
		}
	}

	log.WithField("application", app.Name).Debugf("Application does not use any of the files that have changed")
	return false
}

func ensureAbsPath(input string) string {
	if !filepath.IsAbs(input) {
		return string(filepath.Separator) + input
	}
	return input
}

func sourceRevisionHasChanged(source v1alpha1.ApplicationSource, revision string, touchedHead bool) bool {
	targetRev := parseRevision(source.TargetRevision)
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

func (a *ArgoCDWebhookHandler) Handler(w http.ResponseWriter, r *http.Request) {

	var payload interface{}
	var err error

	switch {
	//Gogs needs to be checked before GitHub since it carries both Gogs and (incompatible) GitHub headers
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
		payload, err = a.gitlab.Parse(r, gitlab.PushEvents, gitlab.TagEvents)
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
		http.Error(w, "Unknown webhook event", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Infof("Webhook processing failed: %s", err)
		status := http.StatusBadRequest
		if r.Method != http.MethodPost {
			status = http.StatusMethodNotAllowed
		}
		http.Error(w, fmt.Sprintf("Webhook processing failed: %s", html.EscapeString(err.Error())), status)
		return
	}

	a.HandleEvent(payload)
}
