package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func findParameter(params []*argoappv1.ComponentParameter, name string) *argoappv1.ComponentParameter {
	for _, param := range params {
		if param.Name == name {
			return param
		}
	}
	return nil
}

func TestHelmTemplateParams(t *testing.T) {
	h := NewHelmApp("./testdata/minio")
	overrides := []*argoappv1.ComponentParameter{
		{
			Name:  "service.type",
			Value: "LoadBalancer",
		},
		{
			Name:  "service.port",
			Value: "1234",
		},
	}
	objs, err := h.Template("test", nil, overrides)
	assert.Nil(t, err)
	assert.Equal(t, 5, len(objs))

	for _, obj := range objs {
		if obj.GetKind() == "Service" && obj.GetName() == "test-minio" {
			var svc apiv1.Service
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &svc)
			assert.Nil(t, err)
			assert.Equal(t, apiv1.ServiceTypeLoadBalancer, svc.Spec.Type)
			assert.Equal(t, int32(1234), svc.Spec.Ports[0].TargetPort.IntVal)
		}
	}
}

func TestHelmTemplateValues(t *testing.T) {
	h := NewHelmApp("./testdata/redis")
	valuesFiles := []string{"values-production.yaml"}
	objs, err := h.Template("test", valuesFiles, nil)
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
	h := NewHelmApp("./testdata/redis")
	params, err := h.GetParameters([]string{})
	assert.Nil(t, err)

	slaveCountParam := findParameter(params, "cluster.slaveCount")
	assert.NotNil(t, slaveCountParam)
	assert.Equal(t, slaveCountParam.Value, "1")
}

func TestHelmGetParamsValueFiles(t *testing.T) {
	h := NewHelmApp("./testdata/redis")
	params, err := h.GetParameters([]string{"values-production.yaml"})
	assert.Nil(t, err)

	slaveCountParam := findParameter(params, "cluster.slaveCount")
	assert.NotNil(t, slaveCountParam)
	assert.Equal(t, slaveCountParam.Value, "3")
}
