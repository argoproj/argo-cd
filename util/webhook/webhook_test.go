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

func NewMockHandler(reactor *reactorDef, applicationNamespaces []string, objects ...runtime.Object) *ArgoCDWebhookHandler {
	defaultMaxPayloadSize := int64(50) * 1024 * 1024
	return NewMockHandlerWithPayloadLimit(reactor, applicationNamespaces, defaultMaxPayloadSize, objects...)
}

func NewMockHandlerWithPayloadLimit(reactor *reactorDef, applicationNamespaces []string, maxPayloadSize int64, objects ...runtime.Object) *ArgoCDWebhookHandler {
	return newMockHandler(reactor, applicationNamespaces, maxPayloadSize, &mocks.ArgoDB{}, &settings.ArgoCDSettings{}, objects...)
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
	return NewHandler("argocd", applicationNamespaces, 10, appClientset, &fakeAppsLister{clientset: appClientset}, argoSettings, &fakeSettingsSrc{}, cache.NewCache(
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
	var patchData []byte
	reaction := func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patchAction := action.(kubetesting.PatchAction)
		assert.Equal(t, "app-to-refresh", patchAction.GetName())
		patchData = patchAction.GetPatch()
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

	// Verify that only refresh annotation is set, NOT hydrate annotation for sources changes
	var patchMap map[string]any
	err = json.Unmarshal(patchData, &patchMap)
	require.NoError(t, err)
	metadata := patchMap["metadata"].(map[string]any)
	annotations := metadata["annotations"].(map[string]any)
	assert.Equal(t, "normal", annotations["argocd.argoproj.io/refresh"])
	_, hasHydrate := annotations["argocd.argoproj.io/hydrate"]
	assert.False(t, hasHydrate, "sources changes should NOT trigger hydration")
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

// TestGitHubCommitEvent_Hydrate_DrySource tests that a webhook will refresh and hydrate an app when dry source changed.
func TestGitHubCommitEvent_Hydrate_DrySource(t *testing.T) {
	var patchData []byte
	reaction := func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patchAction := action.(kubetesting.PatchAction)
		assert.Equal(t, "app-to-hydrate", patchAction.GetName())
		patchData = patchAction.GetPatch()
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

	// Verify that both refresh and hydrate annotations are set for drySource changes
	var patchMap map[string]any
	err = json.Unmarshal(patchData, &patchMap)
	require.NoError(t, err)
	metadata := patchMap["metadata"].(map[string]any)
	annotations := metadata["annotations"].(map[string]any)
	assert.Equal(t, "normal", annotations["argocd.argoproj.io/refresh"])
	assert.Equal(t, "normal", annotations["argocd.argoproj.io/hydrate"])
}

// TestGitHubCommitEvent_SyncSourceRefresh tests that syncSource changes trigger refresh WITHOUT hydration.
func TestGitHubCommitEvent_SyncSourceRefresh(t *testing.T) {
	var patchData []byte
	reaction := func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patchAction := action.(kubetesting.PatchAction)
		assert.Equal(t, "app-to-refresh", patchAction.GetName())
		patchData = patchAction.GetPatch()
		return true, nil, nil
	}
	h := NewMockHandler(&reactorDef{"patch", "applications", reaction}, []string{}, &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-to-refresh",
			Namespace: "argocd",
			Annotations: map[string]string{
				"argocd.argoproj.io/manifest-generate-paths": ".",
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			SourceHydrator: &v1alpha1.SourceHydrator{
				DrySource: v1alpha1.DrySource{
					RepoURL:        "https://github.com/jessesuen/test-repo",
					TargetRevision: "main",
					Path:           ".",
				},
				SyncSource: v1alpha1.SyncSource{
					TargetBranch: "environments/dev",
					Path:         ".",
				},
				HydrateTo: nil,
			},
		},
	},
	)
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := os.ReadFile("testdata/github-commit-syncsource-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify that only refresh annotation is set, NOT hydrate annotation for syncSource changes
	var patchMap map[string]any
	err = json.Unmarshal(patchData, &patchMap)
	require.NoError(t, err)
	metadata := patchMap["metadata"].(map[string]any)
	annotations := metadata["annotations"].(map[string]any)
	assert.Equal(t, "normal", annotations["argocd.argoproj.io/refresh"])
	_, hasHydrate := annotations["argocd.argoproj.io/hydrate"]
	assert.False(t, hasHydrate, "syncSource changes should NOT trigger hydration")
}

// TestGitHubCommitEvent_SyncSourceRefresh_FileFiltering tests that syncSource webhooks
// filter out irrelevant file changes based on the syncSource path
func TestGitHubCommitEvent_SyncSourceRefresh_FileFiltering(t *testing.T) {
	// The test payload (github-commit-syncsource-event.json) changes files under:
	// - ksapps/test-app/environments/staging-argocd-demo/main.jsonnet
	// - ksapps/test-app/environments/staging-argocd-demo/params.libsonnet
	// - ksapps/test-app/app.yaml

	tests := []struct {
		name            string
		syncSourcePath  string
		expectedRefresh bool
	}{
		{
			name:            "syncSource path matches changed files - should refresh",
			syncSourcePath:  "ksapps",
			expectedRefresh: true,
		},
		{
			name:            "syncSource path does not match changed files - should not refresh",
			syncSourcePath:  "helm-charts",
			expectedRefresh: false,
		},
		{
			name:            "syncSource at root path - should refresh",
			syncSourcePath:  ".",
			expectedRefresh: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var patchData []byte
			reaction := func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
				if action.GetVerb() == "patch" {
					patchAction := action.(kubetesting.PatchAction)
					patchData = patchAction.GetPatch()
				}
				return true, nil, nil
			}

			h := NewMockHandler(&reactorDef{"patch", "applications", reaction}, []string{}, &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app-to-test",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSpec{
					SourceHydrator: &v1alpha1.SourceHydrator{
						DrySource: v1alpha1.DrySource{
							RepoURL:        testRepoURL,
							TargetRevision: "main",
							Path:           ".",
						},
						SyncSource: v1alpha1.SyncSource{
							TargetBranch: "environments/dev",
							Path:         tt.syncSourcePath,
						},
						HydrateTo: nil,
					},
				},
			})
			w := executeWebhook(t, h, "testdata/github-commit-syncsource-event.json")
			assert.Equal(t, http.StatusOK, w.Code)

			// Verify that sync source changes trigger refresh but not hydration
			verifyAnnotations(t, patchData, tt.expectedRefresh, false)
		})
	}
}

