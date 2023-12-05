package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/go-playground/webhooks/v6/bitbucket"
	bitbucketserver "github.com/go-playground/webhooks/v6/bitbucket-server"
	"github.com/go-playground/webhooks/v6/github"
	"github.com/go-playground/webhooks/v6/gitlab"
	gogsclient "github.com/gogits/go-gogs-client"
	"k8s.io/apimachinery/pkg/runtime"
	kubetesting "k8s.io/client-go/testing"

	"github.com/argoproj/argo-cd/v2/util/cache/appstate"

	"github.com/argoproj/argo-cd/v2/util/db/mocks"

	servercache "github.com/argoproj/argo-cd/v2/server/cache"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/v2/reposerver/cache"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

type fakeSettingsSrc struct {
}

func (f fakeSettingsSrc) GetAppInstanceLabelKey() (string, error) {
	return "mycompany.com/appname", nil
}

func (f fakeSettingsSrc) GetTrackingMethod() (string, error) {
	return "", nil
}

type reactorDef struct {
	verb     string
	resource string
	reaction kubetesting.ReactionFunc
}

func NewMockHandler(reactor *reactorDef, applicationNamespaces []string, objects ...runtime.Object) *ArgoCDWebhookHandler {
	appClientset := appclientset.NewSimpleClientset(objects...)
	if reactor != nil {
		defaultReactor := appClientset.ReactionChain[0]
		appClientset.ReactionChain = nil
		appClientset.AddReactor("list", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return defaultReactor.React(action)
		})
		appClientset.AddReactor(reactor.verb, reactor.resource, reactor.reaction)
	}
	cacheClient := cacheutil.NewCache(cacheutil.NewInMemoryCache(1 * time.Hour))

	return NewHandler("argocd", applicationNamespaces, appClientset, &settings.ArgoCDSettings{}, &fakeSettingsSrc{}, cache.NewCache(
		cacheClient,
		1*time.Minute,
		1*time.Minute,
	), servercache.NewCache(appstate.NewCache(cacheClient, time.Minute), time.Minute, time.Minute, time.Minute), &mocks.ArgoDB{})
}

func TestGitHubCommitEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", nil)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := os.ReadFile("testdata/github-commit-event.json")
	assert.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
	expectedLogResult := "Received push event repo: https://github.com/jessesuen/test-repo, revision: master, touchedHead: true"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestAzureDevOpsCommitEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", nil)
	req.Header.Set("X-Vss-Activityid", "abc")
	eventJSON, err := os.ReadFile("testdata/azuredevops-git-push-event.json")
	assert.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
	expectedLogResult := "Received push event repo: https://dev.azure.com/alexander0053/alex-test/_git/alex-test, revision: master, touchedHead: true"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

// TestGitHubCommitEvent_MultiSource_Refresh makes sure that a webhook will refresh a multi-source app when at least
// one source matches.
func TestGitHubCommitEvent_MultiSource_Refresh(t *testing.T) {
	hook := test.NewGlobal()
	var patched bool
	reaction := func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patchAction := action.(kubetesting.PatchAction)
		assert.Equal(t, "app-to-refresh", patchAction.GetName())
		patched = true
		return true, nil, nil
	}
	h := NewMockHandler(&reactorDef{"patch", "applications", reaction}, []string{}, &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-to-refresh",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSpec{
			Sources: v1alpha1.ApplicationSources{
				{
					RepoURL: "https://github.com/some/unrelated-repo",
					Path:    ".",
				},
				{
					RepoURL: "https://github.com/jessesuen/test-repo",
					Path:    ".",
				},
			},
		},
	}, &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app-to-ignore",
		},
		Spec: v1alpha1.ApplicationSpec{
			Sources: v1alpha1.ApplicationSources{
				{
					RepoURL: "https://github.com/some/unrelated-repo",
					Path:    ".",
				},
			},
		},
	},
	)
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", nil)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := os.ReadFile("testdata/github-commit-event.json")
	assert.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
	expectedLogResult := "Requested app 'app-to-refresh' refresh"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	assert.True(t, patched)
	hook.Reset()
}

