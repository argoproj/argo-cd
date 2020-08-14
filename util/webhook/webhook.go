package webhook

import (
	"context"
	"net/http"
	"net/url"
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

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/settings"
)

type ArgoCDWebhookHandler struct {
	ns              string
	appClientset    appclientset.Interface
	github          *github.Webhook
	gitlab          *gitlab.Webhook
	bitbucket       *bitbucket.Webhook
	bitbucketserver *bitbucketserver.Webhook
	gogs            *gogs.Webhook
}

func NewHandler(namespace string, appClientset appclientset.Interface, set *settings.ArgoCDSettings) *ArgoCDWebhookHandler {

	githubWebhook, err := github.New(github.Options.Secret(set.WebhookGitHubSecret))
	if err != nil {
		log.Warnf("Unable to init the Github webhook")
	}
	gitlabWebhook, err := gitlab.New(gitlab.Options.Secret(set.WebhookGitLabSecret))
	if err != nil {
		log.Warnf("Unable to init the Gitlab webhook")
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
		appClientset:    appClientset,
		github:          githubWebhook,
		gitlab:          gitlabWebhook,
		bitbucket:       bitbucketWebhook,
		bitbucketserver: bitbucketserverWebhook,
		gogs:            gogsWebhook,
	}

	return &acdWebhook
}

// affectedRevisionInfo examines a payload from a webhook event, and extracts the repo web URL,
// the revision, and whether or not this affected origin/HEAD (the default branch of the repository)
func affectedRevisionInfo(payloadIf interface{}) (webURLs []string, revision string, touchedHead bool, changedFiles []string) {
	parseRef := func(ref string) string {
		refParts := strings.SplitN(ref, "/", 3)
		return refParts[len(refParts)-1]
	}

	switch payload := payloadIf.(type) {
	case github.PushPayload:
		// See: https://developer.github.com/v3/activity/events/types/#pushevent
		webURLs = append(webURLs, payload.Repository.HTMLURL)
		revision = parseRef(payload.Ref)
		touchedHead = bool(payload.Repository.DefaultBranch == revision)
		for _, commit := range payload.Commits {
			changedFiles = append(changedFiles, commit.Added...)
			changedFiles = append(changedFiles, commit.Modified...)
			changedFiles = append(changedFiles, commit.Removed...)
		}
	case gitlab.PushEventPayload:
		// See: https://docs.gitlab.com/ee/user/project/integrations/webhooks.html
		webURLs = append(webURLs, payload.Project.WebURL)
		revision = parseRef(payload.Ref)
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
		revision = parseRef(payload.Ref)
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
		for _, l := range payload.Repository.Links["clone"].([]interface{}) {
			link := l.(map[string]interface{})
			if link["name"] == "http" {
				webURLs = append(webURLs, link["href"].(string))
			}
			if link["name"] == "ssh" {
				webURLs = append(webURLs, link["href"].(string))
			}
		}

		// TODO: bitbucket includes multiple changes as part of a single event.
		// We only pick the first but need to consider how to handle multiple
		for _, change := range payload.Changes {
			revision = parseRef(change.Reference.ID)
			break
		}
		// Not actually sure how to check if the incoming change affected HEAD just by examining the
		// payload alone. To be safe, we just return true and let the controller check for himself.
		touchedHead = true

		// Bitbucket does not include a list of changed files anywhere in it's payload
		// so we cannot update changedFiles for this type of payload

	case gogsclient.PushPayload:
		webURLs = append(webURLs, payload.Repo.HTMLURL)
		revision = parseRef(payload.Ref)
		touchedHead = bool(payload.Repo.DefaultBranch == revision)
		for _, commit := range payload.Commits {
			changedFiles = append(changedFiles, commit.Added...)
			changedFiles = append(changedFiles, commit.Modified...)
			changedFiles = append(changedFiles, commit.Removed...)
		}
	}
	return webURLs, revision, touchedHead, changedFiles
}

