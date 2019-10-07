package argo

import (
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util/kube"
)

func TestNormalizeObjectWithMatchedGroupKind(t *testing.T) {
	normalizer, err := NewDiffNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:        "apps",
		Kind:         "Deployment",
		JSONPointers: []string{"/not-matching-path", "/spec/template/spec/containers"},
	}}, make(map[string]v1alpha1.ResourceOverride))

	assert.Nil(t, err)

	deployment := kube.MustToUnstructured(test.DemoDeployment())

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.Nil(t, err)
	assert.True(t, has)

	err = normalizer.Normalize(deployment)
	assert.Nil(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.Nil(t, err)
	assert.False(t, has)
}

func TestNormalizeObjectWithMatchingConditions(t *testing.T) {
	normalizer, err := NewDiffNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:         "apps",
		Kind:          "Deployment",
		JSONPointers:  []string{"/not-matching-path", "/spec/template/spec/containers"},
		Conditions:    []string{"/spec/template is defined"},
		MatchStrategy: "all",
	}}, make(map[string]v1alpha1.ResourceOverride))

	assert.NoError(t, err)

	deployment := kube.MustToUnstructured(test.DemoDeployment())

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.True(t, has)

	err = normalizer.Normalize(deployment)
	assert.NoError(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.False(t, has)
}

func TestNormalizeObjectWithNonMatchingConditions(t *testing.T) {
	normalizer, err := NewDiffNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:         "apps",
		Kind:          "Deployment",
		JSONPointers:  []string{"/not-matching-path", "/spec/template/spec/containers"},
		Conditions:    []string{"/spec/template not defined"},
		MatchStrategy: "all",
	}}, make(map[string]v1alpha1.ResourceOverride))

	assert.NoError(t, err)

	deployment := kube.MustToUnstructured(test.DemoDeployment())

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.True(t, has)

	err = normalizer.Normalize(deployment)
	assert.NoError(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.True(t, has)
}

func TestNormalizeObjectWithMultipleMatchingConditionsStrategyAll(t *testing.T) {
	normalizer, err := NewDiffNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:         "apps",
		Kind:          "Deployment",
		JSONPointers:  []string{"/not-matching-path", "/spec/template/spec/containers"},
		Conditions:    []string{"/spec/template is defined", "/spec/template/metadata/labels/app is defined"},
		MatchStrategy: "all",
	}}, make(map[string]v1alpha1.ResourceOverride))

	assert.NoError(t, err)

	deployment := kube.MustToUnstructured(test.DemoDeployment())

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.True(t, has)

	err = normalizer.Normalize(deployment)
	assert.NoError(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.False(t, has)
}

func TestNormalizeObjectWithMultipleNonMatchingConditionsStrategyAll(t *testing.T) {
	normalizer, err := NewDiffNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:         "apps",
		Kind:          "Deployment",
		JSONPointers:  []string{"/not-matching-path", "/spec/template/spec/containers"},
		Conditions:    []string{"/spec/template is defined", "/spec/template/metadata/labels/app not defined"},
		MatchStrategy: "all",
	}}, make(map[string]v1alpha1.ResourceOverride))

	assert.NoError(t, err)

	deployment := kube.MustToUnstructured(test.DemoDeployment())

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.True(t, has)

	err = normalizer.Normalize(deployment)
	assert.NoError(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.True(t, has)
}

func TestNormalizeObjectWithMultipleMatchingConditionsStrategyAny(t *testing.T) {
	normalizer, err := NewDiffNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:         "apps",
		Kind:          "Deployment",
		JSONPointers:  []string{"/not-matching-path", "/spec/template/spec/containers"},
		Conditions:    []string{"/spec/template is defined", "/spec/foo is defined"},
		MatchStrategy: "any",
	}}, make(map[string]v1alpha1.ResourceOverride))

	assert.NoError(t, err)

	deployment := kube.MustToUnstructured(test.DemoDeployment())

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.True(t, has)

	err = normalizer.Normalize(deployment)
	assert.NoError(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.False(t, has)
}

func TestNormalizeObjectWithMultipleNonMatchingConditionsStrategyAny(t *testing.T) {
	normalizer, err := NewDiffNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:         "apps",
		Kind:          "Deployment",
		JSONPointers:  []string{"/not-matching-path", "/spec/template/spec/containers"},
		Conditions:    []string{"/spec/foo is defined", "/spec/template/metadata/labels/app not defined"},
		MatchStrategy: "any",
	}}, make(map[string]v1alpha1.ResourceOverride))

	assert.NoError(t, err)

	deployment := kube.MustToUnstructured(test.DemoDeployment())

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.True(t, has)

	err = normalizer.Normalize(deployment)
	assert.NoError(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.True(t, has)
}

