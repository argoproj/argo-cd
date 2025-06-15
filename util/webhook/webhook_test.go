package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"text/template"
	"time"

	bb "github.com/ktrysmt/go-bitbucket"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/types"

	"github.com/go-playground/webhooks/v6/bitbucket"
	bitbucketserver "github.com/go-playground/webhooks/v6/bitbucket-server"
	"github.com/go-playground/webhooks/v6/github"
	"github.com/go-playground/webhooks/v6/gitlab"
	gogsclient "github.com/gogits/go-gogs-client"
	"github.com/jarcoal/httpmock"
	"k8s.io/apimachinery/pkg/runtime"
	kubetesting "k8s.io/client-go/testing"

	servercache "github.com/argoproj/argo-cd/v3/server/cache"
	"github.com/argoproj/argo-cd/v3/util/cache/appstate"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/db/mocks"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/v3/reposerver/cache"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

type fakeSettingsSrc struct{}

func (f fakeSettingsSrc) GetAppInstanceLabelKey() (string, error) {
	return "mycompany.com/appname", nil
}

func (f fakeSettingsSrc) GetTrackingMethod() (string, error) {
	return "", nil
}

func (f fakeSettingsSrc) GetInstallationID() (string, error) {
	return "", nil
}

type reactorDef struct {
	verb     string
	resource string
	reaction kubetesting.ReactionFunc
}

func NewMockHandler(reactor *reactorDef, applicationNamespaces []string, objects ...runtime.Object) *ArgoCDWebhookHandler {
	defaultMaxPayloadSize := int64(50) * 1024 * 1024
	return NewMockHandlerWithPayloadLimit(reactor, applicationNamespaces, defaultMaxPayloadSize, objects...)
}

func NewMockHandlerWithPayloadLimit(reactor *reactorDef, applicationNamespaces []string, maxPayloadSize int64, objects ...runtime.Object) *ArgoCDWebhookHandler {
	return newMockHandler(reactor, applicationNamespaces, maxPayloadSize, &mocks.ArgoDB{}, &settings.ArgoCDSettings{}, objects...)
}

func NewMockHandlerForBitbucketCallback(reactor *reactorDef, applicationNamespaces []string, objects ...runtime.Object) *ArgoCDWebhookHandler {
	mockDB := mocks.ArgoDB{}
	mockDB.On("ListRepositories", mock.Anything).Return(
		[]*v1alpha1.Repository{
			{
				Repo:     "https://bitbucket.org/test/argocd-examples-pub.git",
				Username: "test",
				Password: "test",
			},
			{
				Repo:     "https://bitbucket.org/test-owner/test-repo.git",
				Username: "test",
				Password: "test",
			},
			{
				Repo:          "git@bitbucket.org:test/argocd-examples-pub.git",
				SSHPrivateKey: "test-ssh-key",
			},
		}, nil)
	argoSettings := settings.ArgoCDSettings{WebhookBitbucketUUID: "abcd-efgh-ijkl-mnop"}
	defaultMaxPayloadSize := int64(50) * 1024 * 1024
	return newMockHandler(reactor, applicationNamespaces, defaultMaxPayloadSize, &mockDB, &argoSettings, objects...)
}

func newMockHandler(reactor *reactorDef, applicationNamespaces []string, maxPayloadSize int64, argoDB db.ArgoDB, argoSettings *settings.ArgoCDSettings, objects ...runtime.Object) *ArgoCDWebhookHandler {
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

	return NewHandler("argocd", applicationNamespaces, 10, appClientset, argoSettings, &fakeSettingsSrc{}, cache.NewCache(
		cacheClient,
		1*time.Minute,
		1*time.Minute,
		10*time.Second,
	), servercache.NewCache(appstate.NewCache(cacheClient, time.Minute), time.Minute, time.Minute), argoDB, maxPayloadSize)
}

func TestGitHubCommitEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := os.ReadFile("testdata/github-commit-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()
	assert.Equal(t, http.StatusOK, w.Code)
	expectedLogResult := "Received push event repo: https://github.com/jessesuen/test-repo, revision: master, touchedHead: true"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestAzureDevOpsCommitEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-Vss-Activityid", "abc")
	eventJSON, err := os.ReadFile("testdata/azuredevops-git-push-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()
	assert.Equal(t, http.StatusOK, w.Code)
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
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := os.ReadFile("testdata/github-commit-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()
	assert.Equal(t, http.StatusOK, w.Code)
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
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := os.ReadFile("testdata/github-commit-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()
	assert.Equal(t, http.StatusOK, w.Code)

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

// TestGitHubCommitEvent_Hydrate makes sure that a webhook will hydrate an app when dry source changed.
func TestGitHubCommitEvent_Hydrate(t *testing.T) {
	hook := test.NewGlobal()
	var patched bool
	reaction := func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patchAction := action.(kubetesting.PatchAction)
		assert.Equal(t, "app-to-hydrate", patchAction.GetName())
		patched = true
		return true, nil, nil
	}
	h := NewMockHandler(&reactorDef{"patch", "applications", reaction}, []string{}, &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-to-hydrate",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSpec{
			SourceHydrator: &v1alpha1.SourceHydrator{
				DrySource: v1alpha1.DrySource{
					RepoURL:        "https://github.com/jessesuen/test-repo",
					TargetRevision: "HEAD",
					Path:           ".",
				},
				SyncSource: v1alpha1.SyncSource{
					TargetBranch: "environments/dev",
					Path:         ".",
				},
				HydrateTo: nil,
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
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := os.ReadFile("testdata/github-commit-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, patched)

	logMessages := make([]string, 0, len(hook.Entries))
	for _, entry := range hook.Entries {
		logMessages = append(logMessages, entry.Message)
	}

	assert.Contains(t, logMessages, "webhook trigger refresh app to hydrate 'app-to-hydrate'")
	assert.NotContains(t, logMessages, "webhook trigger refresh app to hydrate 'app-to-ignore'")

	hook.Reset()
}

func TestGitHubTagEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := os.ReadFile("testdata/github-tag-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()
	assert.Equal(t, http.StatusOK, w.Code)
	expectedLogResult := "Received push event repo: https://github.com/jessesuen/test-repo, revision: v1.0, touchedHead: false"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestGitHubPingEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "ping")
	eventJSON, err := os.ReadFile("testdata/github-ping-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()
	assert.Equal(t, http.StatusOK, w.Code)
	expectedLogResult := "Ignoring webhook event"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestBitbucketServerRepositoryReferenceChangedEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-Event-Key", "repo:refs_changed")
	eventJSON, err := os.ReadFile("testdata/bitbucket-server-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()
	assert.Equal(t, http.StatusOK, w.Code)
	expectedLogResultSSH := "Received push event repo: ssh://git@bitbucketserver:7999/myproject/test-repo.git, revision: master, touchedHead: true"
	assert.Equal(t, expectedLogResultSSH, hook.AllEntries()[len(hook.AllEntries())-2].Message)
	expectedLogResultHTTPS := "Received push event repo: https://bitbucketserver/scm/myproject/test-repo.git, revision: master, touchedHead: true"
	assert.Equal(t, expectedLogResultHTTPS, hook.LastEntry().Message)
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
	close(h.queue)
	h.Wait()
	assert.Equal(t, http.StatusOK, w.Code)
	expectedLogResult := "Ignoring webhook event"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestGogsPushEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-Gogs-Event", "push")
	eventJSON, err := os.ReadFile("testdata/gogs-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()
	assert.Equal(t, http.StatusOK, w.Code)
	expectedLogResult := "Received push event repo: http://gogs-server/john/repo-test, revision: master, touchedHead: true"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestGitLabPushEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-Gitlab-Event", "Push Hook")
	eventJSON, err := os.ReadFile("testdata/gitlab-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()
	assert.Equal(t, http.StatusOK, w.Code)
	expectedLogResult := "Received push event repo: https://gitlab.com/group/name, revision: master, touchedHead: true"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestGitLabSystemEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-Gitlab-Event", "System Hook")
	eventJSON, err := os.ReadFile("testdata/gitlab-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()
	assert.Equal(t, http.StatusOK, w.Code)
	expectedLogResult := "Received push event repo: https://gitlab.com/group/name, revision: master, touchedHead: true"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestInvalidMethod(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodGet, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "push")
	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	expectedLogResult := "Webhook processing failed: invalid HTTP Method"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	assert.Equal(t, expectedLogResult+"\n", w.Body.String())
	hook.Reset()
}

func TestInvalidEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "push")
	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()
	assert.Equal(t, http.StatusBadRequest, w.Code)
	expectedLogResult := "Webhook processing failed: The payload is either too large or corrupted. Please check the payload size (must be under 50 MB) and ensure it is valid JSON"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	assert.Equal(t, expectedLogResult+"\n", w.Body.String())
	hook.Reset()
}

func TestUnknownEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-Unknown-Event", "push")
	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "Unknown webhook event\n", w.Body.String())
	hook.Reset()
}

func TestAppRevisionHasChanged(t *testing.T) {
	t.Parallel()

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
		{"refs/tags/dev target revision, dev, did not touch head", getSource("refs/tags/dev"), "dev", false, true},
		{"env/test target revision, env/test, did not touch head", getSource("env/test"), "env/test", false, true},
		{"refs/heads/env/test target revision, env/test, did not touch head", getSource("refs/heads/env/test"), "env/test", false, true},
		{"refs/tags/env/test target revision, env/test, did not touch head", getSource("refs/tags/env/test"), "env/test", false, true},
		{"three/part/rev target revision, rev, did not touch head", getSource("three/part/rev"), "rev", false, false},
		{"1.* target revision (matching), 1.1.0, did not touch head", getSource("1.*"), "1.1.0", false, true},
		{"refs/tags/1.* target revision (matching), 1.1.0, did not touch head", getSource("refs/tags/1.*"), "1.1.0", false, true},
		{"1.* target revision (not matching), 2.0.0, did not touch head", getSource("1.*"), "2.0.0", false, false},
		{"1.* target revision, dev (not semver), did not touch head", getSource("1.*"), "dev", false, false},
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
	t.Parallel()

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
			Repository: bitbucketserver.Repository{Links: map[string]any{"clone": []any{}}},
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
		hookPayload    any
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
			h := NewMockHandler(nil, []string{})
			_, revisionFromHook, _, _, _ := h.affectedRevisionInfo(testCopy.hookPayload)
			if got := sourceRevisionHasChanged(sourceWithRevision(testCopy.targetRevision), revisionFromHook, false); got != testCopy.hasChanged {
				t.Errorf("sourceRevisionHasChanged() = %v, want %v", got, testCopy.hasChanged)
			}
		})
	}
}

func Test_GetWebURLRegex(t *testing.T) {
	t.Parallel()

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
		{true, "https://example.com/org/repo", "user@example.com:org/repo", "git with non-git username should match"},
		{true, "https://example.com/org/repo", "ssh://git@example.com/org/repo", "git with protocol should match"},
		{true, "https://example.com/org/repo", "ssh://git@example.com:22/org/repo", "git with port number should match"},
		{true, "https://example.com:443/org/repo", "ssh://git@example.com:22/org/repo", "https and ssh w/ different port numbers should match"},
		{true, "https://example.com:443/org/repo", "ssh://git@ssh.example.com:443/org/repo", "https and ssh w/ ssh subdomain should match"},
		{true, "https://example.com:443/org/repo", "ssh://git@altssh.example.com:443/org/repo", "https and ssh w/ altssh subdomain should match"},
		{false, "https://example.com:443/org/repo", "ssh://git@unknown.example.com:443/org/repo", "https and ssh w/ unknown subdomain should not match"},
		{true, "https://example.com/org/repo", "ssh://user-name@example.com/org/repo", "valid usernames with hyphens in repo should match"},
		{false, "https://example.com/org/repo", "ssh://-user-name@example.com/org/repo", "invalid usernames with hyphens in repo should not match"},
		{true, "https://example.com:443/org/repo", "GIT@EXAMPLE.COM:22:ORG/REPO", "matches aren't case-sensitive"},
		{true, "https://example.com/org/repo%20", "https://example.com/org/repo%20", "escape codes in path are preserved"},
		{true, "https://user@example.com/org/repo", "http://example.com/org/repo", "https+username should match http"},
		{true, "https://user@example.com/org/repo", "https://example.com/org/repo", "https+username should match https"},
		{true, "http://example.com/org/repo", "https://user@example.com/org/repo", "http should match https+username"},
		{true, "https://example.com/org/repo", "https://user@example.com/org/repo", "https should match https+username"},
		{true, "https://user@example.com/org/repo", "ssh://example.com/org/repo", "https+username should match ssh"},

		{false, "", "", "empty URLs should not panic"},
	}

	for _, testCase := range tests {
		testCopy := testCase
		t.Run(testCopy.name, func(t *testing.T) {
			t.Parallel()
			regexp, err := GetWebURLRegex(testCopy.webURL)
			require.NoError(t, err)
			if matches := regexp.MatchString(testCopy.repo); matches != testCopy.shouldMatch {
				t.Errorf("sourceRevisionHasChanged() = %v, want %v", matches, testCopy.shouldMatch)
			}
		})
	}

	t.Run("bad URL should error", func(t *testing.T) {
		_, err := GetWebURLRegex("%%")
		require.Error(t, err)
	})
}

func Test_GetAPIURLRegex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		shouldMatch bool
		apiURL      string
		repo        string
		name        string
	}{
		// Ensure input is regex-escaped.
		{false, "https://an.example.com/", "https://an-example.com/", "dots in domain names should not be treated as wildcards"},

		// Standard cases.
		{true, "https://example.com/", "https://example.com/", "exact match should match"},
		{false, "https://example.com/", "ssh://example.com/", "should not match ssh"},
		{true, "https://user@example.com/", "http://example.com/", "https+username should match http"},
		{true, "https://user@example.com/", "https://example.com/", "https+username should match https"},
		{true, "http://example.com/", "https://user@example.com/", "http should match https+username"},
		{true, "https://example.com/", "https://user@example.com/", "https should match https+username"},
	}

	for _, testCase := range tests {
		testCopy := testCase
		t.Run(testCopy.name, func(t *testing.T) {
			t.Parallel()
			regexp, err := GetAPIURLRegex(testCopy.apiURL)
			require.NoError(t, err)
			if matches := regexp.MatchString(testCopy.repo); matches != testCopy.shouldMatch {
				t.Errorf("sourceRevisionHasChanged() = %v, want %v", matches, testCopy.shouldMatch)
			}
		})
	}

	t.Run("bad URL should error", func(t *testing.T) {
		_, err := GetAPIURLRegex("%%")
		require.Error(t, err)
	})
}

