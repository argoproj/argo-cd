package webhook

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"text/template"
	"time"

	"github.com/go-playground/webhooks/v6/azuredevops"

	bb "github.com/ktrysmt/go-bitbucket"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"github.com/go-playground/webhooks/v6/bitbucket"
	bitbucketserver "github.com/go-playground/webhooks/v6/bitbucket-server"
	"github.com/go-playground/webhooks/v6/github"
	"github.com/go-playground/webhooks/v6/gitlab"
	gogsclient "github.com/gogits/go-gogs-client"
	"github.com/jarcoal/httpmock"
	"k8s.io/apimachinery/pkg/runtime"
	kubetesting "k8s.io/client-go/testing"

	argov1 "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	servercache "github.com/argoproj/argo-cd/v3/server/cache"
	"github.com/argoproj/argo-cd/v3/util/cache/appstate"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/db/mocks"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
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

func assertLogContains(t *testing.T, hook *test.Hook, msg string) {
	t.Helper()
	for _, entry := range hook.Entries {
		if entry.Message == msg {
			return
		}
	}
	t.Errorf("log hook did not contain message: %q", msg)
}

func assertLogContainsSubstr(t *testing.T, hook *test.Hook, substr string) {
	t.Helper()
	for _, entry := range hook.Entries {
		if strings.Contains(entry.Message, substr) {
			return
		}
	}
	t.Errorf("log hook did not contain message with substring: %q", substr)
}

func NewMockHandler(reactor *reactorDef, applicationNamespaces []string, objects ...runtime.Object) *ArgoCDWebhookHandler {
	defaultMaxPayloadSize := int64(50) * 1024 * 1024
	return NewMockHandlerWithPayloadLimit(reactor, applicationNamespaces, defaultMaxPayloadSize, objects...)
}

func NewMockHandlerWithPayloadLimit(reactor *reactorDef, applicationNamespaces []string, maxPayloadSize int64, objects ...runtime.Object) *ArgoCDWebhookHandler {
	mockDB := &mocks.ArgoDB{}
	mockDB.EXPECT().ListRepositories(mock.Anything).Return([]*v1alpha1.Repository{}, nil).Maybe()
	return newMockHandler(reactor, applicationNamespaces, maxPayloadSize, mockDB, &settings.ArgoCDSettings{}, objects...)
}

func NewMockHandlerWithDockerHubSecret(secret string, applicationNamespaces []string, objects ...runtime.Object) *ArgoCDWebhookHandler {
	mockDB := &mocks.ArgoDB{}
	mockDB.EXPECT().ListRepositories(mock.Anything).Return([]*v1alpha1.Repository{}, nil).Maybe()
	argoSettings := settings.ArgoCDSettings{WebhookDockerHubSecret: secret}
	defaultMaxPayloadSize := int64(50) * 1024 * 1024
	return newMockHandler(nil, applicationNamespaces, defaultMaxPayloadSize, mockDB, &argoSettings, objects...)
}

func NewMockHandlerForBitbucketCallback(reactor *reactorDef, applicationNamespaces []string, objects ...runtime.Object) *ArgoCDWebhookHandler {
	mockDB := &mocks.ArgoDB{}
	mockDB.EXPECT().ListRepositories(mock.Anything).Return(
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
	return newMockHandler(reactor, applicationNamespaces, defaultMaxPayloadSize, mockDB, &argoSettings, objects...)
}

type fakeProjectNamespaceLister struct {
	argov1.AppProjectNamespaceLister
	namespace string
	clientset *appclientset.Clientset
}

func (f *fakeProjectNamespaceLister) Get(name string) (*v1alpha1.AppProject, error) {
	return f.clientset.ArgoprojV1alpha1().AppProjects(f.namespace).Get(context.Background(), name, metav1.GetOptions{})
}

type fakeAppsLister struct {
	argov1.ApplicationLister
	argov1.ApplicationNamespaceLister
	namespace string
	clientset *appclientset.Clientset
}

func (f *fakeAppsLister) Applications(namespace string) argov1.ApplicationNamespaceLister {
	return &fakeAppsLister{namespace: namespace, clientset: f.clientset}
}

func (f *fakeAppsLister) List(selector labels.Selector) ([]*v1alpha1.Application, error) {
	res, err := f.clientset.ArgoprojV1alpha1().Applications(f.namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}
	var apps []*v1alpha1.Application
	for i := range res.Items {
		apps = append(apps, &res.Items[i])
	}
	return apps, nil
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
	return NewHandler("argocd", applicationNamespaces, 10, 10, appClientset, &fakeAppsLister{clientset: appClientset}, argoSettings, &fakeSettingsSrc{}, cache.NewCache(
		cacheClient,
		1*time.Minute,
		1*time.Minute,
		10*time.Second,
	), servercache.NewCache(appstate.NewCache(cacheClient, time.Minute), time.Minute, time.Minute), argoDB, maxPayloadSize, 0, 10, &fakeProjectNamespaceLister{clientset: appClientset, namespace: "argocd"})
}

func TestGitHubCommitEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := os.ReadFile("testdata/github-commit-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	h.Shutdown()
	assert.Equal(t, http.StatusOK, w.Code)
	expectedLogResult := "Received push event repo: https://github.com/jessesuen/test-repo, revision: master, touchedHead: true"
	assertLogContains(t, hook, expectedLogResult)
	hook.Reset()
}

func TestAzureDevOpsCommitEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-Vss-Activityid", "abc")
	eventJSON, err := os.ReadFile("testdata/azuredevops-git-push-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	h.Shutdown()
	assert.Equal(t, http.StatusOK, w.Code)
	expectedLogResult := "Received push event repo: https://dev.azure.com/my-org/my-project/_git/my-repo, revision: master, touchedHead: true"
	assertLogContains(t, hook, expectedLogResult)
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
			Name:      "app-to-refresh-in-default-namespace",
			Namespace: "argocd",
			Spec: v1alpha1.ApplicationSpec{
				Sources: v1alpha1.ApplicationSources{
					{
						RepoURL: "https://github.com/jessesuen/test-repo",
						Path:    ".",
					},
				},
			},
		}, &v1alpha1.Application{
			Name:      "app-to-ignore",
			Namespace: "kube-system",
			Spec: v1alpha1.ApplicationSpec{
				Sources: v1alpha1.ApplicationSources{
					{
						RepoURL: "https://github.com/jessesuen/test-repo",
						Path:    ".",
					},
				},
			},
		}, &v1alpha1.Application{
			Name:      "app-to-refresh-in-exact-match-namespace",
			Namespace: "end-to-end-tests",
			Spec: v1alpha1.ApplicationSpec{
				Sources: v1alpha1.ApplicationSources{
					{
						RepoURL: "https://github.com/jessesuen/test-repo",
						Path:    ".",
					},
				},
			},
		}, &v1alpha1.Application{
			Name:      "app-to-refresh-in-globbed-namespace",
			Namespace: "app-team-two",
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := os.ReadFile("testdata/github-commit-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	time.Sleep(50 * time.Millisecond) // Give workers time to process the queued items
	h.Shutdown()
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

func TestGitHubTagEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := os.ReadFile("testdata/github-tag-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	h.Shutdown()
	assert.Equal(t, http.StatusOK, w.Code)
	expectedLogResult := "Received push event repo: https://github.com/jessesuen/test-repo, revision: v1.0, touchedHead: false"
	assertLogContains(t, hook, expectedLogResult)
	hook.Reset()
}

func TestGitHubPingEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "ping")
	eventJSON, err := os.ReadFile("testdata/github-ping-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	h.Shutdown()
	assert.Equal(t, http.StatusOK, w.Code)
	expectedLogResult := "Ignoring webhook event"
	assertLogContains(t, hook, expectedLogResult)
	hook.Reset()
}

func TestBitbucketServerRepositoryReferenceChangedEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-Event-Key", "repo:refs_changed")
	eventJSON, err := os.ReadFile("testdata/bitbucket-server-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	h.Shutdown()
	assert.Equal(t, http.StatusOK, w.Code)
	expectedLogResultSSH := "Received push event repo: ssh://git@bitbucketserver:7999/myproject/test-repo.git, revision: master, touchedHead: true"
	assertLogContains(t, hook, expectedLogResultSSH)
	expectedLogResultHTTPS := "Received push event repo: https://bitbucketserver/scm/myproject/test-repo.git, revision: master, touchedHead: true"
	assertLogContains(t, hook, expectedLogResultHTTPS)
	hook.Reset()
}

func TestBitbucketServerRepositoryDiagnosticPingEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	eventJSON := "{\"test\": true}"
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", bytes.NewBufferString(eventJSON))
	req.Header.Set("X-Event-Key", "diagnostics:ping")
	w := httptest.NewRecorder()
	h.Handler(w, req)
	h.Shutdown()
	assert.Equal(t, http.StatusOK, w.Code)
	expectedLogResult := "Ignoring webhook event"
	assertLogContains(t, hook, expectedLogResult)
	hook.Reset()
}

func TestGogsPushEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-Gogs-Event", "push")
	eventJSON, err := os.ReadFile("testdata/gogs-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	h.Shutdown()
	assert.Equal(t, http.StatusOK, w.Code)
	expectedLogResult := "Received push event repo: http://gogs-server/john/repo-test, revision: master, touchedHead: true"
	assertLogContains(t, hook, expectedLogResult)
	hook.Reset()
}

func TestGitLabPushEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-Gitlab-Event", "Push Hook")
	eventJSON, err := os.ReadFile("testdata/gitlab-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	h.Shutdown()
	assert.Equal(t, http.StatusOK, w.Code)
	expectedLogResult := "Received push event repo: https://gitlab.com/group/name, revision: master, touchedHead: true"
	assertLogContains(t, hook, expectedLogResult)
	hook.Reset()
}

func TestGitLabSystemEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-Gitlab-Event", "System Hook")
	eventJSON, err := os.ReadFile("testdata/gitlab-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	h.Shutdown()
	assert.Equal(t, http.StatusOK, w.Code)
	expectedLogResult := "Received push event repo: https://gitlab.com/group/name, revision: master, touchedHead: true"
	assertLogContains(t, hook, expectedLogResult)
	hook.Reset()
}

func TestInvalidMethod(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "push")
	w := httptest.NewRecorder()
	h.Handler(w, req)
	h.Shutdown()
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assertLogContains(t, hook, "Webhook processing failed: invalid HTTP Method")
	assert.Equal(t, "Webhook processing failed\n", w.Body.String())
	hook.Reset()
}

func TestInvalidEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "push")
	w := httptest.NewRecorder()
	h.Handler(w, req)
	h.Shutdown()
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assertLogContainsSubstr(t, hook, "Webhook processing failed: payload too large or corrupted (limit 50 MB)")
	assert.Equal(t, "Webhook processing failed: payload must be valid JSON under 50 MB\n", w.Body.String())
	hook.Reset()
}

func TestUnknownEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-Unknown-Event", "push")
	w := httptest.NewRecorder()
	h.Handler(w, req)
	h.Shutdown()
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "Unknown webhook event\n", w.Body.String())
	hook.Reset()
}

func TestAppRevisionHasChanged(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		targetRevision   string
		revision         string
		touchedHead      bool
		expectHasChanged bool
	}{
		{"no target revision, master, touched head", "", "master", true, true},
		{"no target revision, master, did not touch head", "", "master", false, false},
		{"dev target revision, master, touched head", "dev", "master", true, false},
		{"dev target revision, dev, did not touch head", "dev", "dev", false, true},
		{"refs/heads/dev target revision, master, touched head", "refs/heads/dev", "master", true, false},
		{"refs/heads/dev target revision, dev, did not touch head", "refs/heads/dev", "dev", false, true},
		{"refs/tags/dev target revision, dev, did not touch head", "refs/tags/dev", "dev", false, true},
		{"env/test target revision, env/test, did not touch head", "env/test", "env/test", false, true},
		{"refs/heads/env/test target revision, env/test, did not touch head", "refs/heads/env/test", "env/test", false, true},
		{"refs/tags/env/test target revision, env/test, did not touch head", "refs/tags/env/test", "env/test", false, true},
		{"three/part/rev target revision, rev, did not touch head", "three/part/rev", "rev", false, false},
		{"1.* target revision (matching), 1.1.0, did not touch head", "1.*", "1.1.0", false, true},
		{"refs/tags/1.* target revision (matching), 1.1.0, did not touch head", "refs/tags/1.*", "1.1.0", false, true},
		{"1.* target revision (not matching), 2.0.0, did not touch head", "1.*", "2.0.0", false, false},
		{"1.* target revision, dev (not semver), did not touch head", "1.*", "dev", false, false},
	}

	for _, tc := range testCases {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			t.Parallel()
			changed := RevisionHasChanged(tcc.targetRevision, tcc.revision, tcc.touchedHead)
			assert.Equal(t, tcc.expectHasChanged, changed)
		})
	}
}

