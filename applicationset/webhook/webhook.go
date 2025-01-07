package webhook

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v2/applicationset/generators"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argosettings "github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/argoproj/argo-cd/v2/util/webhook"

	"github.com/go-playground/webhooks/v6/azuredevops"
	"github.com/go-playground/webhooks/v6/github"
	"github.com/go-playground/webhooks/v6/gitlab"
	log "github.com/sirupsen/logrus"
)

const payloadQueueSize = 50000

type WebhookHandler struct {
	sync.WaitGroup // for testing
	namespace      string
	github         *github.Webhook
	gitlab         *gitlab.Webhook
	azuredevops    *azuredevops.Webhook
	client         client.Client
	generators     map[string]generators.Generator
	queue          chan interface{}
}

type gitGeneratorInfo struct {
	Revision    string
	TouchedHead bool
	RepoRegexp  *regexp.Regexp
}

type prGeneratorInfo struct {
	Azuredevops *prGeneratorAzuredevopsInfo
	Github      *prGeneratorGithubInfo
	Gitlab      *prGeneratorGitlabInfo
}

type prGeneratorAzuredevopsInfo struct {
	Repo    string
	Project string
}

type prGeneratorGithubInfo struct {
	Repo      string
	Owner     string
	APIRegexp *regexp.Regexp
}

type prGeneratorGitlabInfo struct {
	Project     string
	APIHostname string
}

func NewWebhookHandler(namespace string, webhookParallelism int, argocdSettingsMgr *argosettings.SettingsManager, client client.Client, generators map[string]generators.Generator) (*WebhookHandler, error) {
	// register the webhook secrets stored under "argocd-secret" for verifying incoming payloads
	argocdSettings, err := argocdSettingsMgr.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("Failed to get argocd settings: %w", err)
	}
	githubHandler, err := github.New(github.Options.Secret(argocdSettings.WebhookGitHubSecret))
	if err != nil {
		return nil, fmt.Errorf("Unable to init GitHub webhook: %w", err)
	}
	gitlabHandler, err := gitlab.New(gitlab.Options.Secret(argocdSettings.WebhookGitLabSecret))
	if err != nil {
		return nil, fmt.Errorf("Unable to init GitLab webhook: %w", err)
	}
	azuredevopsHandler, err := azuredevops.New(azuredevops.Options.BasicAuth(argocdSettings.WebhookAzureDevOpsUsername, argocdSettings.WebhookAzureDevOpsPassword))
	if err != nil {
		return nil, fmt.Errorf("Unable to init Azure DevOps webhook: %w", err)
	}

	webhookHandler := &WebhookHandler{
		namespace:   namespace,
		github:      githubHandler,
		gitlab:      gitlabHandler,
		azuredevops: azuredevopsHandler,
		client:      client,
		generators:  generators,
		queue:       make(chan interface{}, payloadQueueSize),
	}

	webhookHandler.startWorkerPool(webhookParallelism)

	return webhookHandler, nil
}

func (h *WebhookHandler) startWorkerPool(webhookParallelism int) {
	for i := 0; i < webhookParallelism; i++ {
		h.Add(1)
		go func() {
			defer h.Done()
			for {
				payload, ok := <-h.queue
				if !ok {
					return
				}
				h.HandleEvent(payload)
			}
		}()
	}
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
				shouldRefreshPluginGenerator(gen.Plugin) ||
				h.shouldRefreshMatrixGenerator(gen.Matrix, &appSet, gitGenInfo, prGenInfo) ||
				h.shouldRefreshMergeGenerator(gen.Merge, &appSet, gitGenInfo, prGenInfo)
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
		payload, err = h.github.Parse(r, github.PushEvent, github.PullRequestEvent, github.PingEvent)
	case r.Header.Get("X-Gitlab-Event") != "":
		payload, err = h.gitlab.Parse(r, gitlab.PushEvents, gitlab.TagEvents, gitlab.MergeRequestEvents)
	case r.Header.Get("X-Vss-Activityid") != "":
		payload, err = h.azuredevops.Parse(r, azuredevops.GitPushEventType, azuredevops.GitPullRequestCreatedEventType, azuredevops.GitPullRequestUpdatedEventType, azuredevops.GitPullRequestMergedEventType)
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

	select {
	case h.queue <- payload:
	default:
		log.Info("Queue is full, discarding webhook payload")
		http.Error(w, "Queue is full, discarding webhook payload", http.StatusServiceUnavailable)
	}
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
		revision = webhook.ParseRevision(payload.Ref)
		touchedHead = payload.Repository.DefaultBranch == revision
	case gitlab.PushEventPayload:
		webURL = payload.Project.WebURL
		revision = webhook.ParseRevision(payload.Ref)
		touchedHead = payload.Project.DefaultBranch == revision
	case azuredevops.GitPushEvent:
		// See: https://learn.microsoft.com/en-us/azure/devops/service-hooks/events?view=azure-devops#git.push
		webURL = payload.Resource.Repository.RemoteURL
		revision = webhook.ParseRevision(payload.Resource.RefUpdates[0].Name)
		touchedHead = payload.Resource.RefUpdates[0].Name == payload.Resource.Repository.DefaultBranch
		// unfortunately, Azure DevOps doesn't provide a list of changed files
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
		if !isAllowedGithubPullRequestAction(payload.Action) {
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
	case gitlab.MergeRequestEventPayload:
		if !isAllowedGitlabPullRequestAction(payload.ObjectAttributes.Action) {
			return nil
		}

		apiURL := payload.Project.WebURL
		urlObj, err := url.Parse(apiURL)
		if err != nil {
			log.Errorf("Failed to parse repoURL '%s'", apiURL)
			return nil
		}

		info.Gitlab = &prGeneratorGitlabInfo{
			Project:     strconv.FormatInt(payload.ObjectAttributes.TargetProjectID, 10),
			APIHostname: urlObj.Hostname(),
		}
	case azuredevops.GitPullRequestEvent:
		if !isAllowedAzureDevOpsPullRequestAction(string(payload.EventType)) {
			return nil
		}

		repo := payload.Resource.Repository.Name
		project := payload.Resource.Repository.Project.Name

		info.Azuredevops = &prGeneratorAzuredevopsInfo{
			Repo:    repo,
			Project: project,
		}
	default:
		return nil
	}

	return &info
}

