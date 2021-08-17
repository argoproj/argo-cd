package webhook

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func NewMockHandler() *ArgoCDWebhookHandler {
	appClientset := appclientset.NewSimpleClientset()
	cacheClient := cacheutil.NewCache(cacheutil.NewInMemoryCache(1 * time.Hour))
	return NewHandler("", appClientset, &settings.ArgoCDSettings{}, &fakeSettingsSrc{}, cache.NewCache(
		cacheClient,
		1*time.Minute,
		1*time.Minute,
	), servercache.NewCache(appstate.NewCache(cacheClient, time.Minute), time.Minute, time.Minute, time.Minute), &mocks.ArgoDB{})
}

func TestGitHubCommitEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler()
	req := httptest.NewRequest("POST", "/api/webhook", nil)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := ioutil.ReadFile("github-commit-event.json")
	assert.NoError(t, err)
	req.Body = ioutil.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
	expectedLogResult := "Received push event repo: https://github.com/jessesuen/test-repo, revision: master, touchedHead: true"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestGitHubTagEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler()
	req := httptest.NewRequest("POST", "/api/webhook", nil)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := ioutil.ReadFile("github-tag-event.json")
	assert.NoError(t, err)
	req.Body = ioutil.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
	expectedLogResult := "Received push event repo: https://github.com/jessesuen/test-repo, revision: v1.0, touchedHead: false"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestBitbucketServerRepositoryReferenceChangedEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler()
	req := httptest.NewRequest("POST", "/api/webhook", nil)
	req.Header.Set("X-Event-Key", "repo:refs_changed")
	eventJSON, err := ioutil.ReadFile("bitbucket-server-event.json")
	assert.NoError(t, err)
	req.Body = ioutil.NopCloser(bytes.NewReader(eventJSON))
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
	h := NewMockHandler()
	eventJSON := "{\"test\": true}"
	req := httptest.NewRequest("POST", "/api/webhook", bytes.NewBufferString(eventJSON))
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
	h := NewMockHandler()
	req := httptest.NewRequest("POST", "/api/webhook", nil)
	req.Header.Set("X-Gogs-Event", "push")
	eventJSON, err := ioutil.ReadFile("gogs-event.json")
	assert.NoError(t, err)
	req.Body = ioutil.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
	expectedLogResult := "Received push event repo: http://gogs-server/john/repo-test, revision: master, touchedHead: true"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestGitLabPushEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler()
	req := httptest.NewRequest("POST", "/api/webhook", nil)
	req.Header.Set("X-Gitlab-Event", "Push Hook")
	eventJSON, err := ioutil.ReadFile("gitlab-event.json")
	assert.NoError(t, err)
	req.Body = ioutil.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
	expectedLogResult := "Received push event repo: https://gitlab/group/name, revision: master, touchedHead: true"
	assert.Equal(t, expectedLogResult, hook.LastEntry().Message)
	hook.Reset()
}

func TestInvalidMethod(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler()
	req := httptest.NewRequest("GET", "/api/webhook", nil)
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
	h := NewMockHandler()
	req := httptest.NewRequest("POST", "/api/webhook", nil)
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
	h := NewMockHandler()
	req := httptest.NewRequest("POST", "/api/webhook", nil)
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
			Source: v1alpha1.ApplicationSource{
				Path: sourcePath,
			},
		},
	}
}

func Test_getAppRefreshPrefix(t *testing.T) {
	tests := []struct {
		name  string
		app   *v1alpha1.Application
		files []string
		want  bool
	}{
		{"default no path", &v1alpha1.Application{}, []string{"README.md"}, true},
		{"relative path - matching", getApp(".", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"relative path - not matching", getApp(".", "source/path"), []string{"README.md"}, false},
		{"absolute path - matching", getApp("/source/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"absolute path - not matching", getApp("/source/path1", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"two relative paths - matching", getApp(".;../shared", "my-app"), []string{"shared/my-deployment.yaml"}, true},
		{"two relative paths - not matching", getApp(".;../shared", "my-app"), []string{"README.md"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := appFilesHaveChanged(tt.app, tt.files); got != tt.want {
				t.Errorf("getAppRefreshPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppRevisionHasChanged(t *testing.T) {
	assert.True(t, appRevisionHasChanged(&v1alpha1.Application{Spec: v1alpha1.ApplicationSpec{
		Source: v1alpha1.ApplicationSource{},
	}}, "master", true))

	assert.False(t, appRevisionHasChanged(&v1alpha1.Application{Spec: v1alpha1.ApplicationSpec{
		Source: v1alpha1.ApplicationSource{},
	}}, "master", false))

	assert.False(t, appRevisionHasChanged(&v1alpha1.Application{Spec: v1alpha1.ApplicationSpec{
		Source: v1alpha1.ApplicationSource{
			TargetRevision: "dev",
		},
	}}, "master", true))

	assert.True(t, appRevisionHasChanged(&v1alpha1.Application{Spec: v1alpha1.ApplicationSpec{
		Source: v1alpha1.ApplicationSource{
			TargetRevision: "dev",
		},
	}}, "dev", false))

	assert.False(t, appRevisionHasChanged(&v1alpha1.Application{Spec: v1alpha1.ApplicationSpec{
		Source: v1alpha1.ApplicationSource{
			TargetRevision: "refs/heads/dev",
		},
	}}, "master", true))

	assert.True(t, appRevisionHasChanged(&v1alpha1.Application{Spec: v1alpha1.ApplicationSpec{
		Source: v1alpha1.ApplicationSource{
			TargetRevision: "refs/heads/dev",
		},
	}}, "dev", false))
}