// TestGitHubCommitEvent_AppsInOtherNamespaces makes sure that webhooks properly find apps in the configured set of
// allowed namespaces when Apps are allowed in any namespace
func TestGitHubCommitEvent_AppsInOtherNamespaces(t *testing.T) {
	hook := test.NewGlobal()

	patchedApps := make([]types.NamespacedName, 0, 3)
	reaction := func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patchAction := action.(kubetesting.PatchAction)
		patchedApps = append(patchedApps, types.NamespacedName{Name: patchAction.GetName(), Namespace: patchAction.GetNamespace()})
		return true, nil, nil
	}

	h := NewMockHandler(&reactorDef{"patch", "applications", reaction}, []string{"end-to-end-tests", "app-team-*"},
		&v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "app-to-refresh-in-default-namespace",
				Namespace: "argocd",
			},
			Spec: v1alpha1.ApplicationSpec{
				Sources: v1alpha1.ApplicationSources{
					{
						RepoURL: "https://github.com/jessesuen/test-repo",
						Path:    ".",
					},
				},
			},
		}, &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "app-to-ignore",
				Namespace: "kube-system",
			},
			Spec: v1alpha1.ApplicationSpec{
				Sources: v1alpha1.ApplicationSources{
					{
						RepoURL: "https://github.com/jessesuen/test-repo",
						Path:    ".",
					},
				},
			},
		}, &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "app-to-refresh-in-exact-match-namespace",
				Namespace: "end-to-end-tests",
			},
			Spec: v1alpha1.ApplicationSpec{
				Sources: v1alpha1.ApplicationSources{
					{
						RepoURL: "https://github.com/jessesuen/test-repo",
						Path:    ".",
					},
				},
			},
		}, &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "app-to-refresh-in-globbed-namespace",
				Namespace: "app-team-two",
			},
			Spec: v1alpha1.ApplicationSpec{
				Sources: v1alpha1.ApplicationSources{
					{
						RepoURL: "https://github.com/jessesuen/test-repo",
						Path:    ".",
					},
				},
			},
		},
	)
	req := httptest.NewRequest("POST", "/api/webhook", nil)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := os.ReadFile("testdata/github-commit-event.json")
	assert.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)

	logMessages := make([]string, 0, len(hook.Entries))

	for _, entry := range hook.Entries {
		logMessages = append(logMessages, entry.Message)
	}

	assert.Contains(t, logMessages, "Requested app 'app-to-refresh-in-default-namespace' refresh")
	assert.Contains(t, logMessages, "Requested app 'app-to-refresh-in-exact-match-namespace' refresh")
	assert.Contains(t, logMessages, "Requested app 'app-to-refresh-in-globbed-namespace' refresh")
	assert.NotContains(t, logMessages, "Requested app 'app-to-ignore' refresh")

	assert.Contains(t, patchedApps, types.NamespacedName{Name: "app-to-refresh-in-default-namespace", Namespace: "argocd"})
	assert.Contains(t, patchedApps, types.NamespacedName{Name: "app-to-refresh-in-exact-match-namespace", Namespace: "end-to-end-tests"})
	assert.Contains(t, patchedApps, types.NamespacedName{Name: "app-to-refresh-in-globbed-namespace", Namespace: "app-team-two"})
	assert.NotContains(t, patchedApps, types.NamespacedName{Name: "app-to-ignore", Namespace: "kube-system"})
	assert.Len(t, patchedApps, 3)

	hook.Reset()
}

func TestGitHubTagEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", nil)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := os.ReadFile("testdata/github-tag-event.json")
	assert.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
	expectedLogResult := "Received push event repo: https://github.com/jessesuen/test-repo, revision: v1.0, touchedHead: false"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestGitHubPingEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", nil)
	req.Header.Set("X-GitHub-Event", "ping")
	eventJSON, err := os.ReadFile("testdata/github-ping-event.json")
	assert.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
	expectedLogResult := "Ignoring webhook event"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestBitbucketServerRepositoryReferenceChangedEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", nil)
	req.Header.Set("X-Event-Key", "repo:refs_changed")
	eventJSON, err := os.ReadFile("testdata/bitbucket-server-event.json")
	assert.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
	expectedLogResultSsh := "Received push event repo: ssh://git@bitbucketserver:7999/myproject/test-repo.git, revision: master, touchedHead: true"
	assert.Equal(t, expectedLogResultSsh, hook.AllEntries()[len(hook.AllEntries())-2].Message)
	expectedLogResultHttps := "Received push event repo: https://bitbucketserver/scm/myproject/test-repo.git, revision: master, touchedHead: true"
	assert.Equal(t, expectedLogResultHttps, hook.LastEntry().Message)
	hook.Reset()
}

