package helm

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/v3/util/io/path"
)

func template(h Helm, opts *TemplateOpts) ([]*unstructured.Unstructured, error) {
	out, _, err := h.Template(opts)
	if err != nil {
		return nil, err
	}
	return kube.SplitYAML([]byte(out))
}

func TestHelmTemplateParams(t *testing.T) {
	t.Parallel()
	h, err := NewHelmApp("./testdata/minio", []HelmRepository{}, false, "", "", "", false, false)
	require.NoError(t, err)
	opts := TemplateOpts{
		Name: "test",
		Set: map[string]string{
			"service.type": "LoadBalancer",
			"service.port": "1234",
		},
		SetString: map[string]string{
			"service.annotations.prometheus\\.io/scrape": "true",
		},
	}
	objs, err := template(h, &opts)
	require.NoError(t, err)
	assert.Len(t, objs, 5)

	for _, obj := range objs {
		if obj.GetKind() != "Service" || obj.GetName() != "test-minio" {
			continue
		}
		var svc corev1.Service
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &svc)
		require.NoError(t, err)
		assert.Equal(t, corev1.ServiceTypeLoadBalancer, svc.Spec.Type)
		assert.Equal(t, int32(1234), svc.Spec.Ports[0].TargetPort.IntVal)
		assert.Equal(t, "true", svc.Annotations["prometheus.io/scrape"])
	}
}

func TestHelmTemplateValues(t *testing.T) {
	t.Parallel()
	repoRoot := "./testdata/redis"
	repoRootAbs, err := filepath.Abs(repoRoot)
	require.NoError(t, err)
	h, err := NewHelmApp(repoRootAbs, []HelmRepository{}, false, "", "", "", false, false)
	require.NoError(t, err)
	valuesPath, _, err := path.ResolveValueFilePathOrUrl(repoRootAbs, repoRootAbs, "values-production.yaml", nil)
	require.NoError(t, err)
	opts := TemplateOpts{
		Name:   "test",
		Values: []path.ResolvedFilePath{valuesPath},
	}
	objs, err := template(h, &opts)
	require.NoError(t, err)
	assert.Len(t, objs, 8)

	for _, obj := range objs {
		if obj.GetKind() == "Deployment" && obj.GetName() == "test-redis-slave" {
			var dep appsv1.Deployment
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &dep)
			require.NoError(t, err)
			assert.Equal(t, int32(3), *dep.Spec.Replicas)
		}
	}
}

func TestHelmGetParams(t *testing.T) {
	t.Parallel()
	repoRoot := "./testdata/redis"
	repoRootAbs, err := filepath.Abs(repoRoot)
	require.NoError(t, err)
	h, err := NewHelmApp(repoRootAbs, nil, false, "", "", "", false, false)
	require.NoError(t, err)
	params, err := h.GetParameters(nil, repoRootAbs, repoRootAbs)
	require.NoError(t, err)

	slaveCountParam := params["cluster.slaveCount"]
	assert.Equal(t, "1", slaveCountParam)
}

func TestHelmGetParamsValueFiles(t *testing.T) {
	t.Parallel()
	repoRoot := "./testdata/redis"
	repoRootAbs, err := filepath.Abs(repoRoot)
	require.NoError(t, err)
	h, err := NewHelmApp(repoRootAbs, nil, false, "", "", "", false, false)
	require.NoError(t, err)
	valuesPath, _, err := path.ResolveValueFilePathOrUrl(repoRootAbs, repoRootAbs, "values-production.yaml", nil)
	require.NoError(t, err)
	params, err := h.GetParameters([]path.ResolvedFilePath{valuesPath}, repoRootAbs, repoRootAbs)
	require.NoError(t, err)

	slaveCountParam := params["cluster.slaveCount"]
	assert.Equal(t, "3", slaveCountParam)
}

