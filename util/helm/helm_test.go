package helm

import (
	"context"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/io/path"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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

func TestRegistryLoginOCI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		repos          []HelmRepository
		expectedLogins []string // registry hosts that should be logged into
	}{
		{
			name: "OCI repo with credentials is logged in",
			repos: []HelmRepository{
				{Repo: "example.com/myrepo", EnableOci: true, Creds: HelmCreds{Username: "user", Password: "pass"}},
			},
			expectedLogins: []string{"example.com"},
		},
		{
			name: "non-OCI repo is skipped",
			repos: []HelmRepository{
				{Repo: "https://charts.example.com", EnableOci: false, Creds: HelmCreds{Username: "user", Password: "pass"}},
			},
			expectedLogins: nil,
		},
		{
			name: "OCI repo without credentials is skipped",
			repos: []HelmRepository{
				{Repo: "example.com/myrepo", EnableOci: true, Creds: HelmCreds{}},
			},
			expectedLogins: nil,
		},
		{
			name: "mixed repos — only OCI with credentials",
			repos: []HelmRepository{
				{Repo: "example.com/myrepo", EnableOci: true, Creds: HelmCreds{Username: "user", Password: "pass"}},
				{Repo: "https://charts.example.com", EnableOci: false, Creds: HelmCreds{Username: "user", Password: "pass"}},
				{Repo: "other.io/repo", EnableOci: true, Creds: HelmCreds{}},
			},
			expectedLogins: []string{"example.com"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var loggedInRegistries []string
			c, err := newCmdWithVersion(".", false, "", "", func(cmd *exec.Cmd, _ func(string) string) (string, error) {
				// capture the registry host from the login command args
				if len(cmd.Args) >= 3 && cmd.Args[1] == "registry" && cmd.Args[2] == "login" {
					loggedInRegistries = append(loggedInRegistries, cmd.Args[3])
				}
				return "", nil
			})
			require.NoError(t, err)

			h := &helm{cmd: *c, repos: tc.repos}
			err = h.RegistryLoginOCI(t.Context())
			require.NoError(t, err)
			assert.Equal(t, tc.expectedLogins, loggedInRegistries)
		})
	}
}

func TestRegistryLoginOCI_UsesProvidedContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	c, err := newCmdWithVersion(".", false, "", "", func(cmd *exec.Cmd, _ func(string) string) (string, error) {
		return "", cmd.Run()
	})
	require.NoError(t, err)

	h := &helm{cmd: *c, repos: []HelmRepository{{Repo: "example.com/myrepo", EnableOci: true, Creds: HelmCreds{Username: "user", Password: "pass"}}}}
	err = h.RegistryLoginOCI(ctx)
	require.ErrorIs(t, err, context.Canceled)
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
