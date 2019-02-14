package argo

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/settings"
)

func TestNormalizeObjectWithMatchedGroupKind(t *testing.T) {
	normalizer, err := NewDiffNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:         "apps",
		Kind:          "Deployment",
		Json6902Paths: []string{"/spec/template/spec/containers"},
	}}, make(map[string]settings.ResourceOverride))

	assert.Nil(t, err)

	deployment := kube.MustToUnstructured(test.DemoDeployment())

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.Nil(t, err)
	assert.True(t, has)

	deployment, err = normalizer.Normalize(deployment)
	assert.Nil(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.Nil(t, err)
	assert.False(t, has)
}

func TestNormalizeNoMatchedGroupKinds(t *testing.T) {
	normalizer, err := NewDiffNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:         "",
		Kind:          "Service",
		Json6902Paths: []string{"/spec"},
	}}, make(map[string]settings.ResourceOverride))

	assert.Nil(t, err)

	deployment := kube.MustToUnstructured(test.DemoDeployment())

	normalized, err := normalizer.Normalize(deployment)
	assert.Nil(t, err)

	assert.Equal(t, deployment.Object, normalized.Object)
}

func TestNormalizeMatchedResourceOverrides(t *testing.T) {
	normalizer, err := NewDiffNormalizer([]v1alpha1.ResourceIgnoreDifferences{}, map[string]settings.ResourceOverride{
		"apps/Deployment": {
			IgnoreDifferences: `json6902Paths: ["/spec/template/spec/containers"]`,
		},
	})

	assert.Nil(t, err)

	deployment := kube.MustToUnstructured(test.DemoDeployment())

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.Nil(t, err)
	assert.True(t, has)

	deployment, err = normalizer.Normalize(deployment)
	assert.Nil(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.Nil(t, err)
	assert.False(t, has)
}