// TestGitHubCommitEvent_Hydration_DrySource_WithAnnotation tests that drySource webhooks
// with manifest-generate-paths annotation filter files based on annotation paths
func TestGitHubCommitEvent_Hydration_DrySource_WithAnnotation(t *testing.T) {
	hook := test.NewGlobal()
	var patched bool
	reaction := func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patchAction := action.(kubetesting.PatchAction)
		assert.Equal(t, "app-with-annotation", patchAction.GetName())
		patched = true
		return true, nil, nil
	}

	// The test payload changes files under ksapps/ directory
	tests := []struct {
		name            string
		annotation      string
		drySourcePath   string
		expectedRefresh bool
	}{
		{
			name:            "annotation matches changed files - should hydrate",
			annotation:      ".",
			drySourcePath:   "ksapps",
			expectedRefresh: true,
		},
		{
			name:            "annotation does not match changed files - should not hydrate",
			annotation:      ".",
			drySourcePath:   "helm-charts",
			expectedRefresh: false,
		},
		{
			name:            "annotation with relative path matches - should hydrate",
			annotation:      ".;../shared",
			drySourcePath:   "ksapps",
			expectedRefresh: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patched = false
			h := NewMockHandler(&reactorDef{"patch", "applications", reaction}, []string{}, &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app-with-annotation",
					Namespace: "argocd",
					Annotations: map[string]string{
						"argocd.argoproj.io/manifest-generate-paths": tt.annotation,
					},
				},
				Spec: v1alpha1.ApplicationSpec{
					SourceHydrator: &v1alpha1.SourceHydrator{
						DrySource: v1alpha1.DrySource{
							RepoURL:        "https://github.com/jessesuen/test-repo",
							TargetRevision: "HEAD", // Matches master branch from webhook event
							Path:           tt.drySourcePath,
						},
						SyncSource: v1alpha1.SyncSource{
							TargetBranch: "environments/dev",
							Path:         "hydrated",
						},
					},
				},
			})
			req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
			req.Header.Set("X-GitHub-Event", "push")
			// Use main branch event for drySource testing (matches drySource.TargetRevision)
			eventJSON, err := os.ReadFile("testdata/github-commit-event.json")
			require.NoError(t, err)
			req.Body = io.NopCloser(bytes.NewReader(eventJSON))
			w := httptest.NewRecorder()
			h.Handler(w, req)
			close(h.queue)
			h.Wait()
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.expectedRefresh, patched)

			logMessages := make([]string, 0, len(hook.Entries))
			for _, entry := range hook.Entries {
				logMessages = append(logMessages, entry.Message)
			}

			if tt.expectedRefresh {
				assert.Contains(t, logMessages, "webhook trigger refresh app to hydrate 'app-with-annotation'")
			} else {
				assert.NotContains(t, logMessages, "webhook trigger refresh app to hydrate 'app-with-annotation'")
			}
			hook.Reset()
		})
	}
}

