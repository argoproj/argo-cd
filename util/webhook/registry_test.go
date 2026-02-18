package webhook

import (
	"bytes"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubetesting "k8s.io/client-go/testing"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNormalizeOCI(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"lowercase only", "OCI://GHCR.IO/USER/REPO", "oci://ghcr.io/user/repo"},
		{"remove trailing slash", "oci://ghcr.io/user/repo/", "oci://ghcr.io/user/repo"},
		{"already normalized", "oci://ghcr.io/user/repo", "oci://ghcr.io/user/repo"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeOCI(tt.url)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestIsRegistryEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    string
		expected bool
	}{
		{"package event", "package", true},
		{"registry package event", "registry_package", false},
		{"push event", "push", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.Header.Set("X-GitHub-Event", tt.event)
			assert.Equal(t, tt.expected, IsRegistryEvent(req))
		})
	}
}

func TestRegistryPackageEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "package")
	payload, err := os.ReadFile("testdata/ghcr-package-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(payload))

	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, hook.LastEntry().Message, "Received registry webhook event")
}

func TestProcessWebhook_Unsupported(t *testing.T) {
	h := NewWebhookRegistryHandler("")

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
	req.Header.Set("X-GitHub-Event", "push") // not registry

	_, err := h.ProcessWebhook(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported registry webhook")
}

func TestHandleRegistryEvent_RefreshMatchingApp(t *testing.T) {
	hook := test.NewGlobal()

	patchedApps := []string{}

	reaction := func(action kubetesting.Action) (bool, runtime.Object, error) {
		patch := action.(kubetesting.PatchAction)
		patchedApps = append(patchedApps, patch.GetName())
		return true, nil, nil
	}

	h := NewMockHandler(
		&reactorDef{"patch", "applications", reaction},
		[]string{},
		&v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "oci-app",
				Namespace: "argocd",
			},
			Spec: v1alpha1.ApplicationSpec{
				Sources: v1alpha1.ApplicationSources{
					{
						RepoURL:        "oci://ghcr.io/user/repo",
						TargetRevision: "1.0.0",
					},
				},
			},
		},
	)

	event := &WebhookRegistryEvent{
		RegistryURL: "ghcr.io",
		Repository:  "user/repo",
		Tag:         "1.0.0",
	}

	h.HandleRegistryEvent(event)

	assert.Contains(t, patchedApps, "oci-app")
	assert.Contains(t, hook.LastEntry().Message, "Requested app 'oci-app' refresh")
}

func TestHandleRegistryEvent_RepoMismatch(t *testing.T) {
	hook := test.NewGlobal()

	h := NewMockHandler(nil, []string{},
		&v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "oci-app",
				Namespace: "argocd",
			},
			Spec: v1alpha1.ApplicationSpec{
				Sources: v1alpha1.ApplicationSources{
					{
						RepoURL:        "oci://ghcr.io/other/repo",
						TargetRevision: "1.0.0",
					},
				},
			},
		},
	)

	event := &WebhookRegistryEvent{
		RegistryURL: "ghcr.io",
		Repository:  "user/repo",
		Tag:         "1.0.0",
	}

	h.HandleRegistryEvent(event)
	assert.Contains(t, hook.LastEntry().Message, "Skipping normalizing since URLs doesn't match")
}

func TestHandleRegistryEvent_RevisionMismatch(t *testing.T) {
	hook := test.NewGlobal()

	h := NewMockHandler(
		nil,
		[]string{},
		&v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "oci-app",
				Namespace: "argocd",
			},
			Spec: v1alpha1.ApplicationSpec{
				Sources: v1alpha1.ApplicationSources{
					{
						RepoURL:        "oci://ghcr.io/user/repo",
						TargetRevision: "2.0.0",
					},
				},
			},
		},
	)

	event := &WebhookRegistryEvent{
		RegistryURL: "ghcr.io",
		Repository:  "user/repo",
		Tag:         "1.0.0",
	}

	h.HandleRegistryEvent(event)
	assert.Contains(t, hook.LastEntry().Message, "revision and TargetRevision are not matching")
}

func TestHandleRegistryEvent_NamespaceFiltering(t *testing.T) {
	patched := []string{}

	reaction := func(action kubetesting.Action) (bool, runtime.Object, error) {
		patch := action.(kubetesting.PatchAction)
		patched = append(patched, patch.GetNamespace())
		return true, nil, nil
	}

	h := NewMockHandler(
		&reactorDef{"patch", "applications", reaction},
		[]string{"team-*"},
		&v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "app1",
				Namespace: "team-a",
			},
			Spec: v1alpha1.ApplicationSpec{
				Sources: v1alpha1.ApplicationSources{
					{RepoURL: "oci://ghcr.io/user/repo", TargetRevision: "1.0.0"},
				},
			},
		},
		&v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "app2",
				Namespace: "kube-system",
			},
			Spec: v1alpha1.ApplicationSpec{
				Sources: v1alpha1.ApplicationSources{
					{RepoURL: "oci://ghcr.io/user/repo", TargetRevision: "1.0.0"},
				},
			},
		},
	)

	event := &WebhookRegistryEvent{
		RegistryURL: "ghcr.io",
		Repository:  "user/repo",
		Tag:         "1.0.0",
	}

	h.HandleRegistryEvent(event)

	assert.Contains(t, patched, "team-a")
	assert.NotContains(t, patched, "kube-system")
}