func Test_affectedRevisionInfo_appRevisionHasChanged(t *testing.T) {
	t.Parallel()

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
		err := json.Unmarshal(fmt.Appendf(nil, `{"push":{"changes":[{"new":{"name":%q}}]}}`, branchName), &pl)
		require.NoError(t, err)
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

		// Tests fix for https://github.com/argoproj/argo-cd/security/advisories/GHSA-wp4p-9pxh-cgx2
		{true, "test", gogsclient.PushPayload{Ref: "test", Repo: nil}, "gogs push branch with nil repo in payload"},

		// Testing fix for https://github.com/argoproj/argo-cd/security/advisories/GHSA-gpx4-37g2-c8pv
		{false, "test", azuredevops.GitPushEvent{Resource: azuredevops.Resource{RefUpdates: []azuredevops.RefUpdate{}}}, "Azure DevOps malformed push event with no ref updates"},

		{true, "some-ref", bitbucketserver.RepositoryReferenceChangedPayload{
			Changes: []bitbucketserver.RepositoryChange{
				{Reference: bitbucketserver.RepositoryReference{ID: "refs/heads/some-ref"}},
			},
			Repository: bitbucketserver.Repository{Links: map[string]any{"clone": "boom"}}, // The string "boom" here is what previously caused a panic.
		}, "bitbucket push branch or tag name, malformed link"}, // https://github.com/argoproj/argo-cd/security/advisories/GHSA-f9gq-prrc-hrhc

		{true, "some-ref", bitbucketserver.RepositoryReferenceChangedPayload{
			Changes: []bitbucketserver.RepositoryChange{
				{Reference: bitbucketserver.RepositoryReference{ID: "refs/heads/some-ref"}},
			},
			Repository: bitbucketserver.Repository{Links: map[string]any{"clone": []any{map[string]any{"name": "http", "href": []string{}}}}}, // The href as an empty array is what previously caused a panic.
		}, "bitbucket push branch or tag name, malformed href"},
	}
	for _, testCase := range tests {
		testCopy := testCase
		t.Run(testCopy.name, func(t *testing.T) {
			t.Parallel()
			h := NewMockHandler(nil, []string{})
			_, revisionFromHook, _, _, _ := h.affectedRevisionInfo(t.Context(), testCopy.hookPayload)
			if got := RevisionHasChanged(testCopy.targetRevision, revisionFromHook, false); got != testCopy.hasChanged {
				t.Errorf("RevisionHasChanged() = %v, want %v", got, testCopy.hasChanged)
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
				t.Errorf("GetWebURLRegex() = %v, want %v", matches, testCopy.shouldMatch)
			}
		})
	}

	t.Run("bad URL should error", func(t *testing.T) {
		t.Parallel()
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
				t.Errorf("GetAPIURLRegex() = %v, want %v", matches, testCopy.shouldMatch)
			}
		})
	}

	t.Run("bad URL should error", func(t *testing.T) {
		t.Parallel()
		_, err := GetAPIURLRegex("%%")
		require.Error(t, err)
	})
}

func TestGitHubCommitEventMaxPayloadSize(t *testing.T) {
	hook := test.NewGlobal()
	maxPayloadSize := int64(100)
	h := NewMockHandlerWithPayloadLimit(nil, []string{}, maxPayloadSize)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := os.ReadFile("testdata/github-commit-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	h.Shutdown()
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assertLogContainsSubstr(t, hook, "Webhook processing failed: payload too large or corrupted (limit 0 MB)")
	hook.Reset()
}

func TestHandleEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		app         *v1alpha1.Application
		changedFile string // file that was changed in the webhook payload
		hasRefresh  bool   // application has refresh annotation applied
		hasHydrate  bool   // application has hydrate annotation applied
		updateCache bool   // cache should be updated with the new revision
	}{
		{
			name: "single source without annotation - always refreshes",
			app: &v1alpha1.Application{
				Name:      "test-app",
				Namespace: "argocd",
				Spec: v1alpha1.ApplicationSpec{
					Sources: v1alpha1.ApplicationSources{
						{
							RepoURL:        "https://github.com/jessesuen/test-repo",
							Path:           "source/path",
							TargetRevision: "HEAD",
						},
					},
				},
			},
			changedFile: "source/path/app.yaml",
			hasRefresh:  true,
			hasHydrate:  false,
			updateCache: false,
		},
		{
			name: "single source with annotation - matching file triggers refresh",
			app: &v1alpha1.Application{
				Name:      "test-app",
				Namespace: "argocd",
				Annotations: map[string]string{
					"argocd.argoproj.io/manifest-generate-paths": "deploy",
				},
				Spec: v1alpha1.ApplicationSpec{
					Sources: v1alpha1.ApplicationSources{
						{
							RepoURL:        "https://github.com/jessesuen/test-repo",
							Path:           "source/path",
							TargetRevision: "HEAD",
						},
					},
				},
			},
			changedFile: "source/path/deploy/app.yaml",
			hasRefresh:  true,
			hasHydrate:  false,
			updateCache: false,
		},
		{
			name: "single source with annotation - non-matching file updates cache",
			app: &v1alpha1.Application{
				Name:      "test-app",
				Namespace: "argocd",
				Annotations: map[string]string{
					"argocd.argoproj.io/manifest-generate-paths": "manifests",
				},
				Spec: v1alpha1.ApplicationSpec{
					Sources: v1alpha1.ApplicationSources{
						{
							RepoURL:        "https://github.com/jessesuen/test-repo",
							Path:           "source/path",
							TargetRevision: "HEAD",
						},
					},
				},
			},
			changedFile: "source/path/other/app.yaml",
			hasRefresh:  false,
			hasHydrate:  false,
			updateCache: true,
		},
		{
			name: "single source with multiple paths annotation - matching subpath triggers refresh",
			app: &v1alpha1.Application{
				Name:      "test-app",
				Namespace: "argocd",
				Annotations: map[string]string{
					"argocd.argoproj.io/manifest-generate-paths": "manifests;dev/deploy;other/path",
				},
				Spec: v1alpha1.ApplicationSpec{
					Sources: v1alpha1.ApplicationSources{
						{
							RepoURL:        "https://github.com/jessesuen/test-repo",
							Path:           "source/path",
							TargetRevision: "HEAD",
						},
					},
				},
			},
			changedFile: "source/path/dev/deploy/app.yaml",
			hasRefresh:  true,
			hasHydrate:  false,
			updateCache: false,
		},
		{
			name: "multi-source without annotation - always refreshes",
			app: &v1alpha1.Application{
				Name:      "test-app",
				Namespace: "argocd",
				Spec: v1alpha1.ApplicationSpec{
					Sources: v1alpha1.ApplicationSources{
						{
							RepoURL:        "https://github.com/jessesuen/test-repo",
							Path:           "helm-charts",
							TargetRevision: "HEAD",
						},
						{
							RepoURL:        "https://github.com/jessesuen/test-repo",
							Path:           "ksapps",
							TargetRevision: "HEAD",
						},
					},
				},
			},
			changedFile: "ksapps/app.yaml",
			hasRefresh:  true,
			hasHydrate:  false,
			updateCache: false,
		},
		{
			name: "multi-source with annotation - matching file triggers refresh",
			app: &v1alpha1.Application{
				Name:      "test-app",
				Namespace: "argocd",
				Annotations: map[string]string{
					"argocd.argoproj.io/manifest-generate-paths": "components",
				},
				Spec: v1alpha1.ApplicationSpec{
					Sources: v1alpha1.ApplicationSources{
						{
							RepoURL:        "https://github.com/jessesuen/test-repo",
							Path:           "helm-charts",
							TargetRevision: "HEAD",
						},
						{
							RepoURL:        "https://github.com/jessesuen/test-repo",
							Path:           "ksapps",
							TargetRevision: "HEAD",
						},
					},
				},
			},
			changedFile: "ksapps/components/app.yaml",
			hasRefresh:  true,
			hasHydrate:  false,
			updateCache: false,
		},
		{
			name: "source hydrator sync source without annotation - refreshes when sync path matches",
			app: &v1alpha1.Application{
				Name:      "test-app",
				Namespace: "argocd",
				Spec: v1alpha1.ApplicationSpec{
					SourceHydrator: &v1alpha1.SourceHydrator{
						DrySource: v1alpha1.DrySource{
							RepoURL:        "https://github.com/jessesuen/test-repo",
							TargetRevision: "HEAD",
							Path:           "dry/path",
						},
						SyncSource: v1alpha1.SyncSource{
							TargetBranch: "master",
							Path:         "sync/path",
						},
					},
				},
			},
			changedFile: "sync/path/app.yaml",
			hasRefresh:  true,
			hasHydrate:  false,
			updateCache: false,
		},
		{
			name: "source hydrator dry source without annotation - always refreshes and hydrates",
			app: &v1alpha1.Application{
				Name:      "test-app",
				Namespace: "argocd",
				Spec: v1alpha1.ApplicationSpec{
					SourceHydrator: &v1alpha1.SourceHydrator{
						DrySource: v1alpha1.DrySource{
							RepoURL:        "https://github.com/jessesuen/test-repo",
							TargetRevision: "HEAD",
							Path:           "dry/path",
						},
						SyncSource: v1alpha1.SyncSource{
							TargetBranch: "master",
							Path:         "sync/path",
						},
					},
				},
			},
			changedFile: "other/path/app.yaml",
			hasRefresh:  true,
			hasHydrate:  true,
			updateCache: false,
		},
		{
			name: "source hydrator sync source with annotation - refresh only",
			app: &v1alpha1.Application{
				Name:      "test-app",
				Namespace: "argocd",
				Annotations: map[string]string{
					"argocd.argoproj.io/manifest-generate-paths": "deploy",
				},
				Spec: v1alpha1.ApplicationSpec{
					SourceHydrator: &v1alpha1.SourceHydrator{
						DrySource: v1alpha1.DrySource{
							RepoURL:        "https://github.com/jessesuen/test-repo",
							TargetRevision: "HEAD",
							Path:           "dry/path",
						},
						SyncSource: v1alpha1.SyncSource{
							TargetBranch: "master",
							Path:         "sync/path",
						},
					},
				},
			},
			changedFile: "sync/path/deploy/app.yaml",
			hasRefresh:  true,
			hasHydrate:  false,
			updateCache: false,
		},
		{
			name: "source hydrator dry source with annotation - refresh and hydrate",
			app: &v1alpha1.Application{
				Name:      "test-app",
				Namespace: "argocd",
				Annotations: map[string]string{
					"argocd.argoproj.io/manifest-generate-paths": "deploy",
				},
				Spec: v1alpha1.ApplicationSpec{
					SourceHydrator: &v1alpha1.SourceHydrator{
						DrySource: v1alpha1.DrySource{
							RepoURL:        "https://github.com/jessesuen/test-repo",
							TargetRevision: "HEAD",
							Path:           "dry/path",
						},
						SyncSource: v1alpha1.SyncSource{
							TargetBranch: "master",
							Path:         "sync/path",
						},
					},
				},
			},
			changedFile: "dry/path/deploy/app.yaml",
			hasRefresh:  true,
			hasHydrate:  true,
			updateCache: false,
		},
		{
			name: "source hydrator dry source with annotation - non-matching file updates cache",
			app: &v1alpha1.Application{
				Name:      "test-app",
				Namespace: "argocd",
				Annotations: map[string]string{
					"argocd.argoproj.io/manifest-generate-paths": "deploy",
				},
				Spec: v1alpha1.ApplicationSpec{
					SourceHydrator: &v1alpha1.SourceHydrator{
						DrySource: v1alpha1.DrySource{
							RepoURL:        "https://github.com/jessesuen/test-repo",
							TargetRevision: "HEAD",
							Path:           "dry/path",
						},
						SyncSource: v1alpha1.SyncSource{
							TargetBranch: "master",
							Path:         "sync/path",
						},
					},
				},
			},
			changedFile: "dry/path/other/app.yaml",
			hasRefresh:  false,
			hasHydrate:  false,
			updateCache: true,
		},
	}

	for _, tt := range tests {
		ttc := tt
		t.Run(ttc.name, func(t *testing.T) {
			t.Parallel()

			var patchData []byte
			var patched bool
			reaction := func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
				if action.GetVerb() == "patch" {
					patchAction := action.(kubetesting.PatchAction)
					patchData = patchAction.GetPatch()
					patched = true
				}
				return true, nil, nil
			}

			// Setup cache
			inMemoryCache := cacheutil.NewInMemoryCache(1 * time.Hour)
			cacheClient := cacheutil.NewCache(inMemoryCache)
			repoCache := cache.NewCache(
				cacheClient,
				1*time.Minute,
				1*time.Minute,
				10*time.Second,
			)

			// Pre-populate cache with beforeSHA if we're testing cache updates
			if ttc.updateCache {
				var source *v1alpha1.ApplicationSource
				if ttc.app.Spec.SourceHydrator != nil {
					drySource := ttc.app.Spec.SourceHydrator.GetDrySource()
					source = &drySource
				} else if len(ttc.app.Spec.Sources) > 0 {
					source = &ttc.app.Spec.Sources[0]
				}
				if source != nil {
					setupTestCache(t, repoCache, ttc.app.Name, source, nil, []string{"test-manifest"})
				}
			}

			// Setup server cache with cluster info
			serverCache := servercache.NewCache(appstate.NewCache(cacheClient, time.Minute), time.Minute, time.Minute)
			mockDB := &mocks.ArgoDB{}

			// Set destination if not present (required for cache updates)
			if ttc.app.Spec.Destination.Server == "" {
				ttc.app.Spec.Destination.Server = testClusterURL
			}

			mockDB.EXPECT().GetCluster(mock.Anything, testClusterURL).Return(&v1alpha1.Cluster{
				Server: testClusterURL,
				Info: v1alpha1.ClusterInfo{
					ServerVersion:   "1.28.0",
					ConnectionState: v1alpha1.ConnectionState{Status: v1alpha1.ConnectionStatusSuccessful},
					APIVersions:     []string{},
				},
			}, nil).Maybe()
			mockDB.EXPECT().ListRepositories(mock.Anything).Return([]*v1alpha1.Repository{}, nil).Maybe()

			err := serverCache.SetClusterInfo(testClusterURL, &v1alpha1.ClusterInfo{
				ServerVersion:   "1.28.0",
				ConnectionState: v1alpha1.ConnectionState{Status: v1alpha1.ConnectionStatusSuccessful},
				APIVersions:     []string{},
			})
			require.NoError(t, err)

			// Create handler with reaction
			appClientset := appclientset.NewSimpleClientset(ttc.app)
			defaultReactor := appClientset.ReactionChain[0]
			appClientset.ReactionChain = nil
			appClientset.AddReactor("list", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
				return defaultReactor.React(action)
			})
			appClientset.AddReactor("patch", "applications", reaction)

			h := NewHandler(
				"argocd",
				[]string{},
				10,
				1,
				appClientset,
				&fakeAppsLister{clientset: appClientset},
				&settings.ArgoCDSettings{},
				&fakeSettingsSrc{},
				repoCache,
				serverCache,
				mockDB,
				int64(50)*1024*1024,
				0,
				10,
				&fakeProjectNamespaceLister{clientset: appClientset, namespace: "argocd"},
			)

			// Create payload with the changed file
			payload := createTestPayload(ttc.changedFile)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", http.NoBody)
			req.Header.Set("X-GitHub-Event", "push")
			req.Body = io.NopCloser(bytes.NewReader(payload))

			w := httptest.NewRecorder()
			h.Handler(w, req)

			// Wait briefly for refresh processing
			time.Sleep(50 * time.Millisecond)

			// Shutdown properly
			h.Shutdown()

			assert.Equal(t, http.StatusOK, w.Code)

			// Verify refresh behavior
			assert.Equal(t, ttc.hasRefresh, patched, "patch status mismatch for test: %s", ttc.name)
			if patched && patchData != nil {
				verifyAnnotations(t, patchData, ttc.hasRefresh, ttc.hasHydrate)
			}

			// Verify cache update behavior
			if ttc.updateCache {
				var source *v1alpha1.ApplicationSource
				if ttc.app.Spec.SourceHydrator != nil {
					drySource := ttc.app.Spec.SourceHydrator.GetDrySource()
					source = &drySource
				} else if len(ttc.app.Spec.Sources) > 0 {
					source = &ttc.app.Spec.Sources[0]
				}
				if source != nil {
					// Verify cache was updated with afterSHA
					clusterInfo := &mockClusterInfo{}
					var afterManifests cache.CachedManifestResponse
					key := cache.NewManifestKey(testAfterSHA, source, nil, "", "", testAppLabelKey, ttc.app.Name, "", nil, clusterInfo, nil)
					err := repoCache.GetManifests(key, &afterManifests)
					require.NoError(t, err, "cache should be updated with afterSHA")
					if err == nil {
						assert.Equal(t, testAfterSHA, afterManifests.ManifestResponse.Revision, "cached revision should match afterSHA")
					}
				}
			}
		})
	}
}