// TestGitHubCommitEvent_Hydration_DrySource_NoAnnotation tests that drySource webhooks
// without manifest-generate-paths annotation use the entire drySource path as default
func TestGitHubCommitEvent_Hydration_DrySource_NoAnnotation(t *testing.T) {
	hook := test.NewGlobal()
	var patched bool
	reaction := func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patchAction := action.(kubetesting.PatchAction)
		assert.Equal(t, "app-no-annotation", patchAction.GetName())
		patched = true
		return true, nil, nil
	}

	// The test payload changes files under ksapps/ directory
	tests := []struct {
		name            string
		drySourcePath   string
		expectedRefresh bool
	}{
		{
			name:            "drySource path matches changed files - should hydrate",
			drySourcePath:   "ksapps",
			expectedRefresh: true,
		},
		{
			name:            "drySource path does not match changed files - should not hydrate",
			drySourcePath:   "helm-charts",
			expectedRefresh: false,
		},
		{
			name:            "drySource at root path - should hydrate",
			drySourcePath:   ".",
			expectedRefresh: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patched = false
			h := NewMockHandler(&reactorDef{"patch", "applications", reaction}, []string{}, &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app-no-annotation",
					Namespace: "argocd",
					// No manifest-generate-paths annotation
				},
				Spec: v1alpha1.ApplicationSpec{
					SourceHydrator: &v1alpha1.SourceHydrator{
						DrySource: v1alpha1.DrySource{
							RepoURL:        "https://github.com/jessesuen/test-repo",
							TargetRevision: "HEAD", // Matches master branch from webhook event
							Path:           tt.drySourcePath,
						},
						SyncSource: v1alpha1.SyncSource{
							TargetBranch: "environments/dev",
							Path:         "hydrated",
						},
					},
				},
			})
			req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
			req.Header.Set("X-GitHub-Event", "push")
			// Use main branch event for drySource testing (matches drySource.TargetRevision)
			eventJSON, err := os.ReadFile("testdata/github-commit-event.json")
			require.NoError(t, err)
			req.Body = io.NopCloser(bytes.NewReader(eventJSON))
			w := httptest.NewRecorder()
			h.Handler(w, req)
			close(h.queue)
			h.Wait()
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.expectedRefresh, patched)

			logMessages := make([]string, 0, len(hook.Entries))
			for _, entry := range hook.Entries {
				logMessages = append(logMessages, entry.Message)
			}

			if tt.expectedRefresh {
				assert.Contains(t, logMessages, "webhook trigger refresh app to hydrate 'app-no-annotation'")
			} else {
				assert.NotContains(t, logMessages, "webhook trigger refresh app to hydrate 'app-no-annotation'")
			}
			hook.Reset()
		})
	}
}

