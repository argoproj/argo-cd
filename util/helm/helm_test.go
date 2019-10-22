package helm

import (
	"os"
	"regexp"
	"testing"

	"github.com/argoproj/argo-cd/util/kube"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func template(h Helm, opts *TemplateOpts) ([]*unstructured.Unstructured, error) {
	out, err := h.Template(opts)
	if err != nil {
		return nil, err
	}
	return kube.SplitYAML(out)
}

func TestHelmTemplateParams(t *testing.T) {
	h, err := NewHelmApp("./testdata/minio", []HelmRepository{})
	assert.NoError(t, err)
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
	assert.Nil(t, err)
	assert.Equal(t, 5, len(objs))

	for _, obj := range objs {
		if obj.GetKind() == "Service" && obj.GetName() == "test-minio" {
			var svc apiv1.Service
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &svc)
			assert.Nil(t, err)
			assert.Equal(t, apiv1.ServiceTypeLoadBalancer, svc.Spec.Type)
			assert.Equal(t, int32(1234), svc.Spec.Ports[0].TargetPort.IntVal)
			assert.Equal(t, "true", svc.ObjectMeta.Annotations["prometheus.io/scrape"])
		}
	}
}

func TestHelmTemplateValues(t *testing.T) {
	h, err := NewHelmApp("./testdata/redis", []HelmRepository{})
	assert.NoError(t, err)
	opts := TemplateOpts{
		Name:   "test",
		Values: []string{"values-production.yaml"},
	}
	objs, err := template(h, &opts)
	assert.Nil(t, err)
	assert.Equal(t, 8, len(objs))

	for _, obj := range objs {
		if obj.GetKind() == "Deployment" && obj.GetName() == "test-redis-slave" {
			var dep appsv1.Deployment
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &dep)
			assert.Nil(t, err)
			assert.Equal(t, int32(3), *dep.Spec.Replicas)
		}
	}
}

func TestHelmGetParams(t *testing.T) {
	h, err := NewHelmApp("./testdata/redis", nil)
	assert.NoError(t, err)
	params, err := h.GetParameters([]string{})
	assert.Nil(t, err)

	slaveCountParam := params["cluster.slaveCount"]
	assert.Equal(t, slaveCountParam, "1")
}

func TestHelmGetParamsValueFiles(t *testing.T) {
	h, err := NewHelmApp("./testdata/redis", nil)
	assert.NoError(t, err)
	params, err := h.GetParameters([]string{"values-production.yaml"})
	assert.Nil(t, err)

	slaveCountParam := params["cluster.slaveCount"]
	assert.Equal(t, slaveCountParam, "3")
}

func TestHelmDependencyBuild(t *testing.T) {
	clean := func() {
		_ = os.RemoveAll("./testdata/wordpress/charts")
	}
	clean()
	defer clean()
	h, err := NewHelmApp("./testdata/wordpress", nil)
	assert.NoError(t, err)
	err = h.Init()
	assert.NoError(t, err)
	_, err = h.Template(&TemplateOpts{Name: "wordpress"})
	assert.Error(t, err)
	err = h.DependencyBuild()
	assert.NoError(t, err)
	_, err = h.Template(&TemplateOpts{Name: "wordpress"})
	assert.NoError(t, err)
}

func TestHelmTemplateReleaseNameOverwrite(t *testing.T) {
	h, err := NewHelmApp("./testdata/redis", nil)
	assert.NoError(t, err)

	objs, err := template(h, &TemplateOpts{Name: "my-release"})
	assert.Nil(t, err)
	assert.Equal(t, 5, len(objs))

	for _, obj := range objs {
		if obj.GetKind() == "StatefulSet" {
			var stateful appsv1.StatefulSet
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &stateful)
			assert.Nil(t, err)
			assert.Equal(t, "my-release-redis-master", stateful.ObjectMeta.Name)
		}
	}
}

func TestHelmTemplateReleaseName(t *testing.T) {
	h, err := NewHelmApp("./testdata/redis", nil)
	assert.NoError(t, err)
	objs, err := template(h, &TemplateOpts{Name: "test"})
	assert.Nil(t, err)
	assert.Equal(t, 5, len(objs))

	for _, obj := range objs {
		if obj.GetKind() == "StatefulSet" {
			var stateful appsv1.StatefulSet
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &stateful)
			assert.Nil(t, err)
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
	} {
		cleaned := cleanSetParameters(input)
		assert.Equal(t, expected, cleaned)
	}
}

func TestVersion(t *testing.T) {
	ver, err := Version()
	assert.NoError(t, err)
	SemverRegexValidation := `^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-(0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(\.(0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*)?(\+[0-9a-zA-Z-]+(\.[0-9a-zA-Z-]+)*)?$`
	re := regexp.MustCompile(SemverRegexValidation)
	assert.True(t, re.MatchString(ver))
}

func Test_flatVals(t *testing.T) {
	t.Run("Map", func(t *testing.T) {
		output := map[string]string{}

		flatVals(map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}}, output)

		assert.Equal(t, map[string]string{"foo.bar": "baz"}, output)
	})
	t.Run("Array", func(t *testing.T) {
		output := map[string]string{}

		flatVals(map[string]interface{}{"foo": []interface{}{"bar"}}, output)

		assert.Equal(t, map[string]string{"foo[0]": "bar"}, output)
	})
	t.Run("Val", func(t *testing.T) {
		output := map[string]string{}

		flatVals(map[string]interface{}{"foo": 1}, output)

		assert.Equal(t, map[string]string{"foo": "1"}, output)
	})
}