func TestGitHubCommitEventMaxPayloadSize(t *testing.T) {
	hook := test.NewGlobal()
	maxPayloadSize := int64(100)
	h := NewMockHandlerWithPayloadLimit(nil, []string{}, maxPayloadSize)
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := os.ReadFile("testdata/github-commit-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()
	assert.Equal(t, http.StatusBadRequest, w.Code)
	expectedLogResult := "Webhook processing failed: The payload is either too large or corrupted. Please check the payload size (must be under 0 MB) and ensure it is valid JSON"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func Test_affectedRevisionInfo_bitbucket_changed_files(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder("GET",
		"https://api.bitbucket.org/2.0/repositories/test-owner/test-repo/diffstat/abcdef..ghijkl",
		getDiffstatResponderFn())
	httpmock.RegisterResponder("GET",
		"https://api.bitbucket.org/2.0/repositories/test-owner/test-repo",
		getRepositoryResponderFn())
	const payloadTemplateString = `
{
  "push":{
    "changes":[
      {"new":{"name":"{{.branch}}", "target": {"hash": "{{.newHash}}"}}, "old": {"name":"{{.branch}}", "target": {"hash": "{{.oldHash}}"}}}
    ]
  },
  "repository":{
    "type": "repository", 
    "full_name": "{{.owner}}/{{.repo}}",
    "name": "{{.repo}}", 
    "scm": "git", 
    "links": {
      "self": {"href": "https://api.bitbucket.org/2.0/repositories/{{.owner}}/{{.repo}}"},
      "html": {"href": "https://bitbucket.org/{{.owner}}/{{.repo}}"}
    }
  }
}`
	tmpl, err := template.New("test").Parse(payloadTemplateString)
	if err != nil {
		panic(err)
	}

	bitbucketPushPayload := func(branchName, owner, repo string) bitbucket.RepoPushPayload {
		// The payload's "push.changes[0].new.name" member seems to only have the branch name (based on the example payload).
		// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#EventPayloads-Push
		var pl bitbucket.RepoPushPayload
		var doc bytes.Buffer
		err = tmpl.Execute(&doc, map[string]string{
			"branch":  branchName,
			"owner":   owner,
			"repo":    repo,
			"oldHash": "abcdef",
			"newHash": "ghijkl",
		})
		if err != nil {
			require.NoError(t, err)
		}
		_ = json.Unmarshal(doc.Bytes(), &pl)
		return pl
	}

	tests := []struct {
		name                 string
		hasChanged           bool
		revision             string
		hookPayload          bitbucket.RepoPushPayload
		expectedTouchHead    bool
		expectedChangedFiles []string
		expectedChangeInfo   changeInfo
	}{
		{
			"bitbucket branch name containing 'refs/heads/'",
			false,
			"release-0.0",
			bitbucketPushPayload("release-0.0", "test-owner", "test-repo"),
			false,
			[]string{"guestbook/guestbook-ui-deployment.yaml"},
			changeInfo{
				shaBefore: "abcdef",
				shaAfter:  "ghijkl",
			},
		},
		{
			"bitbucket branch name containing 'main'",
			false,
			"main",
			bitbucketPushPayload("main", "test-owner", "test-repo"),
			true,
			[]string{"guestbook/guestbook-ui-deployment.yaml"},
			changeInfo{
				shaBefore: "abcdef",
				shaAfter:  "ghijkl",
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			h := NewMockHandlerForBitbucketCallback(nil, []string{})
			_, revisionFromHook, change, touchHead, changedFiles := h.affectedRevisionInfo(testCase.hookPayload)
			require.Equal(t, testCase.revision, revisionFromHook)
			require.Equal(t, testCase.expectedTouchHead, touchHead)
			require.Equal(t, testCase.expectedChangedFiles, changedFiles)
			require.Equal(t, testCase.expectedChangeInfo, change)
		})
	}
}

func TestLookupRepository(t *testing.T) {
	mockCtx, cancel := context.WithDeadline(t.Context(), time.Now().Add(10*time.Second))
	defer cancel()
	h := NewMockHandlerForBitbucketCallback(nil, []string{})
	data := []string{
		"https://bitbucket.org/test/argocd-examples-pub.git",
		"https://bitbucket.org/test/argocd-examples-pub",
		"https://BITBUCKET.org/test/argocd-examples-pub",
		"https://BITBUCKET.org/test/argocd-examples-pub.git",
		"\thttps://bitbucket.org/test/argocd-examples-pub\n",
		"\thttps://bitbucket.org/test/argocd-examples-pub.git\n",
		"git@BITBUCKET.org:test/argocd-examples-pub",
		"git@BITBUCKET.org:test/argocd-examples-pub.git",
		"git@bitbucket.org:test/argocd-examples-pub",
		"git@bitbucket.org:test/argocd-examples-pub.git",
	}
	for _, url := range data {
		repository, err := h.lookupRepository(mockCtx, url)
		require.NoError(t, err)
		require.NotNil(t, repository)
		require.Contains(t, strings.ToLower(repository.Repo), strings.Trim(strings.ToLower(url), "\t\n"))
		require.True(t, repository.Username == "test" || repository.SSHPrivateKey == "test-ssh-key")
	}
	// when no matching repository is found, then it should return nil error and nil repository
	repository, err := h.lookupRepository(t.Context(), "https://bitbucket.org/test/argocd-examples-not-found.git")
	require.NoError(t, err)
	require.Nil(t, repository)
}

func TestCreateBitbucketClient(t *testing.T) {
	tests := []struct {
		name         string
		apiURL       string
		repository   *v1alpha1.Repository
		expectedAuth string
		expectedErr  error
	}{
		{
			"client creation with username and password",
			"https://api.bitbucket.org/2.0/",
			&v1alpha1.Repository{
				Repo:     "https://bitbucket.org/test",
				Username: "test",
				Password: "test",
			},
			"user:\"test\", password:\"test\"",
			nil,
		},
		{
			"client creation for user x-token-auth and token in password",
			"https://api.bitbucket.org/2.0/",
			&v1alpha1.Repository{
				Repo:     "https://bitbucket.org/test",
				Username: "x-token-auth",
				Password: "test-token",
			},
			"bearerToken:\"test-token\"",
			nil,
		},
		{
			"client creation with oauth bearer token",
			"https://api.bitbucket.org/2.0/",
			&v1alpha1.Repository{
				Repo:        "https://bitbucket.org/test",
				BearerToken: "test-token",
			},
			"bearerToken:\"test-token\"",
			nil,
		},
		{
			"client creation with no auth",
			"https://api.bitbucket.org/2.0/",
			&v1alpha1.Repository{
				Repo: "https://bitbucket.org/test",
			},
			"bearerToken:\"\"",
			nil,
		},
		{
			"client creation with invalid api URL",
			"api.bitbucket.org%%/2.0/",
			&v1alpha1.Repository{},
			"",
			errors.New("failed to parse bitbucket api base URL 'api.bitbucket.org%%/2.0/'"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := newBitbucketClient(t.Context(), tt.repository, tt.apiURL)
			if tt.expectedErr == nil {
				require.NoError(t, err)
				require.NotNil(t, client)
				require.Equal(t, tt.apiURL, client.GetApiBaseURL())
				require.Contains(t, fmt.Sprintf("%#v", *client.Auth), tt.expectedAuth)
			} else {
				require.Error(t, err)
				require.Nil(t, client)
				require.Equal(t, tt.expectedErr, err)
			}
		})
	}
}

func TestFetchDiffStatBitbucketClient(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder("GET",
		"https://api.bitbucket.org/2.0/repositories/test-owner/test-repo/diffstat/abcdef..ghijkl",
		getDiffstatResponderFn())
	client := bb.NewOAuthbearerToken("")
	tt := []struct {
		name                string
		owner               string
		repo                string
		spec                string
		expectedLen         int
		expectedFileChanged string
		expectedErrString   string
	}{
		{
			name:                "valid repo and spec",
			owner:               "test-owner",
			repo:                "test-repo",
			spec:                "abcdef..ghijkl",
			expectedLen:         1,
			expectedFileChanged: "guestbook/guestbook-ui-deployment.yaml",
		},
		{
			name:              "invalid spec",
			owner:             "test-owner",
			repo:              "test-repo",
			spec:              "abcdef..",
			expectedErrString: "error getting the diffstat",
		},
	}

	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			changedFiles, err := fetchDiffStatFromBitbucket(t.Context(), client, test.owner, test.repo, test.spec)
			if test.expectedErrString == "" {
				require.NoError(t, err)
				require.NotNil(t, changedFiles)
				require.Len(t, changedFiles, test.expectedLen)
				require.Equal(t, test.expectedFileChanged, changedFiles[0])
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.expectedErrString)
			}
		})
	}
}

