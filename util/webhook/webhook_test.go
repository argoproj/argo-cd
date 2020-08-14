package webhook

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/util/settings"
)

func NewMockHandler() *ArgoCDWebhookHandler {
	appClientset := appclientset.NewSimpleClientset()
	return NewHandler("", appClientset, &settings.ArgoCDSettings{})
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

func Test_getAppRefreshPrefix(t *testing.T) {
	tests := []struct {
		name string
		app  *v1alpha1.Application
		want string
	}{
		{
			"default no prefix",
			&v1alpha1.Application{},
			"",
		},
		{
			"use path prefix",
			&v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"argocd.argoproj.io/refresh-on-path-updates-only": "true",
					},
				},
				Spec: v1alpha1.ApplicationSpec{
					Source: v1alpha1.ApplicationSource{
						Path: "soource/path",
					},
				},
			},
			"soource/path",
		},
		{
			"use explicit path",
			&v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"argocd.argoproj.io/refresh-prefix": "explicit/refresh/prefix",
					},
				},
				Spec: v1alpha1.ApplicationSpec{
					Source: v1alpha1.ApplicationSource{
						Path: "testpath/here",
					},
				},
			},
			"explicit/refresh/prefix",
		},
		{
			"explicit overrides path",
			&v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"argocd.argoproj.io/refresh-on-path-updates-only": "true",
						"argocd.argoproj.io/refresh-prefix":               "explicit/refresh/prefix",
					},
				},
				Spec: v1alpha1.ApplicationSpec{
					Source: v1alpha1.ApplicationSource{
						Path: "soource/path",
					},
				},
			},
			"explicit/refresh/prefix",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getAppRefreshPrefix(tt.app); got != tt.want {
				t.Errorf("getAppRefreshPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}
