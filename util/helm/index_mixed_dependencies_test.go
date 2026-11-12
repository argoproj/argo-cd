package helm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	imagev1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content"
)

func TestIndexCacheIntegrationHTTPAndOCI(t *testing.T) {
	regressionRequirePinnedHelm(t)
	for _, mixed := range []bool{false, true} {
		t.Run(map[bool]string{false: "OCI only", true: "HTTP and OCI"}[mixed], func(t *testing.T) {
			packageDir := t.TempDir()
			writeChartPackage(t, packageDir, "oci-child", "1.0.0")
			chart, err := os.ReadFile(filepath.Join(packageDir, "oci-child-1.0.0.tgz"))
			require.NoError(t, err)
			config := []byte(`{"name":"oci-child","version":"1.0.0","apiVersion":"v2"}`)
			configDescriptor := content.NewDescriptorFromBytes("application/vnd.cncf.helm.config.v1+json", config)
			layer := content.NewDescriptorFromBytes("application/vnd.cncf.helm.chart.content.v1.tar+gzip", chart)
			manifest, err := json.Marshal(imagev1.Manifest{SchemaVersion: 2, MediaType: imagev1.MediaTypeImageManifest, Config: configDescriptor, Layers: []imagev1.Descriptor{layer}})
			require.NoError(t, err)
			manifestDescriptor := content.NewDescriptorFromBytes(imagev1.MediaTypeImageManifest, manifest)
			blobs := map[string][]byte{configDescriptor.Digest.String(): config, layer.Digest.String(): chart}
			var manifestRequests atomic.Int32
			registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
				username, password, ok := r.BasicAuth()
				if !ok || username != "registry-reader" || password != "registry-secret" {
					w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if r.URL.Path == "/v2/" {
					return
				}
				if strings.HasPrefix(r.URL.Path, "/v2/oci-child/manifests/") {
					manifestRequests.Add(1)
					w.Header().Set("Content-Type", imagev1.MediaTypeImageManifest)
					w.Header().Set("Docker-Content-Digest", manifestDescriptor.Digest.String())
					w.Header().Set("Content-Length", strconv.Itoa(len(manifest)))
					_, _ = w.Write(manifest)
					return
				}
				if body, ok := blobs[strings.TrimPrefix(r.URL.Path, "/v2/oci-child/blobs/")]; ok {
					w.Header().Set("Content-Type", "application/octet-stream")
					w.Header().Set("Content-Length", strconv.Itoa(len(body)))
					_, _ = w.Write(body)
					return
				}
				http.NotFound(w, r)
			}))
			t.Cleanup(registry.Close)
			registryURL := strings.TrimPrefix(registry.URL, "http://")
			fixture := newRegressionChartRepo(t, false, "")
			store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
			for range 2 {
				dir := t.TempDir()
				chart := fmt.Sprintf("apiVersion: v2\nname: mixed\nversion: 1.0.0\ndependencies:\n  - name: oci-child\n    alias: registry\n    version: 1.0.0\n    repository: 'oci://%s'\n", registryURL)
				repos := []HelmRepository{{Name: "registry", Repo: registryURL, Creds: HelmCreds{Username: "registry-reader", Password: "registry-secret"}, EnableOci: true, InsecureOCIForceHttp: true}}
				wantCharts := 1
				if mixed {
					chart += "  - name: child\n    alias: web\n    version: 1.0.0\n    repository: '@web'\n"
					repos = append(repos, HelmRepository{Name: "web", Repo: fixture.server.URL, Creds: HelmCreds{}})
					wantCharts++
				}
				require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chart), 0o600))
				app, err := NewHelmAppWithIndexStore(dir, repos, false, "", "", "", false, false, store, false)
				require.NoError(t, err)
				t.Cleanup(app.Dispose)
				require.NoError(t, app.DependencyBuild(t.Context()))
				output, _, err := app.Template(&TemplateOpts{Name: "mixed", Namespace: "default"})
				require.NoError(t, err)
				require.Equal(t, wantCharts, strings.Count(output, "kind: ConfigMap"))
			}
			requests, _ := fixture.snapshot()
			if mixed {
				require.Len(t, requests, 1, "the warm mixed build must reuse only the HTTP index")
			} else {
				require.Empty(t, requests, "OCI-only builds must not fetch an HTTP index")
				files, err := filepath.Glob(filepath.Join(store.dir, "*.json"))
				require.NoError(t, err)
				require.Empty(t, files, "OCI content must never be published as an index record")
			}
			require.GreaterOrEqual(t, manifestRequests.Load(), int32(2), "OCI resolution and registry authentication must still execute")
		})
	}
}

func writeLocalIndexChart(t *testing.T, dir, name string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(fmt.Sprintf("apiVersion: v2\nname: %s\nversion: 1.0.0\n", name)), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "configmap.yaml"), []byte(fmt.Sprintf("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: %s\n", name)), 0o600))
}

func TestIndexCacheIntegrationFileAndVendoredDependencies(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t, false, "")
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	for range 2 {
		dir := t.TempDir()
		writeLocalIndexChart(t, filepath.Join(dir, "local"), "local")
		writeLocalIndexChart(t, filepath.Join(dir, "charts", "vendored"), "vendored")
		parent := "apiVersion: v2\nname: mixed\nversion: 1.0.0\ndependencies:\n  - name: child\n    version: 1.0.0\n    repository: '@web'\n  - name: local\n    version: 1.0.0\n    repository: 'file://local'\n  - name: vendored\n    version: 1.0.0\n    repository: ''\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(parent), 0o600))
		app, err := NewHelmAppWithIndexStore(dir, []HelmRepository{{Name: "web", Repo: fixture.server.URL, Creds: HelmCreds{}}}, false, "", "", "", false, false, store, false)
		require.NoError(t, err)
		t.Cleanup(app.Dispose)
		require.NoError(t, app.DependencyBuild(t.Context()))
		output, _, err := app.Template(&TemplateOpts{Name: "mixed", Namespace: "default"})
		require.NoError(t, err)
		require.Equal(t, 3, strings.Count(output, "kind: ConfigMap"))
	}
	requests, _ := fixture.snapshot()
	require.Len(t, requests, 1, "file and vendored dependencies must never enter the HTTP provider")
}