// TestGitHubCommitEvent_Standard_WithAnnotation tests that standard apps (no hydration)
// with manifest-generate-paths annotation only refresh when changed files match annotation paths
func TestGitHubCommitEvent_Standard_WithAnnotation(t *testing.T) {
	hook := test.NewGlobal()
	var patched bool
	reaction := func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patchAction := action.(kubetesting.PatchAction)
		assert.Equal(t, "standard-app-with-annotation", patchAction.GetName())
		patched = true
		return true, nil, nil
	}

	// The test payload changes files under ksapps/ directory
	tests := []struct {
		name            string
		annotation      string
		sourcePath      string
		expectedRefresh bool
	}{
		{
			name:            "annotation matches changed files - should refresh",
			annotation:      ".",
			sourcePath:      "ksapps",
			expectedRefresh: true,
		},
		{
			name:            "annotation does not match changed files - should not refresh",
			annotation:      ".",
			sourcePath:      "helm-charts",
			expectedRefresh: false,
		},
		{
			name:            "annotation with multiple paths, one matches - should refresh",
			annotation:      ".;../other",
			sourcePath:      "ksapps",
			expectedRefresh: true,
		},
		{
			name:            "annotation at root matches all - should refresh",
			annotation:      ".",
			sourcePath:      ".",
			expectedRefresh: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patched = false
			h := NewMockHandler(&reactorDef{"patch", "applications", reaction}, []string{}, &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "standard-app-with-annotation",
					Namespace: "argocd",
					Annotations: map[string]string{
						"argocd.argoproj.io/manifest-generate-paths": tt.annotation,
					},
				},
				Spec: v1alpha1.ApplicationSpec{
					Sources: v1alpha1.ApplicationSources{
						{
							RepoURL:        "https://github.com/jessesuen/test-repo",
							Path:           tt.sourcePath,
							TargetRevision: "HEAD", // Matches the master branch from the webhook event
						},
					},
				},
			})
			req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
			req.Header.Set("X-GitHub-Event", "push")
			// Use main branch event for standard app testing (matches source targetRevision: HEAD/master)
			eventJSON, err := os.ReadFile("testdata/github-commit-event.json")
			require.NoError(t, err)
			req.Body = io.NopCloser(bytes.NewReader(eventJSON))
			w := httptest.NewRecorder()
			h.Handler(w, req)
			close(h.queue)
			h.Wait()
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.expectedRefresh, patched)

			logMessages := make([]string, 0, len(hook.Entries))
			for _, entry := range hook.Entries {
				logMessages = append(logMessages, entry.Message)
			}

			if tt.expectedRefresh {
				assert.Contains(t, logMessages, "Requested app 'standard-app-with-annotation' refresh")
			} else {
				assert.NotContains(t, logMessages, "Requested app 'standard-app-with-annotation' refresh")
			}
			hook.Reset()
		})
	}
}

// TestGitHubCommitEvent_Standard_NoAnnotation tests that standard apps (no hydration)
// without manifest-generate-paths annotation always refresh on any change (default behavior)
func TestGitHubCommitEvent_Standard_NoAnnotation(t *testing.T) {
	hook := test.NewGlobal()
	var patched bool
	reaction := func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patchAction := action.(kubetesting.PatchAction)
		assert.Equal(t, "standard-app-no-annotation", patchAction.GetName())
		patched = true
		return true, nil, nil
	}

	// Test that regardless of which files change or which path is configured,
	// the app always refreshes when no annotation is present (default behavior)
	tests := []struct {
		name       string
		sourcePath string
	}{
		{
			name:       "source path matches changed files - should refresh",
			sourcePath: "ksapps",
		},
		{
			name:       "source path does not match changed files - should still refresh",
			sourcePath: "helm-charts",
		},
		{
			name:       "source at root - should refresh",
			sourcePath: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patched = false
			h := NewMockHandler(&reactorDef{"patch", "applications", reaction}, []string{}, &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "standard-app-no-annotation",
					Namespace: "argocd",
					// No manifest-generate-paths annotation
				},
				Spec: v1alpha1.ApplicationSpec{
					Sources: v1alpha1.ApplicationSources{
						{
							RepoURL:        "https://github.com/jessesuen/test-repo",
							Path:           tt.sourcePath,
							TargetRevision: "HEAD", // Matches the master branch from the webhook event
						},
					},
				},
			})
			req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
			req.Header.Set("X-GitHub-Event", "push")
			// Use main branch event for standard app testing (matches source targetRevision: HEAD/master)
			eventJSON, err := os.ReadFile("testdata/github-commit-event.json")
			require.NoError(t, err)
			req.Body = io.NopCloser(bytes.NewReader(eventJSON))
			w := httptest.NewRecorder()
			h.Handler(w, req)
			close(h.queue)
			h.Wait()
			assert.Equal(t, http.StatusOK, w.Code)
			// Should always refresh regardless of path
			assert.True(t, patched, "expected app to refresh but it didn't")

			logMessages := make([]string, 0, len(hook.Entries))
			for _, entry := range hook.Entries {
				logMessages = append(logMessages, entry.Message)
			}

			assert.Contains(t, logMessages, "Requested app 'standard-app-no-annotation' refresh")
			hook.Reset()
		})
	}
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
		err := json.Unmarshal([]byte(fmt.Sprintf(`{"push":{"changes":[{"new":{"name":%q}}]}}`, branchName)), &pl)
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