func Test_storePreviouslyCachedManifests(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		app          *v1alpha1.Application
		project      *v1alpha1.AppProject
		seedCache    bool // seed the cache with manifests for the previous revision
		cacheUpdated bool // cache should be updated with the new revision
		errExpected  bool
	}{
		{
			name: "single source with source integrity",
			app: &v1alpha1.Application{
				Name:      "test-app",
				Namespace: "argocd",
				Annotations: map[string]string{
					"argocd.argoproj.io/manifest-generate-paths": "deploy",
				},
				Spec: v1alpha1.ApplicationSpec{
					Project: "default",
					Source: &v1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/test/repo",
						TargetRevision: "main",
					},
				},
			},
			project: &v1alpha1.AppProject{
				Name:      "default",
				Namespace: "argocd",
				Spec: v1alpha1.AppProjectSpec{
					SourceRepos: []string{"*"},
					Destinations: []v1alpha1.ApplicationDestination{
						{
							Server:    "*",
							Namespace: "*",
						},
					},
					SourceIntegrity: &v1alpha1.SourceIntegrity{
						Git: &v1alpha1.SourceIntegrityGit{
							Policies: []*v1alpha1.SourceIntegrityGitPolicy{{
								GPG: &v1alpha1.SourceIntegrityGitPolicyGPG{
									Mode: v1alpha1.SourceIntegrityGitPolicyGPGModeStrict,
									Keys: []string{"4AEE18F83AFDEB23"},
								},
							}},
						},
					},
				},
			},
			seedCache:    true,
			cacheUpdated: true,
			errExpected:  false,
		},
		{
			name: "application outside the control plane namespace is keyed by its instance name",
			app: &v1alpha1.Application{
				Name:      "test-app",
				Namespace: "argocd-apps",
				Annotations: map[string]string{
					"argocd.argoproj.io/manifest-generate-paths": "deploy",
				},
				Spec: v1alpha1.ApplicationSpec{
					Project: "default",
					Source: &v1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/test/repo",
						TargetRevision: "main",
					},
				},
			},
			project: &v1alpha1.AppProject{
				Name:      "default",
				Namespace: "argocd",
				Spec: v1alpha1.AppProjectSpec{
					SourceRepos: []string{"*"},
					Destinations: []v1alpha1.ApplicationDestination{
						{
							Server:    "*",
							Namespace: "*",
						},
					},
				},
			},
			seedCache:    true,
			cacheUpdated: true,
			errExpected:  false,
		},
		{
			name: "single source rename cache miss is not an error",
			app: &v1alpha1.Application{
				Name:      "test-app",
				Namespace: "argocd",
				Annotations: map[string]string{
					"argocd.argoproj.io/manifest-generate-paths": "deploy",
				},
				Spec: v1alpha1.ApplicationSpec{
					Project: "default",
					Source: &v1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/test/repo",
						TargetRevision: "main",
					},
				},
			},
			project: &v1alpha1.AppProject{
				Name:      "default",
				Namespace: "argocd",
				Spec: v1alpha1.AppProjectSpec{
					SourceRepos: []string{"*"},
					Destinations: []v1alpha1.ApplicationDestination{
						{
							Server:    "*",
							Namespace: "*",
						},
					},
				},
			},
			seedCache:    false,
			cacheUpdated: false,
			errExpected:  false,
		},
		{
			name: "error not support refSourceCommitSHAs",
			app: &v1alpha1.Application{
				Name:      "test-app",
				Namespace: "argocd",
				Annotations: map[string]string{
					"argocd.argoproj.io/manifest-generate-paths": "deploy",
				},
				Spec: v1alpha1.ApplicationSpec{
					Sources: v1alpha1.ApplicationSources{
						{
							RepoURL:        "https://github.com/test/repo",
							TargetRevision: "main",
							Helm: &v1alpha1.ApplicationSourceHelm{
								ValueFiles: []string{"$myref/test.yaml"},
							},
						},
						{
							RepoURL:        "https://github.com/test/repo",
							TargetRevision: "main",
							Ref:            "myref",
						},
					},
				},
			},
			cacheUpdated: false,
			errExpected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockDB := mocks.ArgoDB{}

			inMemoryCache := cacheutil.NewInMemoryCache(1 * time.Hour)
			cacheClient := cacheutil.NewCache(inMemoryCache)
			serverCache := servercache.NewCache(appstate.NewCache(cacheClient, time.Minute), time.Minute, time.Minute)
			repoCache := cache.NewCache(cacheClient, 1*time.Minute, 1*time.Minute, 10*time.Second)

			var source *v1alpha1.ApplicationSource
			if tt.app.Spec.Source != nil {
				source = tt.app.Spec.Source
			} else if tt.app.Spec.Sources != nil {
				source = &tt.app.Spec.Sources[0]
			}

			if tt.app.Spec.Destination.Server == "" {
				tt.app.Spec.Destination.Server = testClusterURL
			}

			if tt.project == nil {
				tt.project = &v1alpha1.AppProject{}
			}

			mockDB.EXPECT().GetCluster(mock.Anything, testClusterURL).Return(&v1alpha1.Cluster{
				Server: testClusterURL,
				Info: v1alpha1.ClusterInfo{
					ServerVersion:   "1.28.0",
					ConnectionState: v1alpha1.ConnectionState{Status: v1alpha1.ConnectionStatusSuccessful},
					APIVersions:     []string{},
				},
			}, nil).Maybe()

			appClientset := appclientset.NewSimpleClientset(tt.app, tt.project)

			err := serverCache.SetClusterInfo(testClusterURL, &v1alpha1.ClusterInfo{
				ServerVersion:   "1.28.0",
				ConnectionState: v1alpha1.ConnectionState{Status: v1alpha1.ConnectionStatusSuccessful},
				APIVersions:     []string{},
			})
			require.NoError(t, err)

			mockDB.EXPECT().ListRepositories(mock.Anything).Return([]*v1alpha1.Repository{}, nil).Maybe()
			mockDB.EXPECT().GetRepository(mock.Anything, mock.Anything, mock.Anything).Return(&v1alpha1.Repository{}, nil).Maybe()

			h := NewHandler(
				"argocd",
				[]string{},
				10,
				5,
				appClientset,
				&fakeAppsLister{clientset: appClientset},
				&settings.ArgoCDSettings{},
				&fakeSettingsSrc{},
				repoCache,
				serverCache,
				&mockDB,
				int64(50)*1024*1024,
				2*time.Second,
				100, // High threshold - won't be exceeded
				&fakeProjectNamespaceLister{clientset: appClientset, namespace: "argocd"},
			)

			if tt.seedCache {
				setupTestCache(t, repoCache, tt.app.InstanceName("argocd"), source, tt.project.EffectiveSourceIntegrity(), []string{"test-manifest"})
			}
			logger, _ := test.NewNullLogger()
			logCtx := logger.WithField("application", tt.app.Name)
			err = h.storePreviouslyCachedManifests(logCtx, tt.app, changeInfo{shaBefore: testBeforeSHA, shaAfter: testAfterSHA}, "", testAppLabelKey, "", *source)

			if tt.errExpected {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.cacheUpdated {
				clusterInfo := &mockClusterInfo{}
				var sourceIntegrity *v1alpha1.SourceIntegrity
				if tt.project != nil {
					sourceIntegrity = tt.project.EffectiveSourceIntegrity()
				}
				key := cache.NewManifestKey(testAfterSHA, source, nil, "", "", testAppLabelKey, tt.app.InstanceName("argocd"), "", sourceIntegrity, clusterInfo, nil)
				err = repoCache.GetManifests(key, &cache.CachedManifestResponse{})
				require.NoError(t, err)
			}
		})
	}
}

