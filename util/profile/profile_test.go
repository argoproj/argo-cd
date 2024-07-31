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

	resp, err := http.Get(srv.URL + "/debug/pprof/")
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestRegisterProfile_FileExist(t *testing.T) {
	mux := http.NewServeMux()
	RegisterProfiler(mux)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	f, err := os.CreateTemp("", "test")
	require.NoError(t, err)
	_, err = f.WriteString("true")
	require.NoError(t, err)

	oldVal := enableProfilerFilePath
	enableProfilerFilePath = f.Name()

	resp, err := http.Get(srv.URL + "/debug/pprof/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	defer func() {
		enableProfilerFilePath = oldVal
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
}