// TestGitHubCommitEvent_NoChanges_CacheNotUpdated_MissingSHAs tests that cache
// is NOT updated when commit SHAs are missing from the webhook payload
func TestGitHubCommitEvent_NoChanges_CacheNotUpdated_MissingSHAs(t *testing.T) {
	tests := []struct {
		name               string
		mockPayload        func() []byte
		expectedLogContain string
	}{
		{
			name: "missing before SHA",
			mockPayload: func() []byte {
				// Create a payload with empty "before" field
				payload := `{
					"ref": "refs/heads/master",
					"before": "",
					"after": "63738bb582c8b540af7bcfc18f87c575c3ed66e0",
					"repository": {
						"html_url": "https://github.com/jessesuen/test-repo",
						"default_branch": "master"
					},
					"commits": [
						{
							"added": ["helm-charts/values.yaml"],
							"modified": [],
							"removed": []
						}
					]
				}`
				return []byte(payload)
			},
		},
		{
			name: "missing after SHA",
			mockPayload: func() []byte {
				payload := `{
					"ref": "refs/heads/master",
					"before": "d5c1ffa8e294bc18c639bfb4e0df499251034414",
					"after": "",
					"repository": {
						"html_url": "https://github.com/jessesuen/test-repo",
						"default_branch": "master"
					},
					"commits": [
						{
							"added": ["helm-charts/values.yaml"],
							"modified": [],
							"removed": []
						}
					]
				}`
				return []byte(payload)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write payload to a temporary file
			tmpFile, err := os.CreateTemp(t.TempDir(), "webhook-test-*.json")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())

			_, err = tmpFile.Write(tt.mockPayload())
			require.NoError(t, err)
			require.NoError(t, tmpFile.Close())

			inMemoryCache := cacheutil.NewInMemoryCache(1 * time.Hour)
			cacheClient := cacheutil.NewCache(inMemoryCache)

			repoCache := cache.NewCache(
				cacheClient,
				1*time.Minute,
				1*time.Minute,
				10*time.Second,
			)

			app := &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "argocd",
					Annotations: map[string]string{
						"argocd.argoproj.io/manifest-generate-paths": "helm-charts", // Different from webhook's ksapps changes
					},
				},
				Spec: v1alpha1.ApplicationSpec{
					Destination: v1alpha1.ApplicationDestination{
						Server:    testClusterURL,
						Namespace: "",
					},
					Sources: v1alpha1.ApplicationSources{
						{
							RepoURL:        testRepoURL,
							Path:           "helm-charts", // Different from webhook's ksapps changes
							TargetRevision: "HEAD",        // Tracks master (webhook branch)
						},
					},
				},
			}

			appClientset := appclientset.NewSimpleClientset(app)
			mockDB := &mocks.ArgoDB{}
			serverCache := servercache.NewCache(appstate.NewCache(cacheClient, time.Minute), time.Minute, time.Minute)

			h := NewHandler(
				"argocd",
				[]string{},
				10,
				appClientset,
				&fakeAppsLister{clientset: appClientset},
				&settings.ArgoCDSettings{},
				&fakeSettingsSrc{},
				repoCache,
				serverCache,
				mockDB,
				int64(50)*1024*1024,
			)

			w := executeWebhook(t, h, tmpFile.Name())
			assert.Equal(t, http.StatusOK, w.Code)

			// Verify cache was not updated
			items, err := inMemoryCache.Items(func() any { return &cache.CachedManifestResponse{} })
			if err != nil {
				t.Fatalf("failed to get items: %v", err)
			}
			require.NoError(t, err)
			assert.Empty(t, items, "cache should be empty when commit SHAs are missing")
		})
	}
}