// createTestPayload creates a GitHub push event payload with the specified changed file
func createTestPayload(changedFile string) []byte {
	payload := fmt.Sprintf(`{
		"ref": "refs/heads/master",
		"before": "%s",
		"after": "%s",
		"repository": {
			"html_url": "https://github.com/jessesuen/test-repo",
			"default_branch": "master"
		},
		"commits": [
			{
				"added": [],
				"modified": ["%s"],
				"removed": []
			}
		]
	}`, testBeforeSHA, testAfterSHA, changedFile)
	return []byte(payload)
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
    "name": "{{.name}}",
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

	bitbucketPushPayload := func(branchName, owner, repo, name string) bitbucket.RepoPushPayload {
		// The payload's "push.changes[0].new.name" member seems to only have the branch name (based on the example payload).
		// https://support.atlassian.com/bitbucket-cloud/docs/event-payloads/#EventPayloads-Push
		var pl bitbucket.RepoPushPayload
		var doc bytes.Buffer
		err = tmpl.Execute(&doc, map[string]string{
			"branch":  branchName,
			"owner":   owner,
			"repo":    repo,
			"name":    name,
			"oldHash": "abcdef",
			"newHash": "ghijkl",
		})
		require.NoError(t, err)
		err = json.Unmarshal(doc.Bytes(), &pl)
		require.NoError(t, err)
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
			bitbucketPushPayload("release-0.0", "test-owner", "test-repo", "test-repo"),
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
			bitbucketPushPayload("main", "test-owner", "test-repo", "test-repo"),
			true,
			[]string{"guestbook/guestbook-ui-deployment.yaml"},
			changeInfo{
				shaBefore: "abcdef",
				shaAfter:  "ghijkl",
			},
		},
		{
			"bitbucket display name is mixed case, differs from repo slug",
			false,
			"main",
			bitbucketPushPayload("main", "test-owner", "test-repo", "Test Repo"),
			true,
			[]string{"guestbook/guestbook-ui-deployment.yaml"},
			changeInfo{
				shaBefore: "abcdef",
				shaAfter:  "ghijkl",
			},
		},
		{
			"bitbucket display name is all uppercase, differs from repo slug",
			false,
			"main",
			bitbucketPushPayload("main", "test-owner", "test-repo", "TESTREPO"),
			true,
			[]string{"guestbook/guestbook-ui-deployment.yaml"},
			changeInfo{
				shaBefore: "abcdef",
				shaAfter:  "ghijkl",
			},
		},
		{
			"bitbucket display name is all lowercase, differs from repo slug",
			false,
			"main",
			bitbucketPushPayload("main", "test-owner", "test-repo", "testrepo"),
			true,
			[]string{"guestbook/guestbook-ui-deployment.yaml"},
			changeInfo{
				shaBefore: "abcdef",
				shaAfter:  "ghijkl",
			},
		},
		{
			"bitbucket display name is all uppercase with spaces, differs from repo slug",
			false,
			"main",
			bitbucketPushPayload("main", "test-owner", "test-repo", "TEST REPO"),
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
			_, revisionFromHook, change, touchHead, changedFiles := h.affectedRevisionInfo(t.Context(), testCase.hookPayload)
			require.Equal(t, testCase.revision, revisionFromHook)
			require.Equal(t, testCase.expectedTouchHead, touchHead)
			require.Equal(t, testCase.expectedChangedFiles, changedFiles())
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
	client, err := bb.NewOAuthbearerToken("")
	require.NoError(t, err)
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
	client, err := bb.NewOAuthbearerToken("")
	require.NoError(t, err)
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

// mockClusterInfo implements cache.ClusterRuntimeInfo for testing
type mockClusterInfo struct{}

func (m *mockClusterInfo) GetApiVersions() []string { return []string{} } //nolint:revive // interface method name
func (m *mockClusterInfo) GetKubeVersion() string   { return "1.28.0" }

// Common test constants
const (
	testBeforeSHA   = "d5c1ffa8e294bc18c639bfb4e0df499251034414"
	testAfterSHA    = "63738bb582c8b540af7bcfc18f87c575c3ed66e0"
	testClusterURL  = "https://kubernetes.default.svc"
	testAppLabelKey = "mycompany.com/appname"
)

// verifyAnnotations is a helper that checks if the expected annotations are present in patch data
func verifyAnnotations(t *testing.T, patchData []byte, expectRefresh bool, expectHydrate bool) {
	t.Helper()
	if patchData == nil {
		if expectRefresh {
			t.Error("expected app to be patched but patchData is nil")
		}
		return
	}

	var patchMap map[string]any
	err := json.Unmarshal(patchData, &patchMap)
	require.NoError(t, err)

	metadata, hasMetadata := patchMap["metadata"].(map[string]any)
	require.True(t, hasMetadata, "patch should have metadata")

	annotations, hasAnnotations := metadata["annotations"].(map[string]any)
	require.True(t, hasAnnotations, "patch should have annotations")

	// Check refresh annotation
	refreshValue, hasRefresh := annotations["argocd.argoproj.io/refresh"]
	if expectRefresh {
		assert.True(t, hasRefresh, "should have refresh annotation")
		assert.Equal(t, "normal", refreshValue, "refresh annotation should be 'normal'")
	} else {
		assert.False(t, hasRefresh, "should not have refresh annotation")
	}

	// Check hydrate annotation
	hydrateValue, hasHydrate := annotations["argocd.argoproj.io/hydrate"]
	if expectHydrate {
		assert.True(t, hasHydrate, "should have hydrate annotation")
		assert.Equal(t, "normal", hydrateValue, "hydrate annotation should be 'normal'")
	} else {
		assert.False(t, hasHydrate, "should not have hydrate annotation")
	}
}

// setupTestCache is a helper that creates and populates a test cache
func setupTestCache(t *testing.T, repoCache *cache.Cache, appName string, source *v1alpha1.ApplicationSource, sourceIntegrity *v1alpha1.SourceIntegrity, manifests []string) {
	t.Helper()
	clusterInfo := &mockClusterInfo{}
	dummyManifests := &cache.CachedManifestResponse{
		ManifestResponse: &apiclient.ManifestResponse{
			Revision:  testBeforeSHA,
			Manifests: manifests,
			Namespace: "",
			Server:    testClusterURL,
		},
	}
	key := cache.NewManifestKey(testBeforeSHA, source, make(map[string]*v1alpha1.RefTarget), "", "", testAppLabelKey, appName, "", sourceIntegrity, clusterInfo, nil)
	err := repoCache.SetManifests(key, dummyManifests)
	require.NoError(t, err)
}

func TestWebhookRefreshWithJitter(t *testing.T) {
	app := v1alpha1.Application{
		Name:      "test-app",
		Namespace: "argocd",
		Spec: v1alpha1.ApplicationSpec{
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/test/repo",
				TargetRevision: "main",
			},
		},
	}

	t.Run("items are processed after jitter delay", func(t *testing.T) {
		var patched bool
		reaction := func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			if action.GetVerb() == "patch" {
				patched = true
			}
			return true, nil, nil
		}

		appClientset := appclientset.NewSimpleClientset(&app)
		defaultReactor := appClientset.ReactionChain[0]
		appClientset.ReactionChain = nil
		appClientset.AddReactor("list", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return defaultReactor.React(action)
		})
		appClientset.AddReactor("patch", "applications", reaction)

		cacheClient := cacheutil.NewCache(cacheutil.NewInMemoryCache(1 * time.Hour))
		h := NewHandler(
			"argocd",
			[]string{},
			10,
			5,
			appClientset,
			&fakeAppsLister{clientset: appClientset},
			&settings.ArgoCDSettings{},
			&fakeSettingsSrc{},
			cache.NewCache(cacheClient, 1*time.Minute, 1*time.Minute, 10*time.Second),
			servercache.NewCache(appstate.NewCache(cacheClient, time.Minute), time.Minute, time.Minute),
			&mocks.ArgoDB{},
			int64(50)*1024*1024,
			2*time.Second,
			10,
			&fakeProjectNamespaceLister{clientset: appClientset, namespace: "argocd"},
		)

		req := &appRefreshRequest{
			appName:      "test-app",
			appNamespace: "argocd",
			hydrateType:  nil,
		}

		startTime := time.Now()
		h.refreshQueue.AddAfter(req, 500*time.Millisecond)

		// Wait for processing
		time.Sleep(800 * time.Millisecond)
		h.Shutdown()

		elapsed := time.Since(startTime)
		// Verify delay was applied
		assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(500))
		// Verify app was actually patched (refresh processed)
		assert.True(t, patched, "app should be patched after jitter delay")
	})

	t.Run("no jitter applied when threshold not exceeded", func(_ *testing.T) {
		appClientset := appclientset.NewSimpleClientset(&app)
		cacheClient := cacheutil.NewCache(cacheutil.NewInMemoryCache(1 * time.Hour))
		h := NewHandler(
			"argocd",
			[]string{},
			10,
			5,
			appClientset,
			&fakeAppsLister{clientset: appClientset},
			&settings.ArgoCDSettings{},
			&fakeSettingsSrc{},
			cache.NewCache(cacheClient, 1*time.Minute, 1*time.Minute, 10*time.Second),
			servercache.NewCache(appstate.NewCache(cacheClient, time.Minute), time.Minute, time.Minute),
			&mocks.ArgoDB{},
			int64(50)*1024*1024,
			2*time.Second,
			100, // High threshold - won't be exceeded
			&fakeProjectNamespaceLister{clientset: appClientset, namespace: "argocd"},
		)

		req := &appRefreshRequest{
			appName:      "test-app",
			appNamespace: "argocd",
			hydrateType:  nil,
		}

		// Add without explicit delay
		h.refreshQueue.Add(req)

		// Should process quickly without jitter
		time.Sleep(100 * time.Millisecond)
		h.Shutdown()
	})
}

func TestProcessAppRefresh(t *testing.T) {
	app := v1alpha1.Application{
		Name:      "test-app",
		Namespace: "argocd",
		Spec: v1alpha1.ApplicationSpec{
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/test/repo",
				TargetRevision: "main",
			},
		},
	}

	t.Run("processes valid refresh request", func(t *testing.T) {
		hook := test.NewGlobal()
		defer hook.Reset()

		h := NewMockHandler(nil, []string{}, &app)
		req := &appRefreshRequest{
			appName:      "test-app",
			appNamespace: "argocd",
			hydrateType:  nil,
		}

		h.processAppRefresh(req)
		h.Shutdown()

		assert.Contains(t, hook.LastEntry().Message, "Requested app 'test-app' refresh")
	})

	t.Run("processes valid hydrate request", func(t *testing.T) {
		hook := test.NewGlobal()
		defer hook.Reset()

		hydrateType := v1alpha1.HydrateTypeNormal
		h := NewMockHandler(nil, []string{}, &app)
		req := &appRefreshRequest{
			appName:      "test-app",
			appNamespace: "argocd",
			hydrateType:  &hydrateType,
		}

		h.processAppRefresh(req)
		h.Shutdown()

		assert.Contains(t, hook.LastEntry().Message, "Requested app 'test-app' hydration")
	})
}

func NewMockHandlerForBitbucketServerCallback(reactor *reactorDef, applicationNamespaces []string, objects ...runtime.Object) *ArgoCDWebhookHandler {
	mockDB := &mocks.ArgoDB{}
	mockDB.EXPECT().ListRepositories(mock.Anything).Return(
		[]*v1alpha1.Repository{
			{
				Repo:     "https://bitbucketserver/scm/myproject/test-repo.git",
				Username: "testuser",
				Password: "testpassword",
			},
			{
				Repo:     "https://bitbucketserver/scm/myproject/paged-repo.git",
				Username: "testuser",
				Password: "testpassword",
			},
			{
				Repo:     "https://bitbucketserver/scm/myproject/auth-error-repo.git",
				Username: "testuser",
				Password: "testpassword",
			},
		}, nil)
	argoSettings := settings.ArgoCDSettings{WebhookBitbucketServerSecret: "my-bb-server-secret"}
	defaultMaxPayloadSize := int64(50) * 1024 * 1024
	return newMockHandler(reactor, applicationNamespaces, defaultMaxPayloadSize, mockDB, &argoSettings, objects...)
}

// getBBServerChangesResponderFn returns a httpmock responder for the Bitbucket Server changes API
func getBBServerChangesResponderFn() func(req *http.Request) (*http.Response, error) {
	return func(_ *http.Request) (*http.Response, error) {
		changesResp := map[string]any{
			"values": []any{
				map[string]any{
					"contentId":     "abc123",
					"fromContentId": "def456",
					"path": map[string]any{
						"components": []any{"guestbook", "guestbook-ui-deployment.yaml"},
						"parent":     "guestbook",
						"name":       "guestbook-ui-deployment.yaml",
						"extension":  "yaml",
						"toString":   "guestbook/guestbook-ui-deployment.yaml",
					},
					"type":     "MODIFY",
					"nodeType": "FILE",
				},
			},
			"isLastPage": true,
			"start":      0,
			"limit":      25,
			"size":       1,
		}
		resp, err := httpmock.NewJsonResponse(200, changesResp)
		if err != nil {
			return httpmock.NewStringResponse(500, ""), nil
		}
		return resp, nil
	}
}

// getBBServerDefaultBranchResponderFn returns a httpmock responder for the Bitbucket Server default branch API
func getBBServerDefaultBranchResponderFn(defaultBranch string) func(req *http.Request) (*http.Response, error) {
	return func(_ *http.Request) (*http.Response, error) {
		branchResp := map[string]any{
			"id":           "refs/heads/" + defaultBranch,
			"displayId":    defaultBranch,
			"type":         "BRANCH",
			"isDefault":    true,
			"latestCommit": "abc123",
		}
		resp, err := httpmock.NewJsonResponse(200, branchResp)
		if err != nil {
			return httpmock.NewStringResponse(500, ""), nil
		}
		return resp, nil
	}
}

func NewMockHandlerForBitbucketServerNoSecret(reactor *reactorDef, applicationNamespaces []string, objects ...runtime.Object) *ArgoCDWebhookHandler {
	mockDB := &mocks.ArgoDB{}
	// No ListRepositories expectation because it must never be called without a secret.
	argoSettings := settings.ArgoCDSettings{} // no WebhookBitbucketServerSecret
	defaultMaxPayloadSize := int64(50) * 1024 * 1024
	return newMockHandler(reactor, applicationNamespaces, defaultMaxPayloadSize, mockDB, &argoSettings, objects...)
}

// getBBServer401ResponderFn returns a 401 Unauthorized responder to simulate auth errors.
func getBBServer401ResponderFn() func(req *http.Request) (*http.Response, error) {
	return func(_ *http.Request) (*http.Response, error) {
		return httpmock.NewStringResponse(401, `{"errors":[{"message":"Unauthenticated"}]}`), nil
	}
}

// getBBServerPagedChangesResponderFn returns a two-page stateful responder for the Bitbucket Server changes API.
func getBBServerPagedChangesResponderFn() func(req *http.Request) (*http.Response, error) {
	callCount := 0
	return func(_ *http.Request) (*http.Response, error) {
		callCount++
		var body map[string]any
		if callCount == 1 {
			body = map[string]any{
				"values": []any{
					map[string]any{"path": map[string]any{"toString": "base/deployment.yaml"}},
				},
				"isLastPage":    false,
				"nextPageStart": float64(1),
				"limit":         1,
			}
		} else {
			body = map[string]any{
				"values": []any{
					map[string]any{"path": map[string]any{"toString": "base/service.yaml"}},
				},
				"isLastPage": true,
				"limit":      1,
			}
		}
		resp, err := httpmock.NewJsonResponse(200, body)
		if err != nil {
			return httpmock.NewStringResponse(500, ""), nil
		}
		return resp, nil
	}
}

