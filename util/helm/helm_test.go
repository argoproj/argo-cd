package helm

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/util/io/path"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
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
	h, err := NewHelmApp("./testdata/minio", []HelmRepository{}, false, "", "", "", false)
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
		if obj.GetKind() == "Service" && obj.GetName() == "test-minio" {
			var svc apiv1.Service
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &svc)
			require.NoError(t, err)
			assert.Equal(t, apiv1.ServiceTypeLoadBalancer, svc.Spec.Type)
			assert.Equal(t, int32(1234), svc.Spec.Ports[0].TargetPort.IntVal)
			assert.Equal(t, "true", svc.ObjectMeta.Annotations["prometheus.io/scrape"])
		}
	}
}

func TestHelmTemplateValues(t *testing.T) {
	repoRoot := "./testdata/redis"
	repoRootAbs, err := filepath.Abs(repoRoot)
	require.NoError(t, err)
	h, err := NewHelmApp(repoRootAbs, []HelmRepository{}, false, "", "", "", false)
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
	repoRoot := "./testdata/redis"
	repoRootAbs, err := filepath.Abs(repoRoot)
	require.NoError(t, err)
	h, err := NewHelmApp(repoRootAbs, nil, false, "", "", "", false)
	require.NoError(t, err)
	params, err := h.GetParameters(nil, repoRootAbs, repoRootAbs)
	require.NoError(t, err)

	slaveCountParam := params["cluster.slaveCount"]
	assert.Equal(t, "1", slaveCountParam)
}

func TestHelmGetParamsValueFiles(t *testing.T) {
	repoRoot := "./testdata/redis"
	repoRootAbs, err := filepath.Abs(repoRoot)
	require.NoError(t, err)
	h, err := NewHelmApp(repoRootAbs, nil, false, "", "", "", false)
	require.NoError(t, err)
	valuesPath, _, err := path.ResolveValueFilePathOrUrl(repoRootAbs, repoRootAbs, "values-production.yaml", nil)
	require.NoError(t, err)
	params, err := h.GetParameters([]path.ResolvedFilePath{valuesPath}, repoRootAbs, repoRootAbs)
	require.NoError(t, err)

	slaveCountParam := params["cluster.slaveCount"]
	assert.Equal(t, "3", slaveCountParam)
}

func TestHelmGetParamsValueFilesThatExist(t *testing.T) {
	repoRoot := "./testdata/redis"
	repoRootAbs, err := filepath.Abs(repoRoot)
	require.NoError(t, err)
	h, err := NewHelmApp(repoRootAbs, nil, false, "", "", "", false)
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
	h, err := NewHelmApp("./testdata/redis", nil, false, "", "", "", false)
	require.NoError(t, err)

	objs, err := template(h, &TemplateOpts{Name: "my-release"})
	require.NoError(t, err)
	assert.Len(t, objs, 5)

	for _, obj := range objs {
		if obj.GetKind() == "StatefulSet" {
			var stateful appsv1.StatefulSet
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &stateful)
			require.NoError(t, err)
			assert.Equal(t, "my-release-redis-master", stateful.ObjectMeta.Name)
		}
	}
}

func TestHelmTemplateReleaseName(t *testing.T) {
	h, err := NewHelmApp("./testdata/redis", nil, false, "", "", "", false)
	require.NoError(t, err)
	objs, err := template(h, &TemplateOpts{Name: "test"})
	require.NoError(t, err)
	assert.Len(t, objs, 5)

	for _, obj := range objs {
		if obj.GetKind() == "StatefulSet" {
			var stateful appsv1.StatefulSet
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &stateful)
			require.NoError(t, err)
			assert.Equal(t, "test-redis-master", stateful.ObjectMeta.Name)
		}
	}
}

func TestHelmArgCleaner(t *testing.T) {
	for input, expected := range map[string]string{
		`val`:        `val`,
		`bar`:        `bar`,
		`not, clean`: `not\, clean`,
		`a\,b,c`:     `a\,b\,c`,
		`{a,b,c}`:    `{a,b,c}`,
	} {
		cleaned := cleanSetParameters(input)
		assert.Equal(t, expected, cleaned)
	}
}

func TestVersion(t *testing.T) {
	ver, err := Version(false)
	require.NoError(t, err)
	assert.NotEmpty(t, ver)
}

func Test_flatVals(t *testing.T) {
	t.Run("Map", func(t *testing.T) {
		output := map[string]string{}

		flatVals(map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}}, output)

		assert.Equal(t, map[string]string{"foo.bar": "baz"}, output)
	})
	t.Run("Array", func(t *testing.T) {
		output := map[string]string{}

		flatVals(map[string]interface{}{"foo": []interface{}{"bar", "baz"}}, output)

		assert.Equal(t, map[string]string{"foo[0]": "bar", "foo[1]": "baz"}, output)
	})
	t.Run("Val", func(t *testing.T) {
		output := map[string]string{}

		flatVals(map[string]interface{}{"foo": 1}, output)

		assert.Equal(t, map[string]string{"foo": "1"}, output)
	})
}

func TestAPIVersions(t *testing.T) {
	h, err := NewHelmApp("./testdata/api-versions", nil, false, "", "", "", false)
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

func TestSkipCrds(t *testing.T) {
	h, err := NewHelmApp("./testdata/crds", nil, false, "", "", "", false)
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