// TestGitHubCommitEvent_CacheUpdated_FilesOutsideRefreshPaths tests that the cache IS updated
// when files are changed outside the refresh paths (for both regular sources and hydrator sources).
// This consolidates tests for cache updates when app tracks revision but files don't match refresh paths.
func TestGitHubCommitEvent_CacheUpdated_FilesOutsideRefreshPaths(t *testing.T) {
	tests := []struct {
		name           string
		appPath        string
		annotation     string
		sourceHydrator *v1alpha1.SourceHydrator
		description    string
	}{
		{
			name:           "app tracks revision but files in different path",
			appPath:        "helm-charts",
			annotation:     "helm-charts", // App cares about helm-charts, but webhook changes ksapps
			sourceHydrator: nil,
			description:    "app tracks HEAD (master) but files don't match annotation paths",
		},
		{
			name:           "sync source - files outside refresh paths",
			appPath:        "helm-charts",
			annotation:     "specific-dir", // Files changed in ksapps, not specific-dir
			sourceHydrator: nil,
			description:    "regular source with files changed outside annotation paths",
		},
		{
			name:       "dry source - files outside refresh paths",
			appPath:    ".",
			annotation: "specific-dir", // Files changed in ksapps, not specific-dir
			sourceHydrator: &v1alpha1.SourceHydrator{
				DrySource: v1alpha1.DrySource{
					RepoURL:        testRepoURL,
					TargetRevision: "HEAD",
					Path:           ".",
				},
				SyncSource: v1alpha1.SyncSource{
					TargetBranch: "environments/dev",
					Path:         "hydrated",
				},
			},
			description: "dry source with files changed outside annotation paths",
		},
		{
			name:       "sync source (hydrator) - files outside refresh paths for sync source",
			appPath:    ".",
			annotation: "dry-specific-dir", // Only dry source files
			sourceHydrator: &v1alpha1.SourceHydrator{
				DrySource: v1alpha1.DrySource{
					RepoURL:        testRepoURL,
					TargetRevision: "HEAD",
					Path:           ".",
				},
				SyncSource: v1alpha1.SyncSource{
					TargetBranch: "environments/dev", // Different branch from webhook (master)
					Path:         ".",
				},
			},
			description: "sync source in hydrator where files changed outside dry source annotation paths",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inMemoryCache := cacheutil.NewInMemoryCache(1 * time.Hour)
			cacheClient := cacheutil.NewCache(inMemoryCache)

			repoCache := cache.NewCache(
				cacheClient,
				1*time.Minute,
				1*time.Minute,
				10*time.Second,
			)

			app := &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "argocd",
					Annotations: map[string]string{
						"argocd.argoproj.io/manifest-generate-paths": tt.annotation,
					},
				},
				Spec: v1alpha1.ApplicationSpec{
					Destination: v1alpha1.ApplicationDestination{
						Server:    testClusterURL,
						Namespace: "",
					},
					Sources: v1alpha1.ApplicationSources{
						{
							RepoURL:        testRepoURL,
							Path:           tt.appPath,
							TargetRevision: "HEAD",
						},
					},
					SourceHydrator: tt.sourceHydrator,
				},
			}

			// Pre-populate cache with manifests for the "before" SHA to simulate existing cache
			testSource := &v1alpha1.ApplicationSource{
				RepoURL:        testRepoURL,
				Path:           tt.appPath,
				TargetRevision: "HEAD",
			}
			clusterInfo := &mockClusterInfo{}
			testManifests := []string{"apiVersion: v1\nkind: Service\nmetadata:\n  name: test-service"}
			setupTestCache(t, repoCache, "test-app", testSource, testManifests)

			appClientset := appclientset.NewSimpleClientset(app)
			mockDB := &mocks.ArgoDB{}
			mockDB.EXPECT().GetCluster(mock.Anything, testClusterURL).Return(&v1alpha1.Cluster{
				Server: testClusterURL,
				Info: v1alpha1.ClusterInfo{
					ServerVersion:   "1.28.0",
					ConnectionState: v1alpha1.ConnectionState{Status: v1alpha1.ConnectionStatusSuccessful},
					APIVersions:     []string{},
				},
			}, nil).Maybe()
			serverCache := servercache.NewCache(appstate.NewCache(cacheClient, time.Minute), time.Minute, time.Minute)
			err := serverCache.SetClusterInfo(testClusterURL, &v1alpha1.ClusterInfo{
				ServerVersion:   "1.28.0",
				ConnectionState: v1alpha1.ConnectionState{Status: v1alpha1.ConnectionStatusSuccessful},
				APIVersions:     []string{},
			})
			require.NoError(t, err)

			h := NewHandler(
				"argocd",
				[]string{},
				10,
				appClientset,
				&fakeAppsLister{clientset: appClientset},
				&settings.ArgoCDSettings{},
				&fakeSettingsSrc{},
				repoCache,
				serverCache,
				mockDB,
				int64(50)*1024*1024,
			)

			w := executeWebhook(t, h, testEventDataDir)
			assert.Equal(t, http.StatusOK, w.Code)

			// Verify cache WAS updated - beforeSHA should no longer exist
			var beforeManifests cache.CachedManifestResponse
			err = repoCache.GetManifests(testBeforeSHA, testSource, nil, clusterInfo, "", "", testAppLabelKey, "test-app", &beforeManifests, nil, "")
			require.Error(t, err, "beforeSHA should no longer exist in cache after being moved: %s", tt.description)

			// Verify cache WAS updated - should have entry for afterSHA
			var afterManifests cache.CachedManifestResponse
			err = repoCache.GetManifests(testAfterSHA, testSource, nil, clusterInfo, "", "", testAppLabelKey, "test-app", &afterManifests, nil, "")
			require.NoError(t, err, "cache should be updated with afterSHA when files are outside refresh paths: %s", tt.description)
			if err == nil {
				assert.Equal(t, testAfterSHA, afterManifests.ManifestResponse.Revision, "cached revision should match afterSHA")
				// Verify the manifests were preserved from beforeSHA
				assert.Equal(t, testManifests, afterManifests.ManifestResponse.Manifests, "manifests should be preserved from beforeSHA")
			}
		})
	}
}