func Test_affectedRevisionInfo_bbserver_changed_files(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder("GET",
		`=~^https://bitbucketserver/rest/api/1.0/projects/MYPROJECT/repos/test-repo/changes`,
		getBBServerChangesResponderFn())
	httpmock.RegisterResponder("GET",
		"https://bitbucketserver/rest/api/1.0/projects/MYPROJECT/repos/test-repo/branches/default",
		getBBServerDefaultBranchResponderFn("master"))
	httpmock.RegisterResponder("GET",
		`=~^https://bitbucketserver/rest/api/1.0/projects/MYPROJECT/repos/paged-repo/changes`,
		getBBServerPagedChangesResponderFn())
	httpmock.RegisterResponder("GET",
		"https://bitbucketserver/rest/api/1.0/projects/MYPROJECT/repos/paged-repo/branches/default",
		getBBServerDefaultBranchResponderFn("master"))
	httpmock.RegisterResponder("GET",
		`=~^https://bitbucketserver/rest/api/1.0/projects/MYPROJECT/repos/auth-error-repo/`,
		getBBServer401ResponderFn())

	// payload with both HTTP and SSH clone links
	bbServerPayload := func(branchName, projectKey, repoSlug, fromHash, toHash string) bitbucketserver.RepositoryReferenceChangedPayload {
		return bitbucketserver.RepositoryReferenceChangedPayload{
			Changes: []bitbucketserver.RepositoryChange{
				{
					Reference:   bitbucketserver.RepositoryReference{ID: "refs/heads/" + branchName},
					ReferenceID: "refs/heads/" + branchName,
					FromHash:    fromHash,
					ToHash:      toHash,
					Type:        "UPDATE",
				},
			},
			Repository: bitbucketserver.Repository{
				Slug: repoSlug,
				Project: bitbucketserver.Project{
					Key: projectKey,
				},
				Links: map[string]any{
					"clone": []any{
						map[string]any{
							"href": "https://bitbucketserver/scm/" + strings.ToLower(projectKey) + "/" + repoSlug + ".git",
							"name": "http",
						},
						map[string]any{
							"href": "ssh://git@bitbucketserver:7999/" + strings.ToLower(projectKey) + "/" + repoSlug + ".git",
							"name": "ssh",
						},
					},
				},
			},
		}
	}

	// payload with SSH clone link only (no HTTP)
	bbServerSSHOnlyPayload := func(branchName, projectKey, repoSlug, fromHash, toHash string) bitbucketserver.RepositoryReferenceChangedPayload {
		p := bbServerPayload(branchName, projectKey, repoSlug, fromHash, toHash)
		p.Repository.Links = map[string]any{
			"clone": []any{
				map[string]any{
					"href": "ssh://git@bitbucketserver:7999/" + strings.ToLower(projectKey) + "/" + repoSlug + ".git",
					"name": "ssh",
				},
			},
		}
		return p
	}

	tests := []struct {
		name                 string
		revision             string
		hookPayload          bitbucketserver.RepositoryReferenceChangedPayload
		handler              func() *ArgoCDWebhookHandler
		expectedTouchHead    bool
		expectedChangedFiles []string
		expectedChangeInfo   changeInfo
	}{
		{
			name:                 "push to non-default branch",
			revision:             "feature-branch",
			hookPayload:          bbServerPayload("feature-branch", "MYPROJECT", "test-repo", "abcdef", "ghijkl"),
			handler:              func() *ArgoCDWebhookHandler { return NewMockHandlerForBitbucketServerCallback(nil, []string{}) },
			expectedTouchHead:    false,
			expectedChangedFiles: []string{"guestbook/guestbook-ui-deployment.yaml"},
			expectedChangeInfo:   changeInfo{shaBefore: "abcdef", shaAfter: "ghijkl"},
		},
		{
			name:                 "push to default branch",
			revision:             "master",
			hookPayload:          bbServerPayload("master", "MYPROJECT", "test-repo", "abcdef", "ghijkl"),
			handler:              func() *ArgoCDWebhookHandler { return NewMockHandlerForBitbucketServerCallback(nil, []string{}) },
			expectedTouchHead:    true,
			expectedChangedFiles: []string{"guestbook/guestbook-ui-deployment.yaml"},
			expectedChangeInfo:   changeInfo{shaBefore: "abcdef", shaAfter: "ghijkl"},
		},
		{
			// Without a secret the API must never be called; changed files stay nil and
			// touchedHead defaults to true (safe fallback).
			name:                 "no secret configured - no API call, touchedHead defaults to true",
			revision:             "master",
			hookPayload:          bbServerPayload("master", "MYPROJECT", "test-repo", "abcdef", "ghijkl"),
			handler:              func() *ArgoCDWebhookHandler { return NewMockHandlerForBitbucketServerNoSecret(nil, []string{}) },
			expectedTouchHead:    true,
			expectedChangedFiles: nil,
			expectedChangeInfo:   changeInfo{shaBefore: "abcdef", shaAfter: "ghijkl"},
		},
		{
			// When only an SSH clone URL is present httpCloneURL remains empty and the API
			// block is skipped, same as no-secret.
			name:                 "SSH-only clone URL - API skipped, touchedHead defaults to true",
			revision:             "master",
			hookPayload:          bbServerSSHOnlyPayload("master", "MYPROJECT", "test-repo", "abcdef", "ghijkl"),
			handler:              func() *ArgoCDWebhookHandler { return NewMockHandlerForBitbucketServerCallback(nil, []string{}) },
			expectedTouchHead:    true,
			expectedChangedFiles: nil,
			expectedChangeInfo:   changeInfo{shaBefore: "abcdef", shaAfter: "ghijkl"},
		},
		{
			// Changes API returns two pages; both pages should be collected.
			name:                 "paginated changes response",
			revision:             "master",
			hookPayload:          bbServerPayload("master", "MYPROJECT", "paged-repo", "abcdef", "ghijkl"),
			handler:              func() *ArgoCDWebhookHandler { return NewMockHandlerForBitbucketServerCallback(nil, []string{}) },
			expectedTouchHead:    true,
			expectedChangedFiles: []string{"base/deployment.yaml", "base/service.yaml"},
			expectedChangeInfo:   changeInfo{shaBefore: "abcdef", shaAfter: "ghijkl"},
		},
		{
			// When the Bitbucket Server API returns a 401 error changed files stay nil and
			// touchedHead defaults to true so the controller performs a safe full-refresh.
			name:                 "API auth error - no changed files, touchedHead defaults to true",
			revision:             "master",
			hookPayload:          bbServerPayload("master", "MYPROJECT", "auth-error-repo", "abcdef", "ghijkl"),
			handler:              func() *ArgoCDWebhookHandler { return NewMockHandlerForBitbucketServerCallback(nil, []string{}) },
			expectedTouchHead:    true,
			expectedChangedFiles: nil,
			expectedChangeInfo:   changeInfo{shaBefore: "abcdef", shaAfter: "ghijkl"},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			h := testCase.handler()
			_, revisionFromHook, change, touchHead, changedFiles := h.affectedRevisionInfo(t.Context(), testCase.hookPayload)
			require.Equal(t, testCase.revision, revisionFromHook)
			require.Equal(t, testCase.expectedTouchHead, touchHead)
			require.Equal(t, testCase.expectedChangedFiles, changedFiles())
			require.Equal(t, testCase.expectedChangeInfo, change)
		})
	}
}

func TestExtractBBServerBaseURL(t *testing.T) {
	tests := []struct {
		name        string
		cloneURL    string
		expected    string
		expectedErr string
	}{
		{
			name:     "standard http clone URL",
			cloneURL: "https://bitbucketserver/scm/myproject/test-repo.git",
			expected: "https://bitbucketserver",
		},
		{
			name:     "personal project clone URL",
			cloneURL: "https://bitbucketserver/scm/~testuser/test_a.git",
			expected: "https://bitbucketserver",
		},
		{
			name:     "bitbucket server deployed under a subpath",
			cloneURL: "https://mycompany.com/bitbucket/scm/myproject/test-repo.git",
			expected: "https://mycompany.com/bitbucket",
		},
		{
			name:        "invalid URL missing host",
			cloneURL:    "/scm/myproject/test-repo.git",
			expectedErr: "invalid Bitbucket Server clone URL",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractBBServerBaseURL(tt.cloneURL)
			if tt.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, got)
			}
		})
	}
}

func TestFetchChangesFromBitbucketServer(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder("GET",
		`=~^https://bitbucketserver/rest/api/1.0/projects/MYPROJECT/repos/test-repo/changes`,
		getBBServerChangesResponderFn())

	client := newBitbucketServerClient(t.Context(), &v1alpha1.Repository{}, "https://bitbucketserver")

	tt := []struct {
		name                string
		projectKey          string
		repoSlug            string
		fromHash            string
		toHash              string
		expectedLen         int
		expectedFileChanged string
		expectedErrString   string
	}{
		{
			name:                "valid project and repo",
			projectKey:          "MYPROJECT",
			repoSlug:            "test-repo",
			fromHash:            "abcdef",
			toHash:              "ghijkl",
			expectedLen:         1,
			expectedFileChanged: "guestbook/guestbook-ui-deployment.yaml",
		},
		{
			name:              "unknown repo",
			projectKey:        "MYPROJECT",
			repoSlug:          "unknown-repo",
			fromHash:          "abcdef",
			toHash:            "ghijkl",
			expectedErrString: "error fetching changes from Bitbucket Server",
		},
	}
	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			changedFiles, err := fetchChangesFromBitbucketServer(client, test.projectKey, test.repoSlug, test.fromHash, test.toHash)
			if test.expectedErrString == "" {
				require.NoError(t, err)
				require.Len(t, changedFiles, test.expectedLen)
				require.Equal(t, test.expectedFileChanged, changedFiles[0])
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.expectedErrString)
			}
		})
	}
}

func TestFetchChangesFromBitbucketServerPaginated(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET",
		`=~^https://bitbucketserver/rest/api/1.0/projects/MYPROJECT/repos/paged-repo/changes`,
		getBBServerPagedChangesResponderFn())

	client := newBitbucketServerClient(t.Context(), &v1alpha1.Repository{}, "https://bitbucketserver")
	changedFiles, err := fetchChangesFromBitbucketServer(client, "MYPROJECT", "paged-repo", "abcdef", "ghijkl")
	require.NoError(t, err)
	require.Equal(t, 2, httpmock.GetTotalCallCount(), "expected two API calls for two pages")
	require.Equal(t, []string{"base/deployment.yaml", "base/service.yaml"}, changedFiles)
}

func TestIsBBServerHeadTouched(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder("GET",
		"https://bitbucketserver/rest/api/1.0/projects/MYPROJECT/repos/test-repo/branches/default",
		getBBServerDefaultBranchResponderFn("master"))

	client := newBitbucketServerClient(t.Context(), &v1alpha1.Repository{}, "https://bitbucketserver")

	tt := []struct {
		name              string
		projectKey        string
		repoSlug          string
		revision          string
		expectedTouchHead bool
		expectedErrString string
	}{
		{
			name:              "revision matches default branch",
			projectKey:        "MYPROJECT",
			repoSlug:          "test-repo",
			revision:          "master",
			expectedTouchHead: true,
		},
		{
			name:              "revision does not match default branch",
			projectKey:        "MYPROJECT",
			repoSlug:          "test-repo",
			revision:          "feature-branch",
			expectedTouchHead: false,
		},
		{
			name:              "unknown repo returns error",
			projectKey:        "MYPROJECT",
			repoSlug:          "unknown-repo",
			revision:          "master",
			expectedErrString: "https://bitbucketserver/rest/api/1.0/projects/MYPROJECT/repos/unknown-repo",
		},
	}
	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			touchedHead, err := isBBServerHeadTouched(client, test.projectKey, test.repoSlug, test.revision)
			if test.expectedErrString == "" {
				require.NoError(t, err)
				require.Equal(t, test.expectedTouchHead, touchedHead)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.expectedErrString)
				require.False(t, touchedHead)
			}
		})
	}
}

func TestRepoURLMatches(t *testing.T) {
	repoRegexp, err := GetWebURLRegex("https://github.com/argoproj/argo-cd")
	require.NoError(t, err)

	tests := []struct {
		name     string
		repoURL  string
		expected bool
	}{
		{
			name:     "exact match",
			repoURL:  "https://github.com/argoproj/argo-cd",
			expected: true,
		},
		{
			name:     "git extension",
			repoURL:  "https://github.com/argoproj/argo-cd.git",
			expected: true,
		},
		{
			name:     "ssh form",
			repoURL:  "git@github.com:argoproj/argo-cd.git",
			expected: true,
		},
		{
			name:     "different repo",
			repoURL:  "https://github.com/argoproj/argo-rollouts",
			expected: false,
		},
		{
			name:     "different host",
			repoURL:  "https://gitlab.com/argoproj/argo-cd",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, RepoURLMatches(tt.repoURL, repoRegexp))
		})
	}
}