func TestHelmGetParamsValueFilesThatExist(t *testing.T) {
	t.Parallel()
	repoRoot := "./testdata/redis"
	repoRootAbs, err := filepath.Abs(repoRoot)
	require.NoError(t, err)
	h, err := NewHelmApp(repoRootAbs, nil, false, "", "", "", false, false)
	require.NoError(t, err)
	valuesMissingPath, _, err := path.ResolveValueFilePathOrUrl(repoRootAbs, repoRootAbs, "values-missing.yaml", nil)
	require.NoError(t, err)
	valuesProductionPath, _, err := path.ResolveValueFilePathOrUrl(repoRootAbs, repoRootAbs, "values-production.yaml", nil)
	require.NoError(t, err)
	params, err := h.GetParameters([]path.ResolvedFilePath{valuesMissingPath, valuesProductionPath}, repoRootAbs, repoRootAbs)
	require.NoError(t, err)

	slaveCountParam := params["cluster.slaveCount"]
	assert.Equal(t, "3", slaveCountParam)
}

func TestHelmTemplateReleaseNameOverwrite(t *testing.T) {
	t.Parallel()
	h, err := NewHelmApp("./testdata/redis", nil, false, "", "", "", false, false)
	require.NoError(t, err)

	objs, err := template(h, &TemplateOpts{Name: "my-release"})
	require.NoError(t, err)
	assert.Len(t, objs, 5)

	for _, obj := range objs {
		if obj.GetKind() == "StatefulSet" {
			var stateful appsv1.StatefulSet
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &stateful)
			require.NoError(t, err)
			assert.Equal(t, "my-release-redis-master", stateful.Name)
		}
	}
}

func TestHelmTemplateReleaseName(t *testing.T) {
	t.Parallel()
	h, err := NewHelmApp("./testdata/redis", nil, false, "", "", "", false, false)
	require.NoError(t, err)
	objs, err := template(h, &TemplateOpts{Name: "test"})
	require.NoError(t, err)
	assert.Len(t, objs, 5)

	for _, obj := range objs {
		if obj.GetKind() == "StatefulSet" {
			var stateful appsv1.StatefulSet
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &stateful)
			require.NoError(t, err)
			assert.Equal(t, "test-redis-master", stateful.Name)
		}
	}
}

func TestHelmArgCleaner(t *testing.T) {
	t.Parallel()
	for input, expected := range map[string]string{
		`val`:        `val`,
		`bar`:        `bar`,
		`not, clean`: `not\, clean`,
		`a\,b,c`:     `a\,b\,c`,
		`{a,b,c}`:    `{a,b,c}`,
		`,,,,,\,`:    `\,\,\,\,\,\,`,
		`\,,\\,,`:    `\,\,\\,\,`,
	} {
		cleaned := cleanSetParameters(input)
		assert.Equal(t, expected, cleaned)
	}
}

func TestVersion(t *testing.T) {
	t.Parallel()
	ver, err := Version()
	require.NoError(t, err)
	assert.NotEmpty(t, ver)
}

func Test_flatVals(t *testing.T) {
	t.Run("Map", func(t *testing.T) {
		output := map[string]string{}

		flatVals(map[string]any{"foo": map[string]any{"bar": "baz"}}, output)

		assert.Equal(t, map[string]string{"foo.bar": "baz"}, output)
	})
	t.Run("Array", func(t *testing.T) {
		output := map[string]string{}

		flatVals(map[string]any{"foo": []any{"bar", "baz"}}, output)

		assert.Equal(t, map[string]string{"foo[0]": "bar", "foo[1]": "baz"}, output)
	})
	t.Run("Val", func(t *testing.T) {
		output := map[string]string{}

		flatVals(map[string]any{"foo": 1}, output)

		assert.Equal(t, map[string]string{"foo": "1"}, output)
	})
}

func TestAPIVersions(t *testing.T) {
	t.Parallel()
	h, err := NewHelmApp("./testdata/api-versions", nil, false, "", "", "", false, false)
	require.NoError(t, err)

	objs, err := template(h, &TemplateOpts{})
	require.NoError(t, err)
	require.Len(t, objs, 1)
	assert.Equal(t, "sample/v1", objs[0].GetAPIVersion())

	objs, err = template(h, &TemplateOpts{APIVersions: []string{"sample/v2"}})
	require.NoError(t, err)
	require.Len(t, objs, 1)
	assert.Equal(t, "sample/v2", objs[0].GetAPIVersion())
}