func TestIsHeadTouched(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder("GET",
		"https://api.bitbucket.org/2.0/repositories/test-owner/test-repo",
		getRepositoryResponderFn())
	client := bb.NewOAuthbearerToken("")
	tt := []struct {
		name              string
		owner             string
		repo              string
		revision          string
		expectedErrString string
		expectedTouchHead bool
	}{
		{
			name:              "valid repo with main branch in revision",
			owner:             "test-owner",
			repo:              "test-repo",
			revision:          "main",
			expectedErrString: "",
			expectedTouchHead: true,
		},
		{
			name:              "valid repo with main branch in revision",
			owner:             "test-owner",
			repo:              "test-repo",
			revision:          "release-0.0",
			expectedErrString: "",
			expectedTouchHead: false,
		},
		{
			name:              "valid repo with main branch in revision",
			owner:             "test-owner",
			repo:              "unknown-repo",
			revision:          "master",
			expectedErrString: "Get \"https://api.bitbucket.org/2.0/repositories/test-owner/unknown-repo\"",
			expectedTouchHead: false,
		},
	}
	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			touchedHead, err := isHeadTouched(t.Context(), client, test.owner, test.repo, test.revision)
			if test.expectedErrString == "" {
				require.NoError(t, err)
				require.Equal(t, test.expectedTouchHead, touchedHead)
			} else {
				require.Error(t, err)
				require.False(t, touchedHead)
			}
		})
	}
}