const (
	testADORepoURL   = "https://dev.azure.com/my-org/my-project/_git/my-repo"
	testADORepoID    = "33333333-3333-3333-3333-333333333333"
	testADOShaBefore = "fa51eeb1e50b98293ce281e6d5492b9decae613b"
	testADOShaAfter  = "298a79aa1552799a70718a0ee914d153d5a1a76b"
	testADODiffsPath = "/my-org/my-project/_apis/git/repositories/" + testADORepoID + "/diffs/commits"
)

var testADOBasicAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte("test-user:test-pat"))

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newADOWebhookSettings() *settings.ArgoCDSettings {
	return &settings.ArgoCDSettings{
		WebhookAzureDevOpsUsername: "webhook-user",
		WebhookAzureDevOpsPassword: "webhook-pass",
	}
}

func newMockHandlerForADO(t *testing.T, mockDB *mocks.ArgoDB, argoSettings *settings.ArgoCDSettings) *ArgoCDWebhookHandler {
	t.Helper()
	h := newMockHandler(nil, []string{}, int64(50)*1024*1024, mockDB, argoSettings)
	t.Cleanup(h.Shutdown)
	return h
}

func newMockDBWithADORepository() *mocks.ArgoDB {
	mockDB := &mocks.ArgoDB{}
	mockDB.EXPECT().ListRepositories(mock.Anything).Return([]*v1alpha1.Repository{{
		Repo:     testADORepoURL,
		Username: "test-user",
		Password: "test-pat",
	}}, nil)
	return mockDB
}

// newADOTestClient returns an HTTP client that answers every request with the status code and body
// returned by handler.
func newADOTestClient(handler func(req *http.Request) (int, string)) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		statusCode, body := handler(req)
		return &http.Response{
			StatusCode: statusCode,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})}
}

// newADOUnusedClientFactory returns an HTTP client factory that fails the test when called.
func newADOUnusedClientFactory(t *testing.T) func(*v1alpha1.Repository) *http.Client {
	t.Helper()
	return func(repo *v1alpha1.Repository) *http.Client {
		t.Helper()
		t.Fatalf("unexpected Azure DevOps HTTP client for %s", repo.Repo)
		return nil
	}
}

func adoDiffsJSON(t *testing.T, resp adoDiffsResponse) string {
	t.Helper()
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	return string(data)
}

func adoDiffsTestResponse() adoDiffsResponse {
	return adoDiffsResponse{
		AllChangesIncluded: true,
		Changes: []adoDiffChange{
			{Item: adoDiffItem{Path: "/apps/my-app/values.yaml"}, ChangeType: "edit"},
			{Item: adoDiffItem{Path: "/apps/my-app/deployment.yaml"}, ChangeType: "add"},
			{Item: adoDiffItem{Path: "/apps/my-app", IsFolder: true}, ChangeType: "edit"},
		},
	}
}

func requireADODiffsRequest(t *testing.T, req *http.Request) {
	t.Helper()
	require.Equal(t, http.MethodGet, req.Method)
	require.Equal(t, "dev.azure.com", req.URL.Host)
	require.Equal(t, testADODiffsPath, req.URL.Path)
	query := req.URL.Query()
	require.Equal(t, testADOShaBefore, query.Get("baseVersion"))
	require.Equal(t, "commit", query.Get("baseVersionType"))
	require.Equal(t, testADOShaAfter, query.Get("targetVersion"))
	require.Equal(t, "commit", query.Get("targetVersionType"))
	require.Equal(t, "false", query.Get("diffCommonCommit"))
	require.Equal(t, strconv.Itoa(adoDiffsMaxPageSize), query.Get("$top"))
	require.Equal(t, "7.1", query.Get("api-version"))
	require.Equal(t, testADOBasicAuth, req.Header.Get("Authorization"))
}

func Test_affectedRevisionInfo_azuredevops_changed_files(t *testing.T) {
	expectedFiles := []string{"apps/my-app/values.yaml", "apps/my-app/deployment.yaml"}
	credsTemplateDB := func(templateURL string) *mocks.ArgoDB {
		mockDB := &mocks.ArgoDB{}
		mockDB.EXPECT().ListRepositories(mock.Anything).Return([]*v1alpha1.Repository{}, nil)
		mockDB.EXPECT().GetRepositoryCredentials(mock.Anything, mock.Anything).Return(&v1alpha1.RepoCreds{
			URL:      templateURL,
			Username: "test-user",
			Password: "test-pat",
			Proxy:    "http://proxy.example.com:8080",
			NoProxy:  "localhost",
		}, nil)
		return mockDB
	}

	cloneURLRepositoryDB := &mocks.ArgoDB{}
	cloneURLRepositoryDB.EXPECT().ListRepositories(mock.Anything).Return([]*v1alpha1.Repository{{
		Repo:     "https://my-org@dev.azure.com/my-org/my-project/_git/my-repo",
		Username: "test-user",
		Password: "test-pat",
	}}, nil)

	tests := []struct {
		name          string
		settings      *settings.ArgoCDSettings
		mockDB        *mocks.ArgoDB
		updatePayload func(payload *azuredevops.GitPushEvent)
		// expectedFiles is nil when the Diffs API must not be called.
		expectedFiles   []string
		expectedRepoURL string
		expectedProxy   string
	}{
		{
			name:            "repository configured",
			settings:        newADOWebhookSettings(),
			mockDB:          newMockDBWithADORepository(),
			expectedFiles:   expectedFiles,
			expectedRepoURL: testADORepoURL,
		},
		{
			name:            "repository configured with the clone URL",
			settings:        newADOWebhookSettings(),
			mockDB:          cloneURLRepositoryDB,
			expectedFiles:   expectedFiles,
			expectedRepoURL: "https://my-org@dev.azure.com/my-org/my-project/_git/my-repo",
		},
		{
			name:            "credentials template",
			settings:        newADOWebhookSettings(),
			mockDB:          credsTemplateDB("https://dev.azure.com/my-org"),
			expectedFiles:   expectedFiles,
			expectedRepoURL: testADORepoURL,
			expectedProxy:   "http://proxy.example.com:8080",
		},
		{
			name:     "credentials template with the host in another case",
			settings: newADOWebhookSettings(),
			mockDB:   credsTemplateDB("https://dev.azure.com/my-org"),
			updatePayload: func(payload *azuredevops.GitPushEvent) {
				payload.Resource.Repository.RemoteURL = "https://DEV.AZURE.COM/my-org/my-project/_git/my-repo"
			},
			expectedFiles:   expectedFiles,
			expectedRepoURL: testADORepoURL,
			expectedProxy:   "http://proxy.example.com:8080",
		},
		{
			name:     "webhook authentication not configured",
			settings: &settings.ArgoCDSettings{},
			mockDB:   &mocks.ArgoDB{},
		},
		{
			name:     "webhook authentication without password",
			settings: &settings.ArgoCDSettings{WebhookAzureDevOpsUsername: "webhook-user"},
			mockDB:   &mocks.ArgoDB{},
		},
		{
			name:     "multiple ref updates use the first one",
			settings: newADOWebhookSettings(),
			mockDB:   newMockDBWithADORepository(),
			updatePayload: func(payload *azuredevops.GitPushEvent) {
				payload.Resource.RefUpdates = append(payload.Resource.RefUpdates, azuredevops.RefUpdate{
					Name:        "refs/heads/other-branch",
					OldObjectID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					NewObjectID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				})
			},
			expectedFiles:   expectedFiles,
			expectedRepoURL: testADORepoURL,
		},
		{
			name:     "no ref updates",
			settings: newADOWebhookSettings(),
			mockDB:   &mocks.ArgoDB{},
			updatePayload: func(payload *azuredevops.GitPushEvent) {
				payload.Resource.RefUpdates = nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := loadPayload[azuredevops.GitPushEvent](t, "azuredevops-git-push-event.json")
			if tt.updatePayload != nil {
				tt.updatePayload(&payload)
			}
			h := newMockHandlerForADO(t, tt.mockDB, tt.settings)
			// The client is created and called inside sync.OnceValue, where the test must not fail, so
			// the repository and request are recorded and checked afterwards.
			var clientRepo *v1alpha1.Repository
			var apiRequest *http.Request
			body := adoDiffsJSON(t, adoDiffsTestResponse())
			h.adoHTTPClientFactory = func(repo *v1alpha1.Repository) *http.Client {
				clientRepo = repo
				return newADOTestClient(func(req *http.Request) (int, string) {
					apiRequest = req
					return http.StatusOK, body
				})
			}

			_, _, _, _, changedFiles := h.affectedRevisionInfo(t.Context(), payload)
			files := changedFiles()
			if tt.expectedFiles == nil {
				require.Nil(t, files)
				require.Nil(t, clientRepo)
				return
			}
			require.Equal(t, tt.expectedFiles, files)
			require.NotNil(t, clientRepo)
			require.Equal(t, tt.expectedRepoURL, clientRepo.Repo)
			require.Equal(t, tt.expectedProxy, clientRepo.Proxy)
			require.NotNil(t, apiRequest)
			requireADODiffsRequest(t, apiRequest)
			_, hasDeadline := apiRequest.Context().Deadline()
			require.True(t, hasDeadline, "the Azure DevOps diffs API call must have a timeout")
		})
	}
}

func TestLookupAndFetchADOChangedFiles(t *testing.T) {
	noRepositoryDB := &mocks.ArgoDB{}
	noRepositoryDB.EXPECT().ListRepositories(mock.Anything).Return([]*v1alpha1.Repository{}, nil)
	noRepositoryDB.EXPECT().GetRepositoryCredentials(mock.Anything, testADORepoURL).Return(nil, nil)
	lookupErrorDB := &mocks.ArgoDB{}
	lookupErrorDB.EXPECT().ListRepositories(mock.Anything).Return(nil, errors.New("list failed"))
	credsTemplateErrorDB := &mocks.ArgoDB{}
	credsTemplateErrorDB.EXPECT().ListRepositories(mock.Anything).Return([]*v1alpha1.Repository{}, nil)
	credsTemplateErrorDB.EXPECT().GetRepositoryCredentials(mock.Anything, testADORepoURL).Return(nil, errors.New("get failed"))
	noCredentialsDB := &mocks.ArgoDB{}
	noCredentialsDB.EXPECT().ListRepositories(mock.Anything).Return([]*v1alpha1.Repository{{Repo: testADORepoURL}}, nil)
	otherHostCredsTemplateDB := &mocks.ArgoDB{}
	otherHostCredsTemplateDB.EXPECT().ListRepositories(mock.Anything).Return([]*v1alpha1.Repository{}, nil)
	otherHostCredsTemplateDB.EXPECT().GetRepositoryCredentials(mock.Anything, testADORepoURL).Return(&v1alpha1.RepoCreds{
		URL:      "https://ado.example.com",
		Password: "test-pat",
	}, nil)

	tests := []struct {
		name      string
		repoID    string
		shaBefore string
		shaAfter  string
		mockDB    *mocks.ArgoDB
		// transport is nil when no HTTP client must be created.
		transport   roundTripFunc
		expectedLog string
	}{
		{name: "branch creation", shaBefore: adoNullSHA, shaAfter: testADOShaAfter},
		{name: "branch deletion", shaBefore: testADOShaBefore, shaAfter: adoNullSHA},
		{name: "empty commit range"},
		{
			name:        "invalid commit ID before the push",
			shaBefore:   "../../evil",
			shaAfter:    testADOShaAfter,
			expectedLog: "invalid repository ID or commit ID",
		},
		{
			name:        "invalid commit ID after the push",
			shaBefore:   testADOShaBefore,
			shaAfter:    "../../evil",
			expectedLog: "invalid repository ID or commit ID",
		},
		{
			name:        "invalid repository ID",
			repoID:      "../../evil",
			shaBefore:   testADOShaBefore,
			shaAfter:    testADOShaAfter,
			expectedLog: "invalid repository ID or commit ID",
		},
		{
			name:        "repository ID with a prefix",
			repoID:      "x" + testADORepoID,
			shaBefore:   testADOShaBefore,
			shaAfter:    testADOShaAfter,
			expectedLog: "invalid repository ID or commit ID",
		},
		{
			name:        "repository ID with a suffix",
			repoID:      testADORepoID + "/x",
			shaBefore:   testADOShaBefore,
			shaAfter:    testADOShaAfter,
			expectedLog: "invalid repository ID or commit ID",
		},
		{
			name:      "no repository or credentials template configured",
			shaBefore: testADOShaBefore,
			shaAfter:  testADOShaAfter,
			mockDB:    noRepositoryDB,
		},
		{
			name:        "repository lookup error",
			shaBefore:   testADOShaBefore,
			shaAfter:    testADOShaAfter,
			mockDB:      lookupErrorDB,
			expectedLog: "error finding repository for Azure DevOps webhook URL",
		},
		{
			name:        "credentials template for another host",
			shaBefore:   testADOShaBefore,
			shaAfter:    testADOShaAfter,
			mockDB:      otherHostCredsTemplateDB,
			expectedLog: "ignoring credentials template https://ado.example.com",
		},
		{
			name:        "credentials template lookup error",
			shaBefore:   testADOShaBefore,
			shaAfter:    testADOShaAfter,
			mockDB:      credsTemplateErrorDB,
			expectedLog: "error getting repository credentials",
		},
		{
			name:        "no credentials",
			shaBefore:   testADOShaBefore,
			shaAfter:    testADOShaAfter,
			mockDB:      noCredentialsDB,
			expectedLog: "no supported Azure DevOps REST API credentials configured for repository",
		},
		{
			name:      "fetch error",
			shaBefore: testADOShaBefore,
			shaAfter:  testADOShaAfter,
			mockDB:    newMockDBWithADORepository(),
			transport: func(*http.Request) (*http.Response, error) {
				return nil, errors.New("request failed")
			},
			expectedLog: "error fetching changed files from Azure DevOps diffs API",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := test.NewGlobal()
			defer hook.Reset()

			mockDB := tt.mockDB
			if mockDB == nil {
				mockDB = &mocks.ArgoDB{}
			}
			repoID := tt.repoID
			if repoID == "" {
				repoID = testADORepoID
			}
			h := newMockHandlerForADO(t, mockDB, newADOWebhookSettings())
			h.adoHTTPClientFactory = newADOUnusedClientFactory(t)
			if tt.transport != nil {
				h.adoHTTPClientFactory = func(*v1alpha1.Repository) *http.Client {
					return &http.Client{Transport: tt.transport}
				}
			}

			changedFiles := h.lookupAndFetchADOChangedFiles(t.Context(), testADORepoURL, repoID, tt.shaBefore, tt.shaAfter)
			require.Nil(t, changedFiles)
			if tt.expectedLog != "" {
				assertLogContainsSubstr(t, hook, tt.expectedLog)
			}
		})
	}
}