func TestNormalizeObjectWithMultipleMatchingConditionsStrategyNone(t *testing.T) {
	normalizer, err := NewDiffNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:         "apps",
		Kind:          "Deployment",
		JSONPointers:  []string{"/not-matching-path", "/spec/template/spec/containers"},
		Conditions:    []string{"/spec/template is defined", "/spec/template/metadata/labels/app not defined"},
		MatchStrategy: "none",
	}}, make(map[string]v1alpha1.ResourceOverride))

	assert.NoError(t, err)

	deployment := kube.MustToUnstructured(test.DemoDeployment())

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.True(t, has)

	err = normalizer.Normalize(deployment)
	assert.NoError(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.True(t, has)
}

func TestNormalizeObjectWithMultipleNonMatchingConditionsStrategyNone(t *testing.T) {
	normalizer, err := NewDiffNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:         "apps",
		Kind:          "Deployment",
		JSONPointers:  []string{"/not-matching-path", "/spec/template/spec/containers"},
		Conditions:    []string{"/spec/foo is defined", "/spec/template/metadata/labels/app not defined"},
		MatchStrategy: "none",
	}}, make(map[string]v1alpha1.ResourceOverride))

	assert.NoError(t, err)

	deployment := kube.MustToUnstructured(test.DemoDeployment())

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.True(t, has)

	err = normalizer.Normalize(deployment)
	assert.NoError(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.False(t, has)
}

func TestNormalizeObjectWithInvalidMatchingConditions(t *testing.T) {
	normalizer, err := NewDiffNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:         "apps",
		Kind:          "Deployment",
		JSONPointers:  []string{"/not-matching-path", "/spec/template/spec/containers"},
		Conditions:    []string{"/spec/foo are defined", "/spec/template/metadata/labels/app not defined"},
		MatchStrategy: "none",
	}}, make(map[string]v1alpha1.ResourceOverride))

	assert.NoError(t, err)

	deployment := kube.MustToUnstructured(test.DemoDeployment())

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.True(t, has)

	err = normalizer.Normalize(deployment)
	assert.NoError(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.False(t, has)
}

func TestNormalizeObjectWithInvalidMatchingSyntax(t *testing.T) {
	normalizer, err := NewDiffNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:         "apps",
		Kind:          "Deployment",
		JSONPointers:  []string{"/not-matching-path", "/spec/template/spec/containers"},
		Conditions:    []string{"/spec/foo is", "/spec/template/metadata/labels/app not defined"},
		MatchStrategy: "none",
	}}, make(map[string]v1alpha1.ResourceOverride))

	assert.NoError(t, err)

	deployment := kube.MustToUnstructured(test.DemoDeployment())

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.True(t, has)

	err = normalizer.Normalize(deployment)
	assert.NoError(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.NoError(t, err)
	assert.False(t, has)
}

func TestNormalizeNoMatchedGroupKinds(t *testing.T) {
	normalizer, err := NewDiffNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:        "",
		Kind:         "Service",
		JSONPointers: []string{"/spec"},
	}}, make(map[string]v1alpha1.ResourceOverride))

	assert.Nil(t, err)

	deployment := kube.MustToUnstructured(test.DemoDeployment())

	err = normalizer.Normalize(deployment)
	assert.Nil(t, err)

	_, hasSpec, err := unstructured.NestedMap(deployment.Object, "spec")
	assert.Nil(t, err)
	assert.True(t, hasSpec)
}

func TestNormalizeMatchedResourceOverrides(t *testing.T) {
	normalizer, err := NewDiffNormalizer([]v1alpha1.ResourceIgnoreDifferences{}, map[string]v1alpha1.ResourceOverride{
		"apps/Deployment": {
			IgnoreDifferences: `jsonPointers: ["/spec/template/spec/containers"]`,
		},
	})

	assert.Nil(t, err)

	deployment := kube.MustToUnstructured(test.DemoDeployment())

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.Nil(t, err)
	assert.True(t, has)

	err = normalizer.Normalize(deployment)
	assert.Nil(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.Nil(t, err)
	assert.False(t, has)
}

const testCRDYAML = `
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: certificates.certmanager.k8s.io
spec:
  group: certmanager.k8s.io
  names:
    kind: Certificate
    listKind: CertificateList
    plural: certificates
    shortNames:
      - cert
      - certs
    singular: certificate
  scope: Namespaced
  version: v1alpha1`

func TestNormalizeMissingJsonPointer(t *testing.T) {
	normalizer, err := NewDiffNormalizer([]v1alpha1.ResourceIgnoreDifferences{}, map[string]v1alpha1.ResourceOverride{
		"apps/Deployment": {
			IgnoreDifferences: `jsonPointers: ["/garbage"]`,
		},
		"apiextensions.k8s.io/CustomResourceDefinition": {
			IgnoreDifferences: `jsonPointers: ["/spec/additionalPrinterColumns/0/priority"]`,
		},
	})
	assert.NoError(t, err)

	deployment := kube.MustToUnstructured(test.DemoDeployment())

	err = normalizer.Normalize(deployment)
	assert.NoError(t, err)

	crd := unstructured.Unstructured{}
	err = yaml.Unmarshal([]byte(testCRDYAML), &crd)
	assert.NoError(t, err)

	err = normalizer.Normalize(&crd)
	assert.NoError(t, err)
}