func TestKubeVersionWithSymbol(t *testing.T) {
	t.Parallel()
	h, err := NewHelmApp("./testdata/tests", nil, false, "", "", "", false, false)
	require.NoError(t, err)

	objs, err := template(h, &TemplateOpts{KubeVersion: "1.30.11+IKS"})
	require.NoError(t, err)
	require.Len(t, objs, 2)

	for _, obj := range objs {
		if obj.GetKind() != "ConfigMap" {
			continue
		}
		var configMap corev1.ConfigMap
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &configMap)
		require.NoError(t, err)
		if data, ok := configMap.Data["kubeVersion"]; ok {
			assert.Equal(t, "v1.30.11", data)
			return
		}
		t.Fatal("expected kubeVersion key not found in configMap")
	}
}

func TestSkipCrds(t *testing.T) {
	t.Parallel()
	h, err := NewHelmApp("./testdata/crds", nil, false, "", "", "", false, false)
	require.NoError(t, err)

	objs, err := template(h, &TemplateOpts{SkipCrds: false})
	require.NoError(t, err)
	require.Len(t, objs, 1)

	objs, err = template(h, &TemplateOpts{})
	require.NoError(t, err)
	require.Len(t, objs, 1)

	objs, err = template(h, &TemplateOpts{SkipCrds: true})
	require.NoError(t, err)
	require.Empty(t, objs)
}

func TestSkipTests(t *testing.T) {
	t.Parallel()
	h, err := NewHelmApp("./testdata/tests", nil, false, "", "", "", false, false)
	require.NoError(t, err)

	objs, err := template(h, &TemplateOpts{SkipTests: false})
	require.NoError(t, err)
	require.Len(t, objs, 2)

	objs, err = template(h, &TemplateOpts{})
	require.NoError(t, err)
	require.Len(t, objs, 2)

	objs, err = template(h, &TemplateOpts{SkipTests: true})
	require.NoError(t, err)
	require.Empty(t, objs)
}

func TestDependencyBuild_PlainHTTPFromDependencyRepo(t *testing.T) {
	// dependency build has no per-repo --plain-http; if ANY dependency repo is
	// plain-http, the whole build must use --plain-http (see helm.DependencyBuild).
	tests := []struct {
		name            string
		depInsecureHTTP []bool // one entry per dependency repo
		expectPlainHTTP bool
	}{
		{
			name:            "single https dep — no plain-http",
			depInsecureHTTP: []bool{false},
			expectPlainHTTP: false,
		},
		{
			name:            "single plain-http dep — plain-http",
			depInsecureHTTP: []bool{true},
			expectPlainHTTP: true,
		},
		{
			name:            "mixed deps — any plain-http dep forces plain-http for the whole build",
			depInsecureHTTP: []bool{false, true},
			expectPlainHTTP: true,
		},
		{
			name:            "all https deps — no plain-http",
			depInsecureHTTP: []bool{false, false},
			expectPlainHTTP: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedArgs []string
			c, err := newCmdWithVersion(".", false, "", "", func(cmd *exec.Cmd, _ func(string) string) (string, error) {
				capturedArgs = cmd.Args
				return "", nil
			})
			require.NoError(t, err)

			repos := make([]HelmRepository, len(tc.depInsecureHTTP))
			for i, forceHTTP := range tc.depInsecureHTTP {
				repos[i] = HelmRepository{
					Repo:                 "oci://localhost:5000/myrepo",
					EnableOci:            true,
					InsecureOCIForceHttp: forceHTTP,
					Creds:                HelmCreds{},
				}
			}

			h := &helm{
				cmd:   *c,
				repos: repos,
			}

			err = h.DependencyBuild(t.Context())
			require.NoError(t, err)

			require.Equal(t, tc.expectPlainHTTP, slices.Contains(capturedArgs, "--plain-http"))
		})
	}
}

func TestIndexBuildOriginFailureDoesNotFallBack(t *testing.T) {
	for _, forced := range []bool{false, true} {
		t.Run(map[bool]string{false: "expired", true: "hard refresh"}[forced], func(t *testing.T) {
			var requests, commands atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); w.WriteHeader(http.StatusUnauthorized) }))
			t.Cleanup(server.Close)
			command, err := newCmdWithVersion(t.TempDir(), false, "", "", func(*exec.Cmd, func(string) string) (string, error) { commands.Add(1); return "", nil })
			require.NoError(t, err)
			t.Cleanup(command.Close)
			repo := HelmRepository{Name: "private", Repo: server.URL, Creds: HelmCreds{}}
			store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
			require.NoError(t, store.Set(repoIdentity(repo), []byte(regressionIndexBody)))
			old := time.Now().Add(-2 * time.Hour)
			require.NoError(t, os.Chtimes(store.cacheFileName(repoIdentity(repo)), old, old))
			command.persistentIndexStore = store
			app := &helm{cmd: *command, repos: []HelmRepository{repo}, forceRefresh: forced}
			err = app.DependencyBuild(t.Context())
			require.ErrorContains(t, err, "401")
			require.EqualValues(t, 1, requests.Load())
			require.Zero(t, commands.Load(), "origin failure must not be hidden by a second download through Helm")
		})
	}
}