func newADOTestApp(name, refreshPaths string) *v1alpha1.Application {
	app := &v1alpha1.Application{
		Name:      name,
		Namespace: "argocd",
		Spec: v1alpha1.ApplicationSpec{
			Sources: v1alpha1.ApplicationSources{{RepoURL: testADORepoURL, Path: ".", TargetRevision: "HEAD"}},
		},
	}
	if refreshPaths != "" {
		app.Annotations = map[string]string{v1alpha1.AnnotationKeyManifestGeneratePaths: refreshPaths}
	}
	return app
}

func TestHandleEvent_AzureDevOpsChangedFiles(t *testing.T) {
	tests := []struct {
		name              string
		repoURL           string
		apps              []runtime.Object
		apiStatusCode     int
		expectedAPICalls  int32
		expectedRefreshed []string
	}{
		{
			name:              "no application uses manifest-generate-paths",
			apps:              []runtime.Object{newADOTestApp("app-without-paths", "")},
			apiStatusCode:     http.StatusOK,
			expectedAPICalls:  0,
			expectedRefreshed: []string{"app-without-paths"},
		},
		{
			name: "applications use manifest-generate-paths",
			apps: []runtime.Object{
				newADOTestApp("app-with-changed-path", "apps/my-app"),
				newADOTestApp("app-with-unchanged-path", "apps/other-app"),
			},
			apiStatusCode:     http.StatusOK,
			expectedAPICalls:  1,
			expectedRefreshed: []string{"app-with-changed-path"},
		},
		{
			name:    "repository configured with the clone URL",
			repoURL: "https://my-org@dev.azure.com/my-org/my-project/_git/my-repo",
			apps: []runtime.Object{
				newADOTestApp("app-with-changed-path", "apps/my-app"),
				newADOTestApp("app-with-unchanged-path", "apps/other-app"),
			},
			apiStatusCode:     http.StatusOK,
			expectedAPICalls:  1,
			expectedRefreshed: []string{"app-with-changed-path"},
		},
		{
			name: "diffs API error refreshes all applications",
			apps: []runtime.Object{
				newADOTestApp("app-with-changed-path", "apps/my-app"),
				newADOTestApp("app-with-unchanged-path", "apps/other-app"),
			},
			apiStatusCode:     http.StatusInternalServerError,
			expectedAPICalls:  1,
			expectedRefreshed: []string{"app-with-changed-path", "app-with-unchanged-path"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			var refreshed []string
			reaction := func(action kubetesting.Action) (bool, runtime.Object, error) {
				mu.Lock()
				defer mu.Unlock()
				refreshed = append(refreshed, action.(kubetesting.PatchAction).GetName())
				return true, nil, nil
			}
			hook := test.NewGlobal()
			defer hook.Reset()
			repoURL := tt.repoURL
			if repoURL == "" {
				repoURL = testADORepoURL
			}
			mockDB := &mocks.ArgoDB{}
			mockDB.EXPECT().ListRepositories(mock.Anything).Return([]*v1alpha1.Repository{{
				Repo:                             repoURL,
				Username:                         "test-user",
				Password:                         "test-pat",
				WebhookManifestCacheWarmDisabled: true,
			}}, nil)
			h := newMockHandler(&reactorDef{"patch", "applications", reaction}, []string{}, int64(50)*1024*1024, mockDB, newADOWebhookSettings(), tt.apps...)
			var apiCalls atomic.Int32
			body := adoDiffsJSON(t, adoDiffsTestResponse())
			h.adoHTTPClientFactory = func(*v1alpha1.Repository) *http.Client {
				return newADOTestClient(func(*http.Request) (int, string) {
					apiCalls.Add(1)
					return tt.apiStatusCode, body
				})
			}

			h.HandleEvent(t.Context(), loadPayload[azuredevops.GitPushEvent](t, "azuredevops-git-push-event.json"))
			h.Shutdown()

			require.Equal(t, tt.expectedAPICalls, apiCalls.Load())
			require.ElementsMatch(t, tt.expectedRefreshed, refreshed)
			// Manifests of the applications that are not refreshed must not be cached for the new
			// revision, since the repository disables it.
			for _, entry := range hook.AllEntries() {
				require.NotContains(t, entry.Message, "cached manifests", "the manifest cache warming must be disabled")
			}
		})
	}
}

func TestShutdownCancelsAzureDevOpsChangedFilesLookup(t *testing.T) {
	h := newMockHandler(nil, []string{}, int64(50)*1024*1024, newMockDBWithADORepository(), newADOWebhookSettings(), newADOTestApp("app-with-paths", "apps/my-app"))
	started := make(chan struct{})
	var startedOnce sync.Once
	requestErr := make(chan error, 1)
	h.adoHTTPClientFactory = func(*v1alpha1.Repository) *http.Client {
		return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			startedOnce.Do(func() { close(started) })
			<-req.Context().Done()
			requestErr <- req.Context().Err()
			return nil, req.Context().Err()
		})}
	}

	h.queue <- loadPayload[azuredevops.GitPushEvent](t, "azuredevops-git-push-event.json")
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the Azure DevOps diffs API was not called")
	}
	// Shutdown waits for the workers, so it is called asynchronously for the test to fail rather than
	// block when the call is not cancelled.
	shutdownDone := make(chan struct{})
	go func() {
		h.Shutdown()
		close(shutdownDone)
	}()
	select {
	case err := <-requestErr:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("the Azure DevOps diffs API call was not cancelled")
	}
	select {
	case <-shutdownDone:
	case <-time.After(2 * time.Second):
		t.Fatal("the webhook handler did not shut down")
	}
}

func TestLookupRepositoryIgnoringUsername(t *testing.T) {
	const cloneURL = "https://my-org@dev.azure.com/my-org/my-project/_git/my-repo"
	tests := []struct {
		name         string
		repositories []string
		webhookURL   string
		// expectedRepo is empty when no repository must match.
		expectedRepo string
	}{
		{name: "same URL", repositories: []string{testADORepoURL}, webhookURL: testADORepoURL, expectedRepo: testADORepoURL},
		{name: "repository URL with a username", repositories: []string{cloneURL}, webhookURL: testADORepoURL, expectedRepo: cloneURL},
		{name: "repository URL with credentials", repositories: []string{"https://user:secret@dev.azure.com/my-org/my-project/_git/my-repo"}, webhookURL: testADORepoURL, expectedRepo: "https://user:secret@dev.azure.com/my-org/my-project/_git/my-repo"},
		{name: "webhook URL with a username", repositories: []string{testADORepoURL}, webhookURL: cloneURL, expectedRepo: testADORepoURL},
		{name: "exact match takes precedence", repositories: []string{cloneURL, testADORepoURL}, webhookURL: testADORepoURL, expectedRepo: testADORepoURL},
		{name: "SSH URL in both forms", repositories: []string{"ssh://git@ssh.dev.azure.com/v3/my-org/my-project/my-repo"}, webhookURL: "git@ssh.dev.azure.com:v3/my-org/my-project/my-repo", expectedRepo: "ssh://git@ssh.dev.azure.com/v3/my-org/my-project/my-repo"},
		{name: "different repository", repositories: []string{"https://my-org@dev.azure.com/my-org/my-project/_git/other-repo"}, webhookURL: testADORepoURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var repositories []*v1alpha1.Repository
			for _, repoURL := range tt.repositories {
				repositories = append(repositories, &v1alpha1.Repository{Repo: repoURL})
			}
			mockDB := &mocks.ArgoDB{}
			mockDB.EXPECT().ListRepositories(mock.Anything).Return(repositories, nil)
			h := newMockHandlerForADO(t, mockDB, &settings.ArgoCDSettings{})

			repo, err := h.lookupRepositoryIgnoringUsername(t.Context(), tt.webhookURL)
			require.NoError(t, err)
			if tt.expectedRepo == "" {
				require.Nil(t, repo)
				return
			}
			require.NotNil(t, repo)
			require.Equal(t, tt.expectedRepo, repo.Repo)
		})
	}
}

func TestRepoURLOnCredsTemplateHost(t *testing.T) {
	tests := []struct {
		name        string
		templateURL string
		repoURL     string
		// expectedURL is empty when the repository URL does not have the scheme and host of the template.
		expectedURL string
	}{
		{name: "same host", templateURL: "https://dev.azure.com", repoURL: testADORepoURL, expectedURL: testADORepoURL},
		{name: "template with a path", templateURL: "https://dev.azure.com/my-org", repoURL: testADORepoURL, expectedURL: testADORepoURL},
		{name: "repository host in another case", templateURL: "https://dev.azure.com", repoURL: "https://DEV.AZURE.COM/my-org/my-project/_git/my-repo", expectedURL: testADORepoURL},
		{name: "repository host with a Unicode character", templateURL: "https://korp.example.com", repoURL: "https://\u212aorp.example.com/my-org/my-project/_git/my-repo", expectedURL: "https://korp.example.com/my-org/my-project/_git/my-repo"},
		{name: "project with a space", templateURL: "https://dev.azure.com", repoURL: "https://dev.azure.com/my-org/my%20project/_git/my-repo", expectedURL: "https://dev.azure.com/my-org/my%20project/_git/my-repo"},
		{name: "host with the template host as prefix", templateURL: "https://dev.azure.com", repoURL: "https://dev.azure.com.example.com/my-org/my-project/_git/my-repo"},
		{name: "different port", templateURL: "https://dev.azure.com", repoURL: "https://dev.azure.com:8443/my-org/my-project/_git/my-repo"},
		{name: "userinfo in the repository URL", templateURL: "https://dev.azure.com", repoURL: "https://user@dev.azure.com/my-org/my-project/_git/my-repo"},
		{name: "userinfo with the template host", templateURL: "https://dev.azure.com", repoURL: "https://dev.azure.com@example.com/my-org/my-project/_git/my-repo"},
		{name: "different scheme", templateURL: "http://dev.azure.com", repoURL: testADORepoURL},
		{name: "SSH template", templateURL: "git@ssh.dev.azure.com:v3/my-org", repoURL: testADORepoURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoURL, ok := repoURLOnCredsTemplateHost(tt.templateURL, tt.repoURL)
			require.Equal(t, tt.expectedURL != "", ok)
			require.Equal(t, tt.expectedURL, repoURL)
		})
	}
}

