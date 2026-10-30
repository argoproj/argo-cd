package profile

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegisterProfile_FileIsMissing(t *testing.T) {
	mux := http.NewServeMux()
	RegisterProfiler(mux)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/debug/pprof/", http.NoBody)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

func TestRegisterProfile_FileExist(t *testing.T) {
	mux := http.NewServeMux()
	RegisterProfiler(mux)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	f, err := os.CreateTemp(t.TempDir(), "test")
	require.NoError(t, err)
	_, err = f.WriteString("true")
	require.NoError(t, err)

	oldVal := enableProfilerFilePath
	enableProfilerFilePath = f.Name()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/debug/pprof/", http.NoBody)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	enableProfilerFilePath = oldVal
	_ = f.Close()
	_ = os.Remove(f.Name())
}
