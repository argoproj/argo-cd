package utils

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
	argosettings "github.com/argoproj/argo-cd/v2/util/settings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/go-playground/webhooks.v5/github"
	"gopkg.in/go-playground/webhooks.v5/gitlab"
)

type WebhookHandler struct {
	namespace string
	github    *github.Webhook
	gitlab    *gitlab.Webhook
	client    client.Client
}

type gitGeneratorInfo struct {
	Revision    string
	TouchedHead bool
	RepoRegexp  *regexp.Regexp
}

type prGeneratorInfo struct {
	Github *prGeneratorGithubInfo
}

type prGeneratorGithubInfo struct {
	Repo      string
	Owner     string
	APIRegexp *regexp.Regexp
}

func NewWebhookHandler(namespace string, argocdSettingsMgr *argosettings.SettingsManager, client client.Client) (*WebhookHandler, error) {
	// register the webhook secrets stored under "argocd-secret" for verifying incoming payloads
	argocdSettings, err := argocdSettingsMgr.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("Failed to get argocd settings: %v", err)
	}
	githubHandler, err := github.New(github.Options.Secret(argocdSettings.WebhookGitHubSecret))
	if err != nil {
		return nil, fmt.Errorf("Unable to init GitHub webhook: %v", err)
	}
	gitlabHandler, err := gitlab.New(gitlab.Options.Secret(argocdSettings.WebhookGitLabSecret))
	if err != nil {
		return nil, fmt.Errorf("Unable to init GitLab webhook: %v", err)
	}

	return &WebhookHandler{
		namespace: namespace,
		github:    githubHandler,
		gitlab:    gitlabHandler,
		client:    client,
	}, nil
}

func (h *WebhookHandler) HandleEvent(payload interface{}) {
	gitGenInfo := getGitGeneratorInfo(payload)
	prGenInfo := getPRGeneratorInfo(payload)
	if gitGenInfo == nil && prGenInfo == nil {
		return
	}

	appSetList := &v1alpha1.ApplicationSetList{}
	err := h.client.List(context.Background(), appSetList, &client.ListOptions{})
	if err != nil {
		log.Errorf("Failed to list applicationsets: %v", err)
		return
	}

	for _, appSet := range appSetList.Items {
		shouldRefresh := false
		for _, gen := range appSet.Spec.Generators {
			// check if the ApplicationSet uses any generator that is relevant to the payload
			shouldRefresh = shouldRefreshGitGenerator(gen.Git, gitGenInfo) ||
				shouldRefreshPRGenerator(gen.PullRequest, prGenInfo) ||
				shouldRefreshMatrixGenerator(gen.Matrix, gitGenInfo, prGenInfo) ||
				shouldRefreshMergeGenerator(gen.Merge, gitGenInfo, prGenInfo)
			if shouldRefresh {
				break
			}
		}
		if shouldRefresh {
			err := refreshApplicationSet(h.client, &appSet)
			if err != nil {
				log.Errorf("Failed to refresh ApplicationSet '%s' for controller reprocessing", appSet.Name)
				continue
			}
			log.Infof("refresh ApplicationSet %v/%v from webhook", appSet.Namespace, appSet.Name)
		}
	}
}

