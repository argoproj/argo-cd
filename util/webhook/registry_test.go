package webhook

import (
	"bytes"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
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
