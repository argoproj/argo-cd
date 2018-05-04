package webhook

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/common"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/settings"
	log "github.com/sirupsen/logrus"
	webhooks "gopkg.in/go-playground/webhooks.v3"
	"gopkg.in/go-playground/webhooks.v3/bitbucket"
	"gopkg.in/go-playground/webhooks.v3/github"
	"gopkg.in/go-playground/webhooks.v3/gitlab"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ArgoCDWebhookHandler struct {
	ns               string
	appClientset     appclientset.Interface
	github           *github.Webhook
	githubHandler    http.Handler
	gitlab           *gitlab.Webhook
	gitlabHandler    http.Handler
	bitbucket        *bitbucket.Webhook
	bitbucketHandler http.Handler
}

func NewHandler(namespace string, appClientset appclientset.Interface, set settings.ArgoCDSettings) *ArgoCDWebhookHandler {
	acdWebhook := ArgoCDWebhookHandler{
		ns:           namespace,
		appClientset: appClientset,
		github:       github.New(&github.Config{Secret: set.WebhookGitHubSecret}),
		gitlab:       gitlab.New(&gitlab.Config{Secret: set.WebhookGitLabSecret}),
		bitbucket:    bitbucket.New(&bitbucket.Config{UUID: set.WebhookBitbucketUUID}),
	}
	acdWebhook.github.RegisterEvents(acdWebhook.HandleEvent, github.PushEvent)
	acdWebhook.gitlab.RegisterEvents(acdWebhook.HandleEvent, gitlab.PushEvents, gitlab.TagEvents)
	acdWebhook.bitbucket.RegisterEvents(acdWebhook.HandleEvent, bitbucket.RepoPushEvent)
	acdWebhook.githubHandler = webhooks.Handler(acdWebhook.github)
	acdWebhook.gitlabHandler = webhooks.Handler(acdWebhook.gitlab)
	acdWebhook.bitbucketHandler = webhooks.Handler(acdWebhook.bitbucket)
	return &acdWebhook
}

// HandleEvent handles webhook events for repo push events
func (a *ArgoCDWebhookHandler) HandleEvent(payloadIf interface{}, header webhooks.Header) {
	var repoURL string
	var revision string
	touchedHead := false
	switch payload := payloadIf.(type) {
	case github.PushPayload:
		repoURL = payload.Repository.HTMLURL
		refParts := strings.SplitN(payload.Ref, "/", 3)
		revision = refParts[len(refParts)-1]
		if payload.Repository.DefaultBranch == revision {
			touchedHead = true
		}
	case gitlab.PushEventPayload:
		// TODO
		return
	case gitlab.TagEventPayload:
		// TODO
		return
	case bitbucket.RepoPushPayload:
		// TODO
		return
	default:
		log.Info("Ignoring webhook event")
		return
	}
	log.Infof("Received push event repo: %s, revision: %s, touchedHead: %v", repoURL, revision, touchedHead)
	appIf := a.appClientset.ArgoprojV1alpha1().Applications(a.ns)
	apps, err := appIf.List(metav1.ListOptions{})
	if err != nil {
		log.Warnf("Failed to list applications: %v", err)
		return
	}
	urlObj, err := url.Parse(repoURL)
	if err != nil {
		log.Warnf("Failed to parse repoURL '%s'", repoURL)
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
			log.Infof("%s does not match", app.Spec.Source.RepoURL)
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
		if app.ObjectMeta.Annotations == nil {
			app.ObjectMeta.Annotations = make(map[string]string)
		}
		app.ObjectMeta.Annotations[common.AnnotationKeyRefresh] = time.Now().String()
		_, err = appIf.Update(&app)
		if err != nil {
			log.Warnf("Failed to refresh app '%s' for controller reprocessing: %v", app.ObjectMeta.Name, err)
			continue
		}
		log.Infof("Refreshed app '%s' for controller reprocessing", app.ObjectMeta.Name)
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