func (h *WebhookHandler) Handler(w http.ResponseWriter, r *http.Request) {
	var payload interface{}
	var err error

	switch {
	case r.Header.Get("X-GitHub-Event") != "":
		payload, err = h.github.Parse(r, github.PushEvent, github.PullRequestEvent)
	case r.Header.Get("X-Gitlab-Event") != "":
		payload, err = h.gitlab.Parse(r, gitlab.PushEvents, gitlab.TagEvents)
	default:
		log.Debug("Ignoring unknown webhook event")
		http.Error(w, "Unknown webhook event", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Infof("Webhook processing failed: %s", err)
		status := http.StatusBadRequest
		if r.Method != "POST" {
			status = http.StatusMethodNotAllowed
		}
		http.Error(w, fmt.Sprintf("Webhook processing failed: %s", html.EscapeString(err.Error())), status)
		return
	}

	h.HandleEvent(payload)
}

func parseRevision(ref string) string {
	refParts := strings.SplitN(ref, "/", 3)
	return refParts[len(refParts)-1]
}

func getGitGeneratorInfo(payload interface{}) *gitGeneratorInfo {
	var (
		webURL      string
		revision    string
		touchedHead bool
	)
	switch payload := payload.(type) {
	case github.PushPayload:
		webURL = payload.Repository.HTMLURL
		revision = parseRevision(payload.Ref)
		touchedHead = payload.Repository.DefaultBranch == revision
	case gitlab.PushEventPayload:
		webURL = payload.Project.WebURL
		revision = parseRevision(payload.Ref)
		touchedHead = payload.Project.DefaultBranch == revision
	default:
		return nil
	}

	log.Infof("Received push event repo: %s, revision: %s, touchedHead: %v", webURL, revision, touchedHead)
	urlObj, err := url.Parse(webURL)
	if err != nil {
		log.Errorf("Failed to parse repoURL '%s'", webURL)
		return nil
	}
	regexpStr := `(?i)(http://|https://|\w+@|ssh://(\w+@)?)` + urlObj.Hostname() + "(:[0-9]+|)[:/]" + urlObj.Path[1:] + "(\\.git)?"
	repoRegexp, err := regexp.Compile(regexpStr)
	if err != nil {
		log.Errorf("Failed to compile regexp for repoURL '%s'", webURL)
		return nil
	}

	return &gitGeneratorInfo{
		RepoRegexp:  repoRegexp,
		TouchedHead: touchedHead,
		Revision:    revision,
	}
}

func getPRGeneratorInfo(payload interface{}) *prGeneratorInfo {
	var info prGeneratorInfo
	switch payload := payload.(type) {
	case github.PullRequestPayload:
		if !isAllowedPullRequestAction(payload.Action) {
			return nil
		}

		apiURL := payload.Repository.URL
		urlObj, err := url.Parse(apiURL)
		if err != nil {
			log.Errorf("Failed to parse repoURL '%s'", apiURL)
			return nil
		}
		regexpStr := `(?i)(http://|https://|\w+@|ssh://(\w+@)?)` + urlObj.Hostname() + "(:[0-9]+|)[:/]"
		apiRegexp, err := regexp.Compile(regexpStr)
		if err != nil {
			log.Errorf("Failed to compile regexp for repoURL '%s'", apiURL)
			return nil
		}
		info.Github = &prGeneratorGithubInfo{
			Repo:      payload.Repository.Name,
			Owner:     payload.Repository.Owner.Login,
			APIRegexp: apiRegexp,
		}
	default:
		return nil
	}

	return &info
}

// allowedPullRequestActions is a list of actions that allow refresh
var allowedPullRequestActions = []string{
	"opened",
	"closed",
	"synchronize",
	"labeled",
	"reopened",
	"unlabeled",
}

func isAllowedPullRequestAction(action string) bool {
	for _, allow := range allowedPullRequestActions {
		if allow == action {
			return true
		}
	}
	return false
}

func shouldRefreshGitGenerator(gen *v1alpha1.GitGenerator, info *gitGeneratorInfo) bool {
	if gen == nil || info == nil {
		return false
	}

	if !gitGeneratorUsesURL(gen, info.Revision, info.RepoRegexp) {
		return false
	}
	if !genRevisionHasChanged(gen, info.Revision, info.TouchedHead) {
		return false
	}
	return true
}

func genRevisionHasChanged(gen *v1alpha1.GitGenerator, revision string, touchedHead bool) bool {
	targetRev := parseRevision(gen.Revision)
	if targetRev == "HEAD" || targetRev == "" { // revision is head
		return touchedHead
	}

	return targetRev == revision
}

func gitGeneratorUsesURL(gen *v1alpha1.GitGenerator, webURL string, repoRegexp *regexp.Regexp) bool {
	if !repoRegexp.MatchString(gen.RepoURL) {
		log.Debugf("%s does not match %s", gen.RepoURL, repoRegexp.String())
		return false
	}

	log.Debugf("%s uses repoURL %s", gen.RepoURL, webURL)
	return true
}

func shouldRefreshPRGenerator(gen *v1alpha1.PullRequestGenerator, info *prGeneratorInfo) bool {
	if gen == nil || info == nil {
		return false
	}

	if gen.Github == nil || info.Github == nil {
		return false
	}
	if gen.Github.Owner != info.Github.Owner {
		return false
	}
	if gen.Github.Repo != info.Github.Repo {
		return false
	}
	api := gen.Github.API
	if api == "" {
		api = "https://api.github.com/"
	}
	if !info.Github.APIRegexp.MatchString(api) {
		log.Debugf("%s does not match %s", gen.Github.API, info.Github.APIRegexp.String())
		return false
	}

	return true
}

func shouldRefreshMatrixGenerator(gen *v1alpha1.MatrixGenerator, gitGenInfo *gitGeneratorInfo, prGenInfo *prGeneratorInfo) bool {
	if gen == nil {
		return false
	}
	return shouldRefreshNestedGenerator(gen.Generators, gitGenInfo, prGenInfo)
}

func shouldRefreshMergeGenerator(gen *v1alpha1.MergeGenerator, gitGenInfo *gitGeneratorInfo, prGenInfo *prGeneratorInfo) bool {
	if gen == nil {
		return false
	}
	return shouldRefreshNestedGenerator(gen.Generators, gitGenInfo, prGenInfo)
}

func shouldRefreshNestedGenerator(gens []v1alpha1.ApplicationSetNestedGenerator, gitGenInfo *gitGeneratorInfo, prGenInfo *prGeneratorInfo) bool {
	shouldRefresh := false
	for _, gen := range gens {
		shouldRefresh = shouldRefreshGitGenerator(gen.Git, gitGenInfo) || shouldRefreshPRGenerator(gen.PullRequest, prGenInfo)
		if shouldRefresh {
			break
		}
	}
	return shouldRefresh
}

func refreshApplicationSet(c client.Client, appSet *v1alpha1.ApplicationSet) error {
	// patch the ApplicationSet with the refresh annotation to reconcile
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		err := c.Get(context.Background(), types.NamespacedName{Name: appSet.Name, Namespace: appSet.Namespace}, appSet)
		if err != nil {
			return err
		}
		if appSet.Annotations == nil {
			appSet.Annotations = map[string]string{}
		}
		appSet.Annotations[common.AnnotationApplicationSetRefresh] = "true"
		return c.Patch(context.Background(), appSet, client.Merge)
	})
}
