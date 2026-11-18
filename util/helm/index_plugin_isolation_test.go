package helm

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIndexCacheDoesNotDiscoverCheckoutGetters(t *testing.T) {
	regressionRequirePinnedHelm(t)
	for _, mode := range []string{"empty", "missing-home", "relative"} {
		t.Run(mode, func(t *testing.T) {
			for _, name := range []string{"HELM_PLUGINS", "HELM_DATA_HOME", "XDG_DATA_HOME"} {
				t.Setenv(name, "")
				require.NoError(t, os.Unsetenv(name))
			}
			switch mode {
			case "empty":
				t.Setenv("HELM_PLUGINS", "")
			case "missing-home":
				t.Setenv("HOME", "")
			case "relative":
				t.Setenv("HELM_PLUGINS", "checkout-plugins")
			}
			parent := regressionParent(t, "@charts", nil)
			plugins := filepath.Join(parent, helmPluginsDirectory())
			dir := filepath.Join(plugins, "fixture-getter")
			require.NoError(t, os.MkdirAll(dir, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte("name: fixture-getter\nversion: 0.1.0\ndownloaders:\n  - protocols: [fixture]\n    command: getter.sh\n"), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "getter.sh"), []byte("#!/bin/sh\ntouch \"$HELM_PLUGIN_DIR/executed\"\ncat \"$HELM_PLUGIN_DIR/child.tgz\"\n"), 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "child.tgz"), regressionPackage(t, "1.0.0"), 0o600))
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, "apiVersion: v1\nentries:\n  child:\n    - name: child\n      version: 1.0.0\n      apiVersion: v2\n      urls: [fixture://charts/child.tgz]\n")
			}))
			t.Cleanup(origin.Close)
			_, err := regressionBuild(t, parent, HelmRepository{Name: "charts", Repo: origin.URL, Creds: HelmCreds{}}, NewIndexStore(t.TempDir(), time.Minute, 1<<20), false)
			require.Error(t, err, "a checkout getter must not supply the archive")
			_, err = os.Stat(filepath.Join(dir, "executed"))
			require.True(t, os.IsNotExist(err), "Helm must never execute the checkout getter")
		})
	}
}
