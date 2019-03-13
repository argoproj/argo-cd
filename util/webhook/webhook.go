package webhook

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/argoproj/argo-cd/util/previews"

	log "github.com/sirupsen/logrus"
	"gopkg.in/go-playground/webhooks.v3"
	"gopkg.in/go-playground/webhooks.v3/bitbucket"
	"gopkg.in/go-playground/webhooks.v3/github"
	"gopkg.in/go-playground/webhooks.v3/gitlab"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/settings"
)

type ArgoCDWebhookHandler struct {
	ns               string
	appClientset     appclientset.Interface
	previewService   previews.PreviewService
	github           *github.Webhook
	githubHandler    http.Handler
	gitlab           *gitlab.Webhook
	gitlabHandler    http.Handler
	bitbucket        *bitbucket.Webhook
	bitbucketHandler http.Handler
}

func NewHandler(namespace string, appClientset appclientset.Interface, prevewService previews.PreviewService, set *settings.ArgoCDSettings) *ArgoCDWebhookHandler {
	acdWebhook := ArgoCDWebhookHandler{
		ns:             namespace,
		appClientset:   appClientset,
		previewService: prevewService,
		github:         github.New(&github.Config{Secret: set.WebhookGitHubSecret}),
		gitlab:         gitlab.New(&gitlab.Config{Secret: set.WebhookGitLabSecret}),
		bitbucket:      bitbucket.New(&bitbucket.Config{UUID: set.WebhookBitbucketUUID}),
	}
	acdWebhook.github.RegisterEvents(acdWebhook.HandleEvent, github.PushEvent, github.PullRequestEvent)
	acdWebhook.gitlab.RegisterEvents(acdWebhook.HandleEvent, gitlab.PushEvents, gitlab.TagEvents)
	acdWebhook.bitbucket.RegisterEvents(acdWebhook.HandleEvent, bitbucket.RepoPushEvent)
	acdWebhook.githubHandler = webhooks.Handler(acdWebhook.github)
	acdWebhook.gitlabHandler = webhooks.Handler(acdWebhook.gitlab)
	acdWebhook.bitbucketHandler = webhooks.Handler(acdWebhook.bitbucket)
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
	case github.PullRequestPayload:
		// See: https://developer.github.com/v3/activity/events/types/#pullrequestevent
		webURL = payload.Repository.HTMLURL
		revision = parseRef(payload.PullRequest.Head.Ref)
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
	}
	return webURL, revision, touchedHead
}

// HandleEvent handles webhook events for repo push events
func (a *ArgoCDWebhookHandler) HandleEvent(payloadIf interface{}, header webhooks.Header) {
	webURL, revision, touchedHead := affectedRevisionInfo(payloadIf)
	// NOTE: the webURL does not include the .git extension
	if webURL == "" {
		log.Info("Ignoring webhook event")
		return
	}
	log.Infof("Received event repo: %s, revision: %s, touchedHead: %v", webURL, revision, touchedHead)
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
		appName := app.ObjectMeta.Name
		repoUrl := app.Spec.Source.RepoURL
		targetRevision := app.Spec.Source.TargetRevision

		log.Debugf("matching with appName=%s, revision=%s, targetRevision=%s", appName, revision, targetRevision)
		if !repoRegexp.MatchString(repoUrl) {
			log.Debugf("repoRegexp=%s, does not match repoUrl=%s", regexpStr, repoUrl)
			continue
		}

		switch payload := payloadIf.(type) {
		case github.PushPayload, gitlab.PushEventPayload:
			if targetRevision == "HEAD" || targetRevision == "" {
				if !touchedHead {
					log.Debugf("targetRevision=%s does not match revision=%s", targetRevision, revision)
					continue
				}
			} else if targetRevision != revision {
				log.Debugf("targetRevision=%s does not match revision=%s", targetRevision, revision)
				continue
			}
			log.Infof("pushed change, time to sync the app")
			_, err = argo.RefreshApp(appIf, appName, v1alpha1.RefreshTypeNormal)
			if err != nil {
				log.Warnf("Failed to refresh app '%s' for controller reprocessing: %v", appName, err)
			}
		case github.PullRequestPayload:

			if targetRevision != payload.PullRequest.Base.Ref {
				log.Debugf("targetRevision=%s matches revision=%s", targetRevision, revision)
				continue
			}

			log.Infof("payload.action=%s", payload.Action)

			preview := v1alpha1.Preview{
				Owner:    payload.Repository.Owner.Login,
				Repo:     payload.Repository.Name,
				Revision: revision,
			}

			switch payload.Action {
			case "opened", "reopened":
				log.Infof("opened PR, time to set-up a new preview app")

				err := a.previewService.Create(app, preview, payload.PullRequest.Head.Sha)
				if err != nil {
					log.Warnf("Failed to create preview app appName=%s, err=%v", appName, err)
				}

				_, err = argo.RefreshApp(appIf, appName, v1alpha1.RefreshTypeNormal)
				if err != nil {
					log.Warnf("Failed to refresh app '%s' for controller reprocessing: %v", appName, err)
				}
			case "closed":
				log.Infof("closed PR, time to tear-down the preview app")
				err := a.previewService.Delete(app, preview)
				if err != nil {
					log.Warnf("Failed to delete preview app appName=%s, err=%v", appName, err)
				}
			}
		}
	}
}

func (a *ArgoCDWebhookHandler) Handler(w http.ResponseWriter, r *http.Request) {
	event := r.Header.Get("X-GitHub-Event")
	if len(event) > 0 {
		a.githubHandler.ServeHTTP(w, r)
		return
	}
	event = r.Header.Get("X-Gitlab-Event")
	if len(event) > 0 {
		a.gitlabHandler.ServeHTTP(w, r)
		return
	}
	uuid := r.Header.Get("X-Hook-UUID")
	if len(uuid) > 0 {
		a.bitbucketHandler.ServeHTTP(w, r)
		return
	}
	log.Debug("Ignoring unknown webhook event")
}