// TestGitHubCommitEvent_Annotations_AppliedCorrectly tests that refresh and hydrate
// annotations are applied correctly based on file changes
func TestGitHubCommitEvent_Annotations_AppliedCorrectly(t *testing.T) {
	tests := []struct {
		name            string
		appPath         string
		annotation      string
		sourceHydrator  *v1alpha1.SourceHydrator
		expectedRefresh bool
		expectedHydrate bool
		expectedLogMsg  string
	}{
		{
			name:            "files match - refresh annotation applied",
			appPath:         "ksapps",
			annotation:      ".",
			sourceHydrator:  nil,
			expectedRefresh: true,
			expectedHydrate: false,
			expectedLogMsg:  "Requested app 'test-app' refresh",
		},
		{
			name:            "files don't match - no annotations",
			appPath:         "helm-charts",
			annotation:      ".",
			sourceHydrator:  nil,
			expectedRefresh: false,
			expectedHydrate: false,
			expectedLogMsg:  "",
		},
		{
			name:       "hydrator dry source matches - both annotations applied",
			appPath:    ".",
			annotation: ".",
			sourceHydrator: &v1alpha1.SourceHydrator{
				DrySource: v1alpha1.DrySource{
					RepoURL:        "https://github.com/jessesuen/test-repo",
					TargetRevision: "HEAD",
					Path:           "ksapps",
				},
				SyncSource: v1alpha1.SyncSource{
					TargetBranch: "environments/dev",
					Path:         "hydrated",
				},
			},
			expectedRefresh: true,
			expectedHydrate: true,
			expectedLogMsg:  "webhook trigger refresh app to hydrate 'test-app'",
		},
		{
			name:       "hydrator sync source matches - only refresh annotation",
			appPath:    ".",
			annotation: ".",
			sourceHydrator: &v1alpha1.SourceHydrator{
				DrySource: v1alpha1.DrySource{
					RepoURL:        "https://github.com/jessesuen/test-repo",
					TargetRevision: "main",
					Path:           "other-path",
				},
				SyncSource: v1alpha1.SyncSource{
					TargetBranch: "master", // Matches webhook branch
					Path:         "ksapps",
				},
			},
			expectedRefresh: true,
			expectedHydrate: false,
			expectedLogMsg:  "refreshing app 'test-app' from webhook",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := test.NewGlobal()
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

			appAnnotations := map[string]string{}
			if tt.annotation != "" {
				appAnnotations["argocd.argoproj.io/manifest-generate-paths"] = tt.annotation
			}

			app := &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-app",
					Namespace:   "argocd",
					Annotations: appAnnotations,
				},
				Spec: v1alpha1.ApplicationSpec{
					SourceHydrator: tt.sourceHydrator,
				},
			}

			if tt.sourceHydrator == nil {
				app.Spec.Sources = v1alpha1.ApplicationSources{
					{
						RepoURL:        "https://github.com/jessesuen/test-repo",
						Path:           tt.appPath,
						TargetRevision: "HEAD",
					},
				}
			}

			h := NewMockHandler(&reactorDef{"patch", "applications", reaction}, []string{}, app)
			w := executeWebhook(t, h, testEventDataDir)
			assert.Equal(t, http.StatusOK, w.Code)

			// Check if app was patched
			assert.Equal(t, tt.expectedRefresh, patched, "patch status mismatch")

			// Verify annotations using helper
			verifyAnnotations(t, patchData, tt.expectedRefresh, tt.expectedHydrate)

			// Check logs
			if tt.expectedLogMsg != "" {
				logMessages := make([]string, 0, len(hook.Entries))
				for _, entry := range hook.Entries {
					logMessages = append(logMessages, entry.Message)
				}
				assert.Contains(t, logMessages, tt.expectedLogMsg)
			}

			hook.Reset()
		})
	}
}

