package webhook

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubetesting "k8s.io/client-go/testing"
)

func TestNormalizeOCI(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"strips oci:// prefix and lowercases", "oci://GHCR.IO/USER/REPO", "ghcr.io/user/repo"},
		{"strips oci:// prefix and trailing slash", "oci://ghcr.io/user/repo/", "ghcr.io/user/repo"},
		{"already normalized with prefix", "oci://ghcr.io/user/repo", "ghcr.io/user/repo"},
		{"without oci:// prefix", "ghcr.io/user/repo", "ghcr.io/user/repo"},
		{"uppercase without prefix", "GHCR.IO/USER/REPO", "ghcr.io/user/repo"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeOCI(tt.url)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGHCRHandlerCanHandle(t *testing.T) {
	h := newGHCRParser("")

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
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", http.NoBody)
			req.Header.Set("X-GitHub-Event", tt.event)
			assert.Equal(t, tt.expected, h.CanHandle(req))
		})
	}
}

func TestRegistryPackageEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandler(nil, []string{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook", http.NoBody)
	req.Header.Set("X-GitHub-Event", "package")
	payload, err := os.ReadFile("testdata/ghcr-package-event.json")
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(payload))

	w := httptest.NewRecorder()
	h.Handler(w, req)
	close(h.queue)
	h.Wait()

	assert.Equal(t, http.StatusOK, w.Code)
	assertLogContains(t, hook, "Received registry webhook event")
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

	event := &RegistryEvent{
		RegistryURL: "ghcr.io",
		Repository:  "user/repo",
		Tag:         "1.0.0",
	}

	h.HandleRegistryEvent(event)

	assert.Contains(t, patchedApps, "oci-app")
	assert.Contains(t, hook.LastEntry().Message, "Requested app 'oci-app' refresh")
}

func TestHandleRegistryEvent_RepoMismatch(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
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

	event := &RegistryEvent{
		RegistryURL: "ghcr.io",
		Repository:  "user/repo",
		Tag:         "1.0.0",
	}

	h.HandleRegistryEvent(event)
	assert.Contains(t, hook.LastEntry().Message, "Skipping app: OCI repository URLs do not match")
}

func TestHandleRegistryEvent_RevisionMismatch(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
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

	event := &RegistryEvent{
		RegistryURL: "ghcr.io",
		Repository:  "user/repo",
		Tag:         "1.0.0",
	}

	h.HandleRegistryEvent(event)
	assert.Contains(t, hook.LastEntry().Message, "Skipping app: revision does not match targetRevision")
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

	event := &RegistryEvent{
		RegistryURL: "ghcr.io",
		Repository:  "user/repo",
		Tag:         "1.0.0",
	}

	h.HandleRegistryEvent(event)

	assert.Contains(t, patched, "team-a")
	assert.NotContains(t, patched, "kube-system")
}