// githubAllowedPullRequestActions is a list of github actions that allow refresh
var githubAllowedPullRequestActions = []string{
	"opened",
	"closed",
	"synchronize",
	"labeled",
	"reopened",
	"unlabeled",
}

// gitlabAllowedPullRequestActions is a list of gitlab actions that allow refresh
// https://docs.gitlab.com/ee/user/project/integrations/webhook_events.html#merge-request-events
var gitlabAllowedPullRequestActions = []string{
	"open",
	"close",
	"reopen",
	"update",
	"merge",
}

// azuredevopsAllowedPullRequestActions is a list of Azure DevOps actions that allow refresh
var azuredevopsAllowedPullRequestActions = []string{
	"git.pullrequest.created",
	"git.pullrequest.merged",
	"git.pullrequest.updated",
}

func isAllowedGithubPullRequestAction(action string) bool {
	for _, allow := range githubAllowedPullRequestActions {
		if allow == action {
			return true
		}
	}
	return false
}

func isAllowedGitlabPullRequestAction(action string) bool {
	for _, allow := range gitlabAllowedPullRequestActions {
		if allow == action {
			return true
		}
	}
	return false
}

func isAllowedAzureDevOpsPullRequestAction(action string) bool {
	for _, allow := range azuredevopsAllowedPullRequestActions {
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

func shouldRefreshPluginGenerator(gen *v1alpha1.PluginGenerator) bool {
	return gen != nil
}

func genRevisionHasChanged(gen *v1alpha1.GitGenerator, revision string, touchedHead bool) bool {
	targetRev := webhook.ParseRevision(gen.Revision)
	if targetRev == "HEAD" || targetRev == "" { // revision is head
		return touchedHead
	}

	return targetRev == revision || gen.Revision == revision
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

	if gen.GitLab != nil && info.Gitlab != nil {
		if gen.GitLab.Project != info.Gitlab.Project {
			return false
		}

		api := gen.GitLab.API
		if api == "" {
			api = "https://gitlab.com/"
		}

		urlObj, err := url.Parse(api)
		if err != nil {
			log.Errorf("Failed to parse repoURL '%s'", api)
			return false
		}

		if urlObj.Hostname() != info.Gitlab.APIHostname {
			log.Debugf("%s does not match %s", api, info.Gitlab.APIHostname)
			return false
		}

		return true
	}

	if gen.Github != nil && info.Github != nil {
		// repository owner and name are case-insensitive
		// See https://docs.github.com/en/rest/pulls/pulls?apiVersion=2022-11-28#list-pull-requests
		if !strings.EqualFold(gen.Github.Owner, info.Github.Owner) {
			return false
		}
		if !strings.EqualFold(gen.Github.Repo, info.Github.Repo) {
			return false
		}
		api := gen.Github.API
		if api == "" {
			api = "https://api.github.com/"
		}
		if !info.Github.APIRegexp.MatchString(api) {
			log.Debugf("%s does not match %s", api, info.Github.APIRegexp.String())
			return false
		}

		return true
	}

	if gen.AzureDevOps != nil && info.Azuredevops != nil {
		if gen.AzureDevOps.Project != info.Azuredevops.Project {
			return false
		}
		if gen.AzureDevOps.Repo != info.Azuredevops.Repo {
			return false
		}
		return true
	}

	return false
}

func (h *WebhookHandler) shouldRefreshMatrixGenerator(gen *v1alpha1.MatrixGenerator, appSet *v1alpha1.ApplicationSet, gitGenInfo *gitGeneratorInfo, prGenInfo *prGeneratorInfo) bool {
	if gen == nil {
		return false
	}

	// Silently ignore, the ApplicationSetReconciler will log the error as part of the reconcile
	if len(gen.Generators) < 2 || len(gen.Generators) > 2 {
		return false
	}

	g0 := gen.Generators[0]

	// Check first child generator for Git or Pull Request Generator
	if shouldRefreshGitGenerator(g0.Git, gitGenInfo) ||
		shouldRefreshPRGenerator(g0.PullRequest, prGenInfo) {
		return true
	}

	// Check first child generator for nested Matrix generator
	var matrixGenerator0 *v1alpha1.MatrixGenerator
	if g0.Matrix != nil {
		// Since nested matrix generator is represented as a JSON object in the CRD, we unmarshall it back to a Go struct here.
		nestedMatrix, err := v1alpha1.ToNestedMatrixGenerator(g0.Matrix)
		if err != nil {
			log.Errorf("Failed to unmarshall nested matrix generator: %v", err)
			return false
		}
		if nestedMatrix != nil {
			matrixGenerator0 = nestedMatrix.ToMatrixGenerator()
			if h.shouldRefreshMatrixGenerator(matrixGenerator0, appSet, gitGenInfo, prGenInfo) {
				return true
			}
		}
	}

	// Check first child generator for nested Merge generator
	var mergeGenerator0 *v1alpha1.MergeGenerator
	if g0.Merge != nil {
		// Since nested merge generator is represented as a JSON object in the CRD, we unmarshall it back to a Go struct here.
		nestedMerge, err := v1alpha1.ToNestedMergeGenerator(g0.Merge)
		if err != nil {
			log.Errorf("Failed to unmarshall nested merge generator: %v", err)
			return false
		}
		if nestedMerge != nil {
			mergeGenerator0 = nestedMerge.ToMergeGenerator()
			if h.shouldRefreshMergeGenerator(mergeGenerator0, appSet, gitGenInfo, prGenInfo) {
				return true
			}
		}
	}

	// Create ApplicationSetGenerator for first child generator from its ApplicationSetNestedGenerator
	requestedGenerator0 := &v1alpha1.ApplicationSetGenerator{
		List:                    g0.List,
		Clusters:                g0.Clusters,
		Git:                     g0.Git,
		SCMProvider:             g0.SCMProvider,
		ClusterDecisionResource: g0.ClusterDecisionResource,
		PullRequest:             g0.PullRequest,
		Plugin:                  g0.Plugin,
		Matrix:                  matrixGenerator0,
		Merge:                   mergeGenerator0,
	}

	// Generate params for first child generator
	relGenerators := generators.GetRelevantGenerators(requestedGenerator0, h.generators)
	params := []map[string]interface{}{}
	for _, g := range relGenerators {
		p, err := g.GenerateParams(requestedGenerator0, appSet, h.client)
		if err != nil {
			log.Error(err)
			return false
		}
		params = append(params, p...)
	}

	g1 := gen.Generators[1]

	// Create Matrix generator for nested Matrix generator as second child generator
	var matrixGenerator1 *v1alpha1.MatrixGenerator
	if g1.Matrix != nil {
		// Since nested matrix generator is represented as a JSON object in the CRD, we unmarshall it back to a Go struct here.
		nestedMatrix, err := v1alpha1.ToNestedMatrixGenerator(g1.Matrix)
		if err != nil {
			log.Errorf("Failed to unmarshall nested matrix generator: %v", err)
			return false
		}
		if nestedMatrix != nil {
			matrixGenerator1 = nestedMatrix.ToMatrixGenerator()
		}
	}

	// Create Merge generator for nested Merge generator as second child generator
	var mergeGenerator1 *v1alpha1.MergeGenerator
	if g1.Merge != nil {
		// Since nested merge generator is represented as a JSON object in the CRD, we unmarshall it back to a Go struct here.
		nestedMerge, err := v1alpha1.ToNestedMergeGenerator(g1.Merge)
		if err != nil {
			log.Errorf("Failed to unmarshall nested merge generator: %v", err)
			return false
		}
		if nestedMerge != nil {
			mergeGenerator1 = nestedMerge.ToMergeGenerator()
		}
	}

	// Create ApplicationSetGenerator for second child generator from its ApplicationSetNestedGenerator
	requestedGenerator1 := &v1alpha1.ApplicationSetGenerator{
		List:                    g1.List,
		Clusters:                g1.Clusters,
		Git:                     g1.Git,
		SCMProvider:             g1.SCMProvider,
		ClusterDecisionResource: g1.ClusterDecisionResource,
		PullRequest:             g1.PullRequest,
		Plugin:                  g1.Plugin,
		Matrix:                  matrixGenerator1,
		Merge:                   mergeGenerator1,
	}

	// Interpolate second child generator with params from first child generator, if there are any params
	if len(params) != 0 {
		for _, p := range params {
			tempInterpolatedGenerator, err := generators.InterpolateGenerator(requestedGenerator1, p, appSet.Spec.GoTemplate, appSet.Spec.GoTemplateOptions)
			interpolatedGenerator := &tempInterpolatedGenerator
			if err != nil {
				log.Error(err)
				return false
			}

			// Check all interpolated child generators
			if shouldRefreshGitGenerator(interpolatedGenerator.Git, gitGenInfo) ||
				shouldRefreshPRGenerator(interpolatedGenerator.PullRequest, prGenInfo) ||
				shouldRefreshPluginGenerator(interpolatedGenerator.Plugin) ||
				h.shouldRefreshMatrixGenerator(interpolatedGenerator.Matrix, appSet, gitGenInfo, prGenInfo) ||
				h.shouldRefreshMergeGenerator(requestedGenerator1.Merge, appSet, gitGenInfo, prGenInfo) {
				return true
			}
		}
	}

	// First child generator didn't return any params, just check the second child generator
	return shouldRefreshGitGenerator(requestedGenerator1.Git, gitGenInfo) ||
		shouldRefreshPRGenerator(requestedGenerator1.PullRequest, prGenInfo) ||
		shouldRefreshPluginGenerator(requestedGenerator1.Plugin) ||
		h.shouldRefreshMatrixGenerator(requestedGenerator1.Matrix, appSet, gitGenInfo, prGenInfo) ||
		h.shouldRefreshMergeGenerator(requestedGenerator1.Merge, appSet, gitGenInfo, prGenInfo)
}

func (h *WebhookHandler) shouldRefreshMergeGenerator(gen *v1alpha1.MergeGenerator, appSet *v1alpha1.ApplicationSet, gitGenInfo *gitGeneratorInfo, prGenInfo *prGeneratorInfo) bool {
	if gen == nil {
		return false
	}

	for _, g := range gen.Generators {
		// Check Git or Pull Request generator
		if shouldRefreshGitGenerator(g.Git, gitGenInfo) ||
			shouldRefreshPRGenerator(g.PullRequest, prGenInfo) {
			return true
		}

		// Check nested Matrix generator
		if g.Matrix != nil {
			// Since nested matrix generator is represented as a JSON object in the CRD, we unmarshall it back to a Go struct here.
			nestedMatrix, err := v1alpha1.ToNestedMatrixGenerator(g.Matrix)
			if err != nil {
				log.Errorf("Failed to unmarshall nested matrix generator: %v", err)
				return false
			}
			if nestedMatrix != nil {
				if h.shouldRefreshMatrixGenerator(nestedMatrix.ToMatrixGenerator(), appSet, gitGenInfo, prGenInfo) {
					return true
				}
			}
		}

		// Check nested Merge generator
		if g.Merge != nil {
			// Since nested merge generator is represented as a JSON object in the CRD, we unmarshall it back to a Go struct here.
			nestedMerge, err := v1alpha1.ToNestedMergeGenerator(g.Merge)
			if err != nil {
				log.Errorf("Failed to unmarshall nested merge generator: %v", err)
				return false
			}
			if nestedMerge != nil {
				if h.shouldRefreshMergeGenerator(nestedMerge.ToMergeGenerator(), appSet, gitGenInfo, prGenInfo) {
					return true
				}
			}
		}
	}

	return false
}

func refreshApplicationSet(c client.Client, appSet *v1alpha1.ApplicationSet) error {
	// patch the ApplicationSet with the refresh annotation to reconcile
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		err := c.Get(context.Background(), types.NamespacedName{Name: appSet.Name, Namespace: appSet.Namespace}, appSet)
		if err != nil {
			return fmt.Errorf("error getting ApplicationSet: %w", err)
		}
		if appSet.Annotations == nil {
			appSet.Annotations = map[string]string{}
		}
		appSet.Annotations[common.AnnotationApplicationSetRefresh] = "true"
		return c.Patch(context.Background(), appSet, client.Merge)
	})
}
