package webhook

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/util/previews"
	"github.com/argoproj/argo-cd/util/settings"
)

func NewMockHandler() *ArgoCDWebhookHandler {
	appClientset := appclientset.NewSimpleClientset()
	return NewHandler("", appClientset, previews.PreviewService{}, &settings.ArgoCDSettings{})
}
func TestGitHubCommitEvent(t *testing.T) {
	h := NewMockHandler()
	req := httptest.NewRequest("POST", "/api/webhook", nil)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := ioutil.ReadFile("github-commit-event.json")
	assert.NoError(t, err)
	req.Body = ioutil.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
}

func TestGitHubTagEvent(t *testing.T) {
	h := NewMockHandler()
	req := httptest.NewRequest("POST", "/api/webhook", nil)
	req.Header.Set("X-GitHub-Event", "push")
	eventJSON, err := ioutil.ReadFile("github-tag-event.json")
	assert.NoError(t, err)
	req.Body = ioutil.NopCloser(bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
}