func TestIndexBuildUnsupportedGetterUsesWholeLegacyPath(t *testing.T) {
	t.Setenv("HELM_DATA_HOME", "/inherited/getter-plugins")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(regressionIndexBody))
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("apiVersion: v2\nname: plugins\nversion: 1.0.0\ndependencies:\n  - name: custom\n    version: 1.0.0\n    repository: s3://custom-getter/charts\n"), 0o600))
	var calls [][]string
	var environment []string
	command, err := newCmdWithVersion(dir, false, "", "", func(command *exec.Cmd, _ func(string) string) (string, error) {
		calls = append(calls, append([]string(nil), command.Args...))
		environment = command.Env
		return "", nil
	})
	require.NoError(t, err)
	t.Cleanup(command.Close)
	command.persistentIndexStore = NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	app := &helm{cmd: *command, repos: []HelmRepository{{Name: "http", Repo: server.URL, Creds: HelmCreds{}}}}
	require.NoError(t, app.DependencyBuild(t.Context()))
	require.Zero(t, requests.Load(), "classification must happen before any provider fetch")
	require.Len(t, calls, 2)
	require.Contains(t, strings.Join(calls[0], " "), "repo add")
	require.Contains(t, strings.Join(calls[1], " "), "dependency build")
	require.NotContains(t, calls[1], "--skip-refresh")
	require.Contains(t, environment, "HELM_DATA_HOME=/inherited/getter-plugins", "unsupported getters must retain their legacy plugin environment")
}

func TestIndexBuildDeduplicatesForcedAliases(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t, false, "")
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("apiVersion: v2\nname: parent\nversion: 0.1.0\ndependencies:\n  - name: child\n    alias: first\n    version: 1.0.0\n    repository: '@first'\n  - name: child\n    alias: second\n    version: 1.0.0\n    repository: 'alias:second'\n"), 0o600))
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	app, err := NewHelmAppWithIndexStore(dir, []HelmRepository{
		{Name: "first", Repo: fixture.server.URL, Creds: HelmCreds{}},
		{Name: "second", Repo: fixture.server.URL + "/", Creds: HelmCreds{}},
	}, false, "", "", "", false, false, store, true)
	require.NoError(t, err)
	t.Cleanup(app.Dispose)
	require.NoError(t, app.DependencyBuild(t.Context()))
	output, _, err := app.Template(&TemplateOpts{Name: "aliases", Namespace: "default"})
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(output, "kind: ConfigMap"))
	requests, _ := fixture.snapshot()
	require.Len(t, requests, 1, "aliases and trailing-slash variants share one forced fetch")
	require.Empty(t, requests[0].etag)
}

func TestIndexBuildIsolatesInheritedHelmHomes(t *testing.T) {
	regressionRequirePinnedHelm(t)
	shared := t.TempDir()
	for _, name := range []string{"HELM_CONFIG_HOME", "HELM_CACHE_HOME", "HELM_DATA_HOME", "HELM_REPOSITORY_CACHE"} {
		t.Setenv(name, shared)
	}
	t.Setenv("HELM_REPOSITORY_CONFIG", filepath.Join(shared, "repositories.yaml"))
	t.Setenv("HELM_REGISTRY_CONFIG", filepath.Join(shared, "registry.json"))
	fixture := newRegressionChartRepo(t, false, "")
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	app, err := regressionBuild(t, regressionParent(t, "@fixture", nil), repo, NewIndexStore(t.TempDir(), time.Hour, 1<<20), false)
	require.NoError(t, err)
	regressionRender(t, app, "1.0.0")
	files, err := os.ReadDir(shared)
	require.NoError(t, err)
	require.Empty(t, files, "enabled builds must never write config, indexes or credentials into inherited Helm homes")
}