// TestGitHubCommitEvent_CacheNotUpdated_WhenAppDoesNotTrackRevision tests that cache is NOT updated
// when an app tracks a different branch than the one in the webhook event
func TestGitHubCommitEvent_CacheNotUpdated_WhenAppDoesNotTrackRevision(t *testing.T) {
	hook := test.NewGlobal()

	var patched bool
	reaction := func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetVerb() == "patch" {
			patched = true
		}
		return false, nil, nil
	}

	// Test app watches "dev" branch, but webhook is for "master" branch
	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-different-branch",
			Namespace: "argocd",
			Annotations: map[string]string{
				"argocd.argoproj.io/manifest-generate-paths": ".",
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			Destination: v1alpha1.ApplicationDestination{
				Server:    testClusterURL,
				Namespace: "",
			},
			Sources: v1alpha1.ApplicationSources{
				{
					RepoURL:        testRepoURL,
					Path:           "helm-charts",
					TargetRevision: "dev", // Tracks dev branch (webhook is for master)
				},
			},
		},
	}

	// Setup cache and mocks
	cacheClient := cacheutil.NewCache(cacheutil.NewInMemoryCache(1 * time.Hour))
	repoCache := cache.NewCache(cacheClient, 1*time.Minute, 1*time.Minute, 10*time.Second)
	testSource := &v1alpha1.ApplicationSource{
		RepoURL:        testRepoURL,
		Path:           "helm-charts",
		TargetRevision: "dev",
	}

	testManifests := []string{"apiVersion: v1\nkind: Service\nmetadata:\n  name: test-service"}
	setupTestCache(t, repoCache, "app-different-branch", testSource, testManifests)

	appClientset := appclientset.NewSimpleClientset(app)
	appClientset.PrependReactor("*", "*", reaction)

	mockDB := &mocks.ArgoDB{}
	serverCache := servercache.NewCache(appstate.NewCache(cacheClient, time.Minute), time.Minute, time.Minute)

	h := NewHandler("argocd", []string{}, 10, appClientset, &fakeAppsLister{clientset: appClientset},
		&settings.ArgoCDSettings{}, &fakeSettingsSrc{}, repoCache, serverCache, mockDB, int64(50)*1024*1024)

	w := executeWebhook(t, h, testEventDataDir)
	assert.Equal(t, http.StatusOK, w.Code)

	// App should NOT be patched since it tracks a different branch
	assert.False(t, patched, "app should not be patched when tracking a different branch")

	// Cache should NOT be updated since app doesn't track this revision
	clusterInfo := &mockClusterInfo{}
	var retrievedManifests cache.CachedManifestResponse
	err := repoCache.GetManifests(testAfterSHA, testSource, nil, clusterInfo, "", "", testAppLabelKey, "app-different-branch", &retrievedManifests, nil, "")
	require.Error(t, err, "cache should not be updated when app doesn't track this revision")

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
	testBeforeSHA    = "d5c1ffa8e294bc18c639bfb4e0df499251034414"
	testAfterSHA     = "63738bb582c8b540af7bcfc18f87c575c3ed66e0"
	testRepoURL      = "https://github.com/jessesuen/test-repo"
	testClusterURL   = "https://kubernetes.default.svc"
	testAppLabelKey  = "mycompany.com/appname"
	testEventDataDir = "testdata/github-commit-event.json"
)

// executeWebhook is a helper that executes a webhook request and waits for completion
func executeWebhook(t *testing.T, h *ArgoCDWebhookHandler, eventFile string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := os.ReadFile(eventFile)
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()
	return w
}

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
func setupTestCache(t *testing.T, repoCache *cache.Cache, appName string, source *v1alpha1.ApplicationSource, manifests []string) {
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
	err := repoCache.SetManifests(testBeforeSHA, source, nil, clusterInfo, "", "", testAppLabelKey, appName, dummyManifests, nil, "")
	require.NoError(t, err)
}
