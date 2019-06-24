package webhook

import (
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
func affectedRevisionInfo(payloadIf interface{}) (string, string, bool) {
	var webURL string
	var revision string
	var touchedHead bool

	parseRef := func(ref string) string {
		refParts := strings.SplitN(ref, "/", 3)
		return refParts[len(refParts)-1]
	}

	switch payload := payloadIf.(type) {
	case github.PushPayload:
		// See: https://developer.github.com/v3/activity/events/types/#pushevent
		webURL = payload.Repository.HTMLURL
		revision = parseRef(payload.Ref)
		touchedHead = bool(payload.Repository.DefaultBranch == revision)
	case gitlab.PushEventPayload:
		// See: https://docs.gitlab.com/ee/user/project/integrations/webhooks.html
		// NOTE: this is untested
		webURL = payload.Project.WebURL
		revision = parseRef(payload.Ref)
		touchedHead = bool(payload.Project.DefaultBranch == revision)
	case gitlab.TagEventPayload:
		// See: https://docs.gitlab.com/ee/user/project/integrations/webhooks.html
		// NOTE: this is untested
		webURL = payload.Project.WebURL
		revision = parseRef(payload.Ref)
		touchedHead = bool(payload.Project.DefaultBranch == revision)
	case bitbucket.RepoPushPayload:
		// See: https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Push
		// NOTE: this is untested
		webURL = payload.Repository.Links.HTML.Href
		// TODO: bitbucket includes multiple changes as part of a single event.
		// We only pick the first but need to consider how to handle multiple
		for _, change := range payload.Push.Changes {
			revision = change.New.Name
			break
		}
		// Not actually sure how to check if the incoming change affected HEAD just by examining the
		// payload alone. To be safe, we just return true and let the controller check for himself.
		touchedHead = true
	case bitbucketserver.RepositoryReferenceChangedPayload:

		// Webhook module does not parse the inner links
		for _, l := range payload.Repository.Links["clone"].([]interface{}) {
			link := l.(map[string]interface{})
			if link["name"] == "http" {
				webURL = link["href"].(string)
				break
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

	case gogsclient.PushPayload:
		webURL = payload.Repo.HTMLURL
		revision = parseRef(payload.Ref)
		touchedHead = bool(payload.Repo.DefaultBranch == revision)
	}
	return webURL, revision, touchedHead
}

// HandleEvent handles webhook events for repo push events
func (a *ArgoCDWebhookHandler) HandleEvent(payload interface{}) {
	webURL, revision, touchedHead := affectedRevisionInfo(payload)
	// NOTE: the webURL does not include the .git extension
	if webURL == "" {
		log.Info("Ignoring webhook event")
		return
	}
	log.Infof("Received push event repo: %s, revision: %s, touchedHead: %v", webURL, revision, touchedHead)
	appIf := a.appClientset.ArgoprojV1alpha1().Applications(a.ns)
	apps, err := appIf.List(metav1.ListOptions{})
	if err != nil {
		log.Warnf("Failed to list applications: %v", err)
		return
	}
	urlObj, err := url.Parse(webURL)
	if err != nil {
		log.Warnf("Failed to parse repoURL '%s'", webURL)
		return
	}
	regexpStr := "(?i)(http://|https://|git@)" + urlObj.Host + "[:/]" + urlObj.Path[1:] + "(\\.git)?"
	repoRegexp, err := regexp.Compile(regexpStr)
	if err != nil {
		log.Warn("Failed to compile repoURL regexp")
		return
	}

	for _, app := range apps.Items {
		if !repoRegexp.MatchString(app.Spec.Source.RepoURL) {
			log.Debugf("%s does not match", app.Spec.Source.RepoURL)
			continue
		}
		targetRev := app.Spec.Source.TargetRevision
		if targetRev == "HEAD" || targetRev == "" {
			if !touchedHead {
				continue
			}
		} else if targetRev != revision {
			continue
		}
		_, err = argo.RefreshApp(appIf, app.ObjectMeta.Name, v1alpha1.RefreshTypeNormal)
		if err != nil {
			log.Warnf("Failed to refresh app '%s' for controller reprocessing: %v", app.ObjectMeta.Name, err)
			continue
		}
	}
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