// HandleEvent handles webhook events for repo push events
func (a *ArgoCDWebhookHandler) HandleEvent(payload interface{}) {
	webURLs, revision, touchedHead, changedFiles := affectedRevisionInfo(payload)
	// NOTE: the webURL does not include the .git extension
	if len(webURLs) == 0 {
		log.Info("Ignoring webhook event")
		return
	}
	for _, webURL := range webURLs {
		log.Infof("Received push event repo: %s, revision: %s, touchedHead: %v", webURL, revision, touchedHead)
	}
	appIf := a.appClientset.ArgoprojV1alpha1().Applications(a.ns)
	apps, err := appIf.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Warnf("Failed to list applications: %v", err)
		return
	}

	for _, webURL := range webURLs {
		urlObj, err := url.Parse(webURL)
		if err != nil {
			log.Warnf("Failed to parse repoURL '%s'", webURL)
			continue
		}

		regexpStr := `(?i)(http://|https://|\w+@|ssh://(\w+@)?)` + urlObj.Host + "(:[0-9]+|)[:/]" + urlObj.Path[1:] + "(\\.git)?"
		repoRegexp, err := regexp.Compile(regexpStr)
		if err != nil {
			log.Warnf("Failed to compile regexp for repoURL '%s'", webURL)
			continue
		}

		for _, app := range apps.Items {
			if appRevisionHasChanged(&app, revision, touchedHead) && appUsesURL(&app, webURL, repoRegexp) && appFilesHaveChanged(&app, changedFiles) {
				_, err = argo.RefreshApp(appIf, app.ObjectMeta.Name, v1alpha1.RefreshTypeNormal)
				if err != nil {
					log.Warnf("Failed to refresh app '%s' for controller reprocessing: %v", app.ObjectMeta.Name, err)
					continue
				}
			}
		}
	}
}

func getAppRefreshPrefix(app *v1alpha1.Application) string {
	// an explicitPrefix setting always takes precence
	if explicitPrefix := app.Annotations[common.AnnotationKeyRefreshPrefix]; explicitPrefix != "" {
		return explicitPrefix
	}

	// use app path when asked
	if app.Annotations[common.AnnotationKeyRefreshPathUpdatesOnly] == "true" {
		return app.Spec.Source.Path
	}

	// default to an empty prefix
	return ""
}

func appFilesHaveChanged(app *v1alpha1.Application, changedFiles []string) bool {
	// an ampty slice of changed files means that the payload didn't include a list
	// of changed files and w have to assume that a refresh is required
	if len(changedFiles) == 0 {
		return true
	}

	// Check to see if the app has requested refreshes only on a specific prefix
	refreshPrefix := getAppRefreshPrefix(app)

	if refreshPrefix == "" {
		// Apps without a given prefix should always be refreshed, regardless of changed files
		// this is the "default" behavior
		return true
	}

	// At last one changed file must have match the given prefix
	for _, f := range changedFiles {
		if strings.HasPrefix(f, refreshPrefix) {
			log.WithField("application", app.Name).Debugf("Application uses files that have changed")
			return true
		}
	}

	log.WithField("application", app.Name).Debugf("Application does not use any of the files that have changed")
	return false
}

func appRevisionHasChanged(app *v1alpha1.Application, revision string, touchedHead bool) bool {
	targetRev := app.Spec.Source.TargetRevision
	if targetRev == "HEAD" || targetRev == "" { // revision is head
		if !touchedHead { // and head has not updated
			return false // revision has not changed
		}
	}

	return targetRev != revision
}

func appUsesURL(app *v1alpha1.Application, webURL string, repoRegexp *regexp.Regexp) bool {
	if !repoRegexp.MatchString(app.Spec.Source.RepoURL) {
		log.Debugf("%s does not match %s", app.Spec.Source.RepoURL, repoRegexp.String())
		return false
	}

	log.Debugf("%s uses repoURL %s", app.Spec.Source.RepoURL, webURL)
	return true
}

func (a *ArgoCDWebhookHandler) Handler(w http.ResponseWriter, r *http.Request) {

	var payload interface{}
	var err error

	switch {
	//Gogs needs to be checked before Github since it carries both Gogs and (incompatible) Github headers
	case r.Header.Get("X-Gogs-Event") != "":
		payload, err = a.gogs.Parse(r, gogs.PushEvent)
	case r.Header.Get("X-GitHub-Event") != "":
		payload, err = a.github.Parse(r, github.PushEvent)
	case r.Header.Get("X-Gitlab-Event") != "":
		payload, err = a.gitlab.Parse(r, gitlab.PushEvents, gitlab.TagEvents)
	case r.Header.Get("X-Hook-UUID") != "":
		payload, err = a.bitbucket.Parse(r, bitbucket.RepoPushEvent)
	case r.Header.Get("X-Event-Key") != "":
		payload, err = a.bitbucketserver.Parse(r, bitbucketserver.RepositoryReferenceChangedEvent)
	default:
		log.Debug("Ignoring unknown webhook event")
		return
	}

	if err != nil {
		log.Infof("Webhook processing failed: %s", err)
		return
	}

	a.HandleEvent(payload)
}