func TestParseADOBaseURL(t *testing.T) {
	tests := []struct {
		name        string
		repoURL     string
		expectedURL string
		expectedErr string
	}{
		{
			name:        "dev.azure.com",
			repoURL:     "https://dev.azure.com/my-org/my-project/_git/my-repo",
			expectedURL: "https://dev.azure.com/my-org/my-project",
		},
		{
			name:        "dev.azure.com without project",
			repoURL:     "https://dev.azure.com/my-org/_git/my-repo",
			expectedURL: "https://dev.azure.com/my-org",
		},
		{
			name:        "dev.azure.com with username",
			repoURL:     "https://my-org@dev.azure.com/my-org/my-project/_git/my-repo",
			expectedURL: "https://dev.azure.com/my-org/my-project",
		},
		{
			name:        "project with a space",
			repoURL:     "https://dev.azure.com/my-org/my%20project/_git/my-repo",
			expectedURL: "https://dev.azure.com/my-org/my%20project",
		},
		{
			name:        "visualstudio.com",
			repoURL:     "https://my-org.visualstudio.com/my-project/_git/my-repo",
			expectedURL: "https://my-org.visualstudio.com/my-project",
		},
		{
			name:        "visualstudio.com with collection",
			repoURL:     "https://my-org.visualstudio.com/DefaultCollection/my-project/_git/my-repo",
			expectedURL: "https://my-org.visualstudio.com/DefaultCollection/my-project",
		},
		{
			name:        "Azure DevOps Server",
			repoURL:     "https://ado.example.com/tfs/my-collection/my-project/_git/my-repo",
			expectedURL: "https://ado.example.com/tfs/my-collection/my-project",
		},
		{
			name:        "no _git path segment",
			repoURL:     "https://dev.azure.com/my-org/my-project",
			expectedErr: "no /_git/ path segment",
		},
		{
			name:        "HTTP",
			repoURL:     "http://dev.azure.com/my-org/my-project/_git/my-repo",
			expectedErr: "only https is supported",
		},
		{
			name:        "SSH",
			repoURL:     "ssh://git@ssh.dev.azure.com/v3/my-org/my-project/my-repo",
			expectedErr: "only https is supported",
		},
		{
			name:        "credentials are not included in errors",
			repoURL:     "http://user:secret@dev.azure.com/my-org/my-project/_git/my-repo",
			expectedErr: `"http://dev.azure.com/my-org/my-project/_git/my-repo"`,
		},
		{
			name:        "credentials are not included in errors without the _git path segment",
			repoURL:     "https://user:secret@dev.azure.com/my-org/my-project",
			expectedErr: `"https://dev.azure.com/my-org/my-project"`,
		},
		{
			name:        "credentials are not included in parse errors",
			repoURL:     "https://user:secret@dev.azure.com/my-org/%zz/_git/my-repo",
			expectedErr: "failed to parse Azure DevOps repository URL",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseADOBaseURL(tt.repoURL)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				require.NotContains(t, err.Error(), "secret")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedURL, result)
		})
	}
}

func TestFetchChangedFilesFromADO(t *testing.T) {
	tests := []struct {
		name          string
		repo          *v1alpha1.Repository
		statusCode    int
		body          string
		expectedFiles []string
		expectedErr   string
	}{
		{
			name:          "changed files",
			statusCode:    http.StatusOK,
			body:          adoDiffsJSON(t, adoDiffsTestResponse()),
			expectedFiles: []string{"apps/my-app/values.yaml", "apps/my-app/deployment.yaml"},
		},
		{
			name:       "renamed files",
			statusCode: http.StatusOK,
			body: adoDiffsJSON(t, adoDiffsResponse{
				AllChangesIncluded: true,
				Changes: []adoDiffChange{
					{Item: adoDiffItem{Path: "/apps/new-app/values.yaml"}, ChangeType: "rename", OriginalPath: "/apps/old-app/values.yaml"},
					{Item: adoDiffItem{Path: "/apps/my-app/deployment.yaml"}, ChangeType: "edit, rename", OriginalPath: "/apps/my-app/deployment.yaml"},
					{Item: adoDiffItem{Path: "/apps/new-app", IsFolder: true}, ChangeType: "rename", OriginalPath: "/apps/old-app"},
				},
			}),
			expectedFiles: []string{"apps/new-app/values.yaml", "apps/old-app/values.yaml", "apps/my-app/deployment.yaml"},
		},
		{
			name:       "renamed file without original path",
			statusCode: http.StatusOK,
			body: adoDiffsJSON(t, adoDiffsResponse{
				AllChangesIncluded: true,
				Changes:            []adoDiffChange{{Item: adoDiffItem{Path: "/apps/new-app/values.yaml"}, ChangeType: "Edit, Rename"}},
			}),
			expectedErr: "without original path",
		},
		{
			name:       "change without path",
			statusCode: http.StatusOK,
			body: adoDiffsJSON(t, adoDiffsResponse{
				AllChangesIncluded: true,
				Changes: []adoDiffChange{
					{Item: adoDiffItem{Path: "/apps/my-app/values.yaml"}, ChangeType: "edit"},
					{ChangeType: "edit"},
				},
			}),
			expectedFiles: []string{"apps/my-app/values.yaml"},
		},
		{
			name:        "truncated response",
			statusCode:  http.StatusOK,
			body:        adoDiffsJSON(t, adoDiffsResponse{Changes: adoDiffsTestResponse().Changes}),
			expectedErr: "truncated",
		},
		{
			name:        "unexpected status",
			statusCode:  http.StatusUnauthorized,
			body:        `{"message":"unauthorized"}`,
			expectedErr: "unexpected status 401",
		},
		{
			// Azure DevOps answers requests with an invalid personal access token with a sign-in page.
			name:        "sign-in page",
			statusCode:  http.StatusNonAuthoritativeInfo,
			body:        `<html></html>`,
			expectedErr: "unexpected status 203 from Azure DevOps diffs API, check the repository credentials",
		},
		{
			name:        "malformed response",
			statusCode:  http.StatusOK,
			body:        `{"changes":`,
			expectedErr: "error decoding Azure DevOps diffs API response",
		},
		{
			name:        "response too large",
			statusCode:  http.StatusOK,
			body:        `{"allChangesIncluded":true,"changes":[],"padding":"` + strings.Repeat("a", adoRespMaxSize) + `"}`,
			expectedErr: "error decoding Azure DevOps diffs API response",
		},
		{
			name:        "no credentials",
			repo:        &v1alpha1.Repository{Repo: testADORepoURL, BearerToken: "test-token"},
			expectedErr: errNoADOCredentials.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.repo
			if repo == nil {
				repo = &v1alpha1.Repository{Repo: testADORepoURL, Username: "test-user", Password: "test-pat"}
			}
			newHTTPClient := newADOUnusedClientFactory(t)
			if tt.statusCode != 0 {
				newHTTPClient = func(*v1alpha1.Repository) *http.Client {
					return newADOTestClient(func(req *http.Request) (int, string) {
						requireADODiffsRequest(t, req)
						return tt.statusCode, tt.body
					})
				}
			}

			changedFiles, err := fetchChangedFilesFromADO(t.Context(), newHTTPClient, repo, testADORepoID, testADOShaBefore, testADOShaAfter)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				require.Nil(t, changedFiles)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedFiles, changedFiles)
		})
	}
}

// eofTrackingBody records whether the response body was read to the end, which is required for
// the transport to reuse the connection.
type eofTrackingBody struct {
	reader  io.Reader
	readEOF bool
	closed  bool
}

func (b *eofTrackingBody) Read(p []byte) (int, error) {
	n, err := b.reader.Read(p)
	if errors.Is(err, io.EOF) {
		b.readEOF = true
	}
	return n, err
}

func (b *eofTrackingBody) Close() error {
	b.closed = true
	return nil
}

func TestFetchChangedFilesFromADO_DrainsResponseBody(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		expectedErr bool
	}{
		{name: "successful response", statusCode: http.StatusOK},
		{name: "error response", statusCode: http.StatusUnauthorized, expectedErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Trailing data after the JSON document is left unread by the decoder.
			body := &eofTrackingBody{reader: strings.NewReader(adoDiffsJSON(t, adoDiffsTestResponse()) + "\n\n")}
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tt.statusCode, Header: make(http.Header), Body: body, Request: req}, nil
			})}
			repo := &v1alpha1.Repository{Repo: testADORepoURL, Password: "test-pat"}

			_, err := fetchChangedFilesFromADO(t.Context(), func(*v1alpha1.Repository) *http.Client { return client }, repo, testADORepoID, testADOShaBefore, testADOShaAfter)
			if tt.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.True(t, body.readEOF, "response body should be read to EOF so that the connection can be reused")
			require.True(t, body.closed)
		})
	}
}

func TestSetADOAuthHeader(t *testing.T) {
	tests := []struct {
		name string
		repo *v1alpha1.Repository
		// expectedAuthorization is empty when no credentials must be applied.
		expectedAuthorization string
	}{
		{
			name:                  "basic auth with username and password",
			repo:                  &v1alpha1.Repository{Username: "test-user", Password: "test-pat"},
			expectedAuthorization: testADOBasicAuth,
		},
		{
			name:                  "personal access token without username",
			repo:                  &v1alpha1.Repository{Password: "test-pat"},
			expectedAuthorization: "Basic " + base64.StdEncoding.EncodeToString([]byte(":test-pat")),
		},
		{
			name: "bearer token",
			repo: &v1alpha1.Repository{BearerToken: "test-token"},
		},
		{
			name: "username without password",
			repo: &v1alpha1.Repository{Username: "test-user"},
		},
		{
			name: "no supported credentials",
			repo: &v1alpha1.Repository{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://dev.azure.com/my-org/my-project/_apis", http.NoBody)
			require.NoError(t, err)

			require.Equal(t, tt.expectedAuthorization != "", setADOAuthHeader(req, tt.repo))
			require.Equal(t, tt.expectedAuthorization, req.Header.Get("Authorization"))
		})
	}
}

func TestNewADOHTTPClientUsesRepositoryTransportSettings(t *testing.T) {
	repo := &v1alpha1.Repository{
		Repo:     testADORepoURL,
		Username: "test-user",
		Password: "test-pat",
		Proxy:    "http://proxy.example.com:8080",
		NoProxy:  "localhost",
	}

	client := newADOHTTPClient(repo)
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://dev.azure.com/my-org/my-project/_apis", http.NoBody)
	require.NoError(t, err)
	proxyURL, err := transport.Proxy(req)
	require.NoError(t, err)
	require.Equal(t, "http://proxy.example.com:8080", proxyURL.String())

	noProxyReq, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://localhost/_apis", http.NoBody)
	require.NoError(t, err)
	proxyURL, err = transport.Proxy(noProxyReq)
	require.NoError(t, err)
	require.Nil(t, proxyURL)
}

func requireKeepAlivesEnabled(t *testing.T, client *http.Client, enabled bool) {
	t.Helper()
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.Equal(t, enabled, !transport.DisableKeepAlives)
}

func TestADOHTTPClientCache(t *testing.T) {
	t.Setenv(common.EnvVarTLSDataPath, t.TempDir())

	t.Run("reuses the client for repositories with the same transport settings", func(t *testing.T) {
		c := newADOHTTPClientCache()
		client := c.get(&v1alpha1.Repository{Repo: testADORepoURL, Username: "user-a", Password: "pat-a"})
		other := c.get(&v1alpha1.Repository{Repo: "https://dev.azure.com/my-org/other-project/_git/other-repo", Password: "pat-b"})
		require.Same(t, client, other)
		requireKeepAlivesEnabled(t, client, true)
	})

	t.Run("creates a new client when the transport settings differ", func(t *testing.T) {
		tests := []struct {
			name string
			repo *v1alpha1.Repository
		}{
			{name: "host", repo: &v1alpha1.Repository{Repo: "https://my-org.visualstudio.com/my-project/_git/my-repo", Password: "test-pat"}},
			{name: "insecure", repo: &v1alpha1.Repository{Repo: testADORepoURL, Password: "test-pat", Insecure: true}},
			{name: "proxy", repo: &v1alpha1.Repository{Repo: testADORepoURL, Password: "test-pat", Proxy: "http://proxy.example.com:8080"}},
			{name: "no proxy", repo: &v1alpha1.Repository{Repo: testADORepoURL, Password: "test-pat", NoProxy: "localhost"}},
			{name: "client certificate", repo: &v1alpha1.Repository{Repo: testADORepoURL, Password: "test-pat", TLSClientCertData: "cert"}},
			{name: "client certificate key", repo: &v1alpha1.Repository{Repo: testADORepoURL, Password: "test-pat", TLSClientCertKey: "key"}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				c := newADOHTTPClientCache()
				client := c.get(&v1alpha1.Repository{Repo: testADORepoURL, Password: "test-pat"})
				require.NotSame(t, client, c.get(tt.repo))
			})
		}
	})

	t.Run("does not mix up settings with the same value", func(t *testing.T) {
		c := newADOHTTPClientCache()
		client := c.get(&v1alpha1.Repository{Repo: testADORepoURL, Password: "test-pat", Proxy: "localhost"})
		require.NotSame(t, client, c.get(&v1alpha1.Repository{Repo: testADORepoURL, Password: "test-pat", NoProxy: "localhost"}))
	})

	t.Run("creates a new client when the server certificates change", func(t *testing.T) {
		tlsDataPath := t.TempDir()
		t.Setenv(common.EnvVarTLSDataPath, tlsDataPath)
		repo := &v1alpha1.Repository{Repo: testADORepoURL, Password: "test-pat"}
		c := newADOHTTPClientCache()
		client := c.get(repo)

		certPEM := "-----BEGIN CERTIFICATE-----\nZmFrZQ==\n-----END CERTIFICATE-----\n"
		require.NoError(t, os.WriteFile(filepath.Join(tlsDataPath, "dev.azure.com"), []byte(certPEM), 0o600))
		require.NotSame(t, client, c.get(repo))
	})

	t.Run("does not cache clients beyond the limit", func(t *testing.T) {
		c := newADOHTTPClientCache()
		for i := range adoHTTPClientCacheLimit {
			c.get(&v1alpha1.Repository{Repo: testADORepoURL, Password: "test-pat", Proxy: "http://proxy" + strconv.Itoa(i) + ".example.com:8080"})
		}
		repo := &v1alpha1.Repository{Repo: testADORepoURL, Password: "test-pat"}
		client := c.get(repo)
		require.NotSame(t, client, c.get(repo))
		require.Len(t, c.clients, adoHTTPClientCacheLimit)
		requireKeepAlivesEnabled(t, client, false)
	})
}
