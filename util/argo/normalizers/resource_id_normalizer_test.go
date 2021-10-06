package normalizers

import (
	"io/ioutil"
	"testing"

	"github.com/argoproj/argo-cd/v2/util/kube"

	"github.com/argoproj/argo-cd/v2/util/resource_tracking"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/common"
)

func TestResourceIdNormalizer_Normalize(t *testing.T) {
	yamlBytes, err := ioutil.ReadFile("testdata/svc.yaml")
	assert.Nil(t, err)
	var obj *unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)

	resourceTracking := resource_tracking.NewResourceTracking()

	err = resourceTracking.SetAppInstance(obj, common.LabelKeyAppInstance, "my-app", "", resource_tracking.TrackingMethodLabel)

	yamlBytes, err = ioutil.ReadFile("testdata/svc.yaml")
	assert.Nil(t, err)
	var obj2 *unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj2)
	assert.Nil(t, err)

	err = resourceTracking.SetAppInstance(obj2, common.AnnotationKeyAppInstance, "my-app2", "", resource_tracking.TrackingMethodAnnotation)

	normalizer, _ := NewResourceIdNormalizer("annotation")

	_ = normalizer.Normalize(obj2, nil, obj)

	annotation := kube.GetAppInstanceAnnotation(obj2, common.AnnotationKeyAppInstance)

	assert.Equal(t, obj.GetAnnotations()[common.AnnotationKeyAppInstance], annotation)
}