// getRepositoryResponderFn return a httpmock responder function to mock a get repository api call to bitbucket server
func getRepositoryResponderFn() func(req *http.Request) (*http.Response, error) {
	return func(_ *http.Request) (*http.Response, error) {
		// sample response: https://api.bitbucket.org/2.0/repositories/anandjoseph/argocd-examples-pub
		repository := &bb.Repository{
			Type:        "repository",
			Full_name:   "test-owner/test-repo",
			Name:        "test-repo",
			Is_private:  false,
			Fork_policy: "allow_forks",
			Mainbranch: bb.RepositoryBranch{
				Name: "main",
				Type: "branch",
			},
		}
		resp, err := httpmock.NewJsonResponse(200, repository)
		if err != nil {
			return httpmock.NewStringResponse(500, ""), nil
		}
		return resp, nil
	}
}

// getDiffstatResponderFn return a httpmock responder function to mock a diffstat api call to bitbucket server
func getDiffstatResponderFn() func(req *http.Request) (*http.Response, error) {
	return func(_ *http.Request) (*http.Response, error) {
		// sample response : https://api.bitbucket.org/2.0/repositories/anandjoseph/argocd-examples-pub/diffstat/3a53cee247fc820fbae0a9cf463a6f4a18369f90..3d0965f36fcc07e88130b2d5c917a37c2876c484
		diffStatRes := &bb.DiffStatRes{
			Page:    1,
			Size:    1,
			Pagelen: 500,
			DiffStats: []*bb.DiffStat{
				{
					Type:         "diffstat",
					Status:       "added",
					LinedAdded:   20,
					LinesRemoved: 0,
					New: map[string]any{
						"path":         "guestbook/guestbook-ui-deployment.yaml",
						"type":         "commit_file",
						"escaped_path": "guestbook/guestbook-ui-deployment.yaml",
						"links": map[string]any{
							"self": map[string]any{
								"href": "https://bitbucket.org/guestbook/guestbook-ui-deployment.yaml",
							},
						},
					},
				},
			},
		}
		resp, err := httpmock.NewJsonResponse(200, diffStatRes)
		if err != nil {
			return httpmock.NewStringResponse(500, ""), nil
		}
		return resp, nil
	}
}
