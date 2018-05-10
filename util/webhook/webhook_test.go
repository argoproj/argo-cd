package webhook

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/util/settings"
	"github.com/gobuffalo/packr"
	"github.com/stretchr/testify/assert"
)

var (
	box packr.Box
)

func init() {
	box = packr.NewBox(".")
}

func NewMockHandler() *ArgoCDWebhookHandler {
	appClientset := appclientset.NewSimpleClientset()
	return NewHandler("", appClientset, &settings.ArgoCDSettings{})
}
func TestGitHubCommitEvent(t *testing.T) {
	h := NewMockHandler()
	req := httptest.NewRequest("POST", "/api/webhook", nil)
	req.Header.Set("X-GitHub-Event", "push")
	req.Body = ioutil.NopCloser(bytes.NewReader(box.Bytes("github-commit-event.json")))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
}

func TestGitHubTagEvent(t *testing.T) {
	h := NewMockHandler()
	req := httptest.NewRequest("POST", "/api/webhook", nil)
	req.Header.Set("X-GitHub-Event", "push")
	req.Body = ioutil.NopCloser(bytes.NewReader(box.Bytes("github-tag-event.json")))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
}