func TestBitbucketServerRepositoryDiagnosticPingEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	eventJSON := "{\"test\": true}"
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", bytes.NewBufferString(eventJSON))
	req.Header.Set("X-Event-Key", "diagnostics:ping")
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
	expectedLogResult := "Ignoring webhook event"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestGogsPushEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", nil)
	req.Header.Set("X-Gogs-Event", "push")
	eventJSON, err := os.ReadFile("testdata/gogs-event.json")
	assert.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
	expectedLogResult := "Received push event repo: http://gogs-server/john/repo-test, revision: master, touchedHead: true"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestGitLabPushEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", nil)
	req.Header.Set("X-Gitlab-Event", "Push Hook")
	eventJSON, err := os.ReadFile("testdata/gitlab-event.json")
	assert.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
	expectedLogResult := "Received push event repo: https://gitlab/group/name, revision: master, touchedHead: true"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestGitLabSystemEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", nil)
	req.Header.Set("X-Gitlab-Event", "System Hook")
	eventJSON, err := os.ReadFile("testdata/gitlab-event.json")
	assert.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
	expectedLogResult := "Received push event repo: https://gitlab/group/name, revision: master, touchedHead: true"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestInvalidMethod(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodGet, "/api/webhook", nil)
	req.Header.Set("X-GitHub-Event", "push")
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusMethodNotAllowed)
	expectedLogResult := "Webhook processing failed: invalid HTTP Method"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	assert.Equal(t, expectedLogResult+"\n", w.Body.String())
	hook.Reset()
}

func TestInvalidEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", nil)
	req.Header.Set("X-GitHub-Event", "push")
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusBadRequest)
	expectedLogResult := "Webhook processing failed: error parsing payload"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	assert.Equal(t, expectedLogResult+"\n", w.Body.String())
	hook.Reset()
}

func TestUnknownEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", nil)
	req.Header.Set("X-Unknown-Event", "push")
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusBadRequest)
	assert.Equal(t, "Unknown webhook event\n", w.Body.String())
	hook.Reset()
}

func getApp(annotation string, sourcePath string) *v1alpha1.Application {
	return &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				v1alpha1.AnnotationKeyManifestGeneratePaths: annotation,
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			Source: &v1alpha1.ApplicationSource{
				Path: sourcePath,
			},
		},
	}
}

func getMultiSourceApp(annotation string, paths ...string) *v1alpha1.Application {
	var sources v1alpha1.ApplicationSources
	for _, path := range paths {
		sources = append(sources, v1alpha1.ApplicationSource{Path: path})
	}
	return &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				v1alpha1.AnnotationKeyManifestGeneratePaths: annotation,
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			Sources: sources,
		},
	}
}

func Test_getAppRefreshPrefix(t *testing.T) {
	tests := []struct {
		name           string
		app            *v1alpha1.Application
		files          []string
		changeExpected bool
	}{
		{"default no path", &v1alpha1.Application{}, []string{"README.md"}, true},
		{"relative path - matching", getApp(".", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"relative path, multi source - matching #1", getMultiSourceApp(".", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"relative path, multi source - matching #2", getMultiSourceApp(".", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"relative path - not matching", getApp(".", "source/path"), []string{"README.md"}, false},
		{"relative path, multi source - not matching", getMultiSourceApp(".", "other/path", "unrelated/path"), []string{"README.md"}, false},
		{"absolute path - matching", getApp("/source/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"absolute path, multi source - matching #1", getMultiSourceApp("/source/path", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"absolute path, multi source - matching #2", getMultiSourceApp("/source/path", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"absolute path - not matching", getApp("/source/path1", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"absolute path, multi source - not matching", getMultiSourceApp("/source/path1", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"two relative paths - matching", getApp(".;../shared", "my-app"), []string{"shared/my-deployment.yaml"}, true},
		{"two relative paths, multi source - matching #1", getMultiSourceApp(".;../shared", "my-app", "other/path"), []string{"shared/my-deployment.yaml"}, true},
		{"two relative paths, multi source - matching #2", getMultiSourceApp(".;../shared", "my-app", "other/path"), []string{"shared/my-deployment.yaml"}, true},
		{"two relative paths - not matching", getApp(".;../shared", "my-app"), []string{"README.md"}, false},
		{"two relative paths, multi source - not matching", getMultiSourceApp(".;../shared", "my-app", "other/path"), []string{"README.md"}, false},
		{"file relative path - matching", getApp("./my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file relative path, multi source - matching #1", getMultiSourceApp("./my-deployment.yaml", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file relative path, multi source - matching #2", getMultiSourceApp("./my-deployment.yaml", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file relative path - not matching", getApp("./my-deployment.yaml", "source/path"), []string{"README.md"}, false},
		{"file relative path, multi source - not matching", getMultiSourceApp("./my-deployment.yaml", "source/path", "other/path"), []string{"README.md"}, false},
		{"file absolute path - matching", getApp("/source/path/my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file absolute path, multi source - matching #1", getMultiSourceApp("/source/path/my-deployment.yaml", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file absolute path, multi source - matching #2", getMultiSourceApp("/source/path/my-deployment.yaml", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file absolute path - not matching", getApp("/source/path1/README.md", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"file absolute path, multi source - not matching", getMultiSourceApp("/source/path1/README.md", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"file two relative paths - matching", getApp("./README.md;../shared/my-deployment.yaml", "my-app"), []string{"shared/my-deployment.yaml"}, true},
		{"file two relative paths, multi source - matching", getMultiSourceApp("./README.md;../shared/my-deployment.yaml", "my-app", "other-path"), []string{"shared/my-deployment.yaml"}, true},
		{"file two relative paths - not matching", getApp(".README.md;../shared/my-deployment.yaml", "my-app"), []string{"kustomization.yaml"}, false},
		{"file two relative paths, multi source - not matching", getMultiSourceApp(".README.md;../shared/my-deployment.yaml", "my-app", "other-path"), []string{"kustomization.yaml"}, false},
	}
	for _, tt := range tests {
		ttc := tt
		t.Run(ttc.name, func(t *testing.T) {
			t.Parallel()
			if got := appFilesHaveChanged(ttc.app, ttc.files); got != ttc.changeExpected {
				t.Errorf("getAppRefreshPrefix() = %v, want %v", got, ttc.changeExpected)
			}
		})
	}
}

func TestAppRevisionHasChanged(t *testing.T) {
	getSource := func(targetRevision string) v1alpha1.ApplicationSource {
		return v1alpha1.ApplicationSource{TargetRevision: targetRevision}
	}

	testCases := []struct {
		name             string
		source           v1alpha1.ApplicationSource
		revision         string
		touchedHead      bool
		expectHasChanged bool
	}{
		{"no target revision, master, touched head", getSource(""), "master", true, true},
		{"no target revision, master, did not touch head", getSource(""), "master", false, false},
		{"dev target revision, master, touched head", getSource("dev"), "master", true, false},
		{"dev target revision, dev, did not touch head", getSource("dev"), "dev", false, true},
		{"refs/heads/dev target revision, master, touched head", getSource("refs/heads/dev"), "master", true, false},
		{"refs/heads/dev target revision, dev, did not touch head", getSource("refs/heads/dev"), "dev", false, true},
		{"env/test target revision, env/test, did not touch head", getSource("env/test"), "env/test", false, true},
		{"refs/heads/env/test target revision, env/test, did not touch head", getSource("refs/heads/env/test"), "env/test", false, true},
	}

	for _, tc := range testCases {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			t.Parallel()
			changed := sourceRevisionHasChanged(tcc.source, tcc.revision, tcc.touchedHead)
			assert.Equal(t, tcc.expectHasChanged, changed)
		})
	}
}

func Test_affectedRevisionInfo_appRevisionHasChanged(t *testing.T) {
	sourceWithRevision := func(targetRevision string) v1alpha1.ApplicationSource {
		return v1alpha1.ApplicationSource{TargetRevision: targetRevision}
	}

	githubPushPayload := func(branchName string) github.PushPayload {
		// This payload's "ref" member always has the full git ref, according to the field description.
		// https://docs.github.com/en/developers/webhooks-and-events/webhooks/webhook-events-and-payloads#push
		return github.PushPayload{Ref: "refs/heads/" + branchName}
	}

	gitlabPushPayload := func(branchName string) gitlab.PushEventPayload {
		// This payload's "ref" member seems to always have the full git ref (based on the example payload).
		// https://docs.gitlab.com/ee/user/project/integrations/webhook_events.html#push-events
		return gitlab.PushEventPayload{Ref: "refs/heads/" + branchName}
	}

	gitlabTagPayload := func(tagName string) gitlab.TagEventPayload {
		// This payload's "ref" member seems to always have the full git ref (based on the example payload).
		// https://docs.gitlab.com/ee/user/project/integrations/webhook_events.html#tag-events
		return gitlab.TagEventPayload{Ref: "refs/tags/" + tagName}
	}

	bitbucketPushPayload := func(branchName string) bitbucket.RepoPushPayload {
		// The payload's "push.changes[0].new.name" member seems to only have the branch name (based on the example payload).
		// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#EventPayloads-Push
		var pl bitbucket.RepoPushPayload
		_ = json.Unmarshal([]byte(fmt.Sprintf(`{"push":{"changes":[{"new":{"name":"%s"}}]}}`, branchName)), &pl)
		return pl
	}

	bitbucketRefChangedPayload := func(branchName string) bitbucketserver.RepositoryReferenceChangedPayload {
		// This payload's "changes[0].ref.id" member seems to always have the full git ref (based on the example payload).
		// https://confluence.atlassian.com/bitbucketserver/event-payload-938025882.html#Eventpayload-Push
		return bitbucketserver.RepositoryReferenceChangedPayload{
			Changes: []bitbucketserver.RepositoryChange{
				{Reference: bitbucketserver.RepositoryReference{ID: "refs/heads/" + branchName}},
			},
			Repository: bitbucketserver.Repository{Links: map[string]interface{}{"clone": []interface{}{}}},
		}
	}

	gogsPushPayload := func(branchName string) gogsclient.PushPayload {
		// This payload's "ref" member seems to always have the full git ref (based on the example payload).
		// https://gogs.io/docs/features/webhook#event-information
		return gogsclient.PushPayload{Ref: "refs/heads/" + branchName, Repo: &gogsclient.Repository{}}
	}

	tests := []struct {
		hasChanged     bool
		targetRevision string
		hookPayload    interface{}
		name           string
	}{
		// Edge cases for bitbucket.
		// Bitbucket push events just have tag or branch names instead of fully-qualified refs. If someone were to create
		// a branch starting with refs/heads/ or refs/tags/, they couldn't use the branch name in targetRevision.
		{false, "refs/heads/x", bitbucketPushPayload("refs/heads/x"), "bitbucket branch name containing 'refs/heads/'"},
		{false, "refs/tags/x", bitbucketPushPayload("refs/tags/x"), "bitbucket branch name containing 'refs/tags/'"},
		{false, "x", bitbucketPushPayload("refs/heads/x"), "bitbucket branch name containing 'refs/heads/', targetRevision with just the part after refs/heads/"},
		{false, "x", bitbucketPushPayload("refs/tags/x"), "bitbucket branch name containing 'refs/tags/', targetRevision with just the part after refs/tags/"},
		// However, a targetRevision prefixed with refs/heads/ or refs/tags/ would match a payload with just the suffix.
		{true, "refs/heads/x", bitbucketPushPayload("x"), "bitbucket branch name containing 'refs/heads/', targetRevision with just the part after refs/heads/"},
		{true, "refs/tags/x", bitbucketPushPayload("x"), "bitbucket branch name containing 'refs/tags/', targetRevision with just the part after refs/tags/"},
		// They could also hack around the issue by prepending another refs/heads/
		{true, "refs/heads/refs/heads/x", bitbucketPushPayload("refs/heads/x"), "bitbucket branch name containing 'refs/heads/'"},
		{true, "refs/heads/refs/tags/x", bitbucketPushPayload("refs/tags/x"), "bitbucket branch name containing 'refs/tags/'"},

		// Standard cases. These tests show that
		//  1) Slashes in branch names do not cause missed refreshes.
		//  2) Fully-qualifying branches/tags by adding the refs/(heads|tags)/ prefix does not cause missed refreshes.
		//  3) Branches and tags are not differentiated. A branch event with branch name 'x' will match all the following:
		//      a. targetRevision: x
		//      b. targetRevision: refs/heads/x
		//      c. targetRevision: refs/tags/x
		//     A tag event with tag name 'x' will match all of those as well.

		{true, "has/slashes", githubPushPayload("has/slashes"), "github push branch name with slashes, targetRevision not prefixed"},
		{true, "has/slashes", gitlabPushPayload("has/slashes"), "gitlab push branch name with slashes, targetRevision not prefixed"},
		{true, "has/slashes", bitbucketPushPayload("has/slashes"), "bitbucket push branch name with slashes, targetRevision not prefixed"},
		{true, "has/slashes", bitbucketRefChangedPayload("has/slashes"), "bitbucket ref changed branch name with slashes, targetRevision not prefixed"},
		{true, "has/slashes", gogsPushPayload("has/slashes"), "gogs push branch name with slashes, targetRevision not prefixed"},

		{true, "refs/heads/has/slashes", githubPushPayload("has/slashes"), "github push branch name with slashes, targetRevision branch prefixed"},
		{true, "refs/heads/has/slashes", gitlabPushPayload("has/slashes"), "gitlab push branch name with slashes, targetRevision branch prefixed"},
		{true, "refs/heads/has/slashes", bitbucketPushPayload("has/slashes"), "bitbucket push branch name with slashes, targetRevision branch prefixed"},
		{true, "refs/heads/has/slashes", bitbucketRefChangedPayload("has/slashes"), "bitbucket ref changed branch name with slashes, targetRevision branch prefixed"},
		{true, "refs/heads/has/slashes", gogsPushPayload("has/slashes"), "gogs push branch name with slashes, targetRevision branch prefixed"},

		// Not testing for refs/tags/has/slashes, because apparently tags can't have slashes: https://stackoverflow.com/a/32850142/684776

		{true, "no-slashes", githubPushPayload("no-slashes"), "github push branch or tag name without slashes, targetRevision not prefixed"},
		{true, "no-slashes", gitlabTagPayload("no-slashes"), "gitlab tag branch or tag name without slashes, targetRevision not prefixed"},
		{true, "no-slashes", gitlabPushPayload("no-slashes"), "gitlab push branch or tag name without slashes, targetRevision not prefixed"},
		{true, "no-slashes", bitbucketPushPayload("no-slashes"), "bitbucket push branch or tag name without slashes, targetRevision not prefixed"},
		{true, "no-slashes", bitbucketRefChangedPayload("no-slashes"), "bitbucket ref changed branch or tag name without slashes, targetRevision not prefixed"},
		{true, "no-slashes", gogsPushPayload("no-slashes"), "gogs push branch or tag name without slashes, targetRevision not prefixed"},

		{true, "refs/heads/no-slashes", githubPushPayload("no-slashes"), "github push branch or tag name without slashes, targetRevision branch prefixed"},
		{true, "refs/heads/no-slashes", gitlabTagPayload("no-slashes"), "gitlab tag branch or tag name without slashes, targetRevision branch prefixed"},
		{true, "refs/heads/no-slashes", gitlabPushPayload("no-slashes"), "gitlab push branch or tag name without slashes, targetRevision branch prefixed"},
		{true, "refs/heads/no-slashes", bitbucketPushPayload("no-slashes"), "bitbucket push branch or tag name without slashes, targetRevision branch prefixed"},
		{true, "refs/heads/no-slashes", bitbucketRefChangedPayload("no-slashes"), "bitbucket ref changed branch or tag name without slashes, targetRevision branch prefixed"},
		{true, "refs/heads/no-slashes", gogsPushPayload("no-slashes"), "gogs push branch or tag name without slashes, targetRevision branch prefixed"},

		{true, "refs/tags/no-slashes", githubPushPayload("no-slashes"), "github push branch or tag name without slashes, targetRevision tag prefixed"},
		{true, "refs/tags/no-slashes", gitlabTagPayload("no-slashes"), "gitlab tag branch or tag name without slashes, targetRevision tag prefixed"},
		{true, "refs/tags/no-slashes", gitlabPushPayload("no-slashes"), "gitlab push branch or tag name without slashes, targetRevision tag prefixed"},
		{true, "refs/tags/no-slashes", bitbucketPushPayload("no-slashes"), "bitbucket push branch or tag name without slashes, targetRevision tag prefixed"},
		{true, "refs/tags/no-slashes", bitbucketRefChangedPayload("no-slashes"), "bitbucket ref changed branch or tag name without slashes, targetRevision tag prefixed"},
		{true, "refs/tags/no-slashes", gogsPushPayload("no-slashes"), "gogs push branch or tag name without slashes, targetRevision tag prefixed"},
	}
	for _, testCase := range tests {
		testCopy := testCase
		t.Run(testCopy.name, func(t *testing.T) {
			t.Parallel()
			_, revisionFromHook, _, _, _ := affectedRevisionInfo(testCopy.hookPayload)
			if got := sourceRevisionHasChanged(sourceWithRevision(testCopy.targetRevision), revisionFromHook, false); got != testCopy.hasChanged {
				t.Errorf("sourceRevisionHasChanged() = %v, want %v", got, testCopy.hasChanged)
			}
		})
	}
}

func Test_getWebUrlRegex(t *testing.T) {
	tests := []struct {
		shouldMatch bool
		webURL      string
		repo        string
		name        string
	}{
		// Ensure input is regex-escaped.
		{false, "https://example.com/org/a..d", "https://example.com/org/abcd", "dots in repo names should not be treated as wildcards"},
		{false, "https://an.example.com/org/repo", "https://an-example.com/org/repo", "dots in domain names should not be treated as wildcards"},

		// Standard cases.
		{true, "https://example.com/org/repo", "https://example.com/org/repo", "exact match should match"},
		{false, "https://example.com/org/repo", "https://example.com/org/repo-2", "partial match should not match"},
		{true, "https://example.com/org/repo", "https://example.com/org/repo.git", "no .git should match with .git"},
		{true, "https://example.com/org/repo", "git@example.com:org/repo", "git without protocol should match"},
		{true, "https://example.com/org/repo", "user@example.com:org/repo", "git with non-git username shout match"},
		{true, "https://example.com/org/repo", "ssh://git@example.com/org/repo", "git with protocol should match"},
		{true, "https://example.com/org/repo", "ssh://git@example.com:22/org/repo", "git with port number should should match"},
		{true, "https://example.com:443/org/repo", "ssh://git@example.com:22/org/repo", "https and ssh w/ different port numbers should match"},
		{true, "https://example.com/org/repo", "ssh://user-name@example.com/org/repo", "valid usernames with hyphens in repo should match"},
		{false, "https://example.com/org/repo", "ssh://-user-name@example.com/org/repo", "invalid usernames with hyphens in repo should not match"},
		{true, "https://example.com:443/org/repo", "GIT@EXAMPLE.COM:22:ORG/REPO", "matches aren't case-sensitive"},
	}
	for _, testCase := range tests {
		testCopy := testCase
		t.Run(testCopy.name, func(t *testing.T) {
			t.Parallel()
			regexp, err := getWebUrlRegex(testCopy.webURL)
			assert.NoError(t, err)
			if matches := regexp.MatchString(testCopy.repo); matches != testCopy.shouldMatch {
				t.Errorf("sourceRevisionHasChanged() = %v, want %v", matches, testCopy.shouldMatch)
			}
		})
	}
}
