package normalizers

import (
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test"
)

func TestNormalizeObjectWithMatchedGroupKind(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:        "apps",
		Kind:         "Deployment",
		JSONPointers: []string{"/not-matching-path", "/spec/template/spec/containers"},
	}}, make(map[string]v1alpha1.ResourceOverride))

	assert.Nil(t, err)

	deployment := test.NewDeployment()

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.Nil(t, err)
	assert.True(t, has)

	err = normalizer.Normalize(deployment)
	assert.Nil(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.Nil(t, err)
	assert.False(t, has)

	err = normalizer.Normalize(nil)
	assert.Error(t, err)
}

func TestNormalizeNoMatchedGroupKinds(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:        "",
		Kind:         "Service",
		JSONPointers: []string{"/spec"},
	}}, make(map[string]v1alpha1.ResourceOverride))

	assert.Nil(t, err)

	deployment := test.NewDeployment()

	err = normalizer.Normalize(deployment)
	assert.Nil(t, err)

	_, hasSpec, err := unstructured.NestedMap(deployment.Object, "spec")
	assert.Nil(t, err)
	assert.True(t, hasSpec)
}

func TestNormalizeMatchedResourceOverrides(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{}, map[string]v1alpha1.ResourceOverride{
		"apps/Deployment": {
			IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{JSONPointers: []string{"/spec/template/spec/containers"}},
		},
	})

	assert.Nil(t, err)

	deployment := test.NewDeployment()

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
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: certificates.cert-manager.io
spec:
  conversion:
    strategy: None
  group: cert-manager.io
  names:
    kind: Certificate
    listKind: CertificateList
    plural: certificates
    shortNames:
    - cert
    - certs
    singular: certificate
  scope: Namespaced
  version: v1alpha1
  versions:
  - name: v1alpha1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          json:
            x-kubernetes-preserve-unknown-fields: true`

func TestNormalizeMissingJsonPointer(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{}, map[string]v1alpha1.ResourceOverride{
		"apps/Deployment": {
			IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{JSONPointers: []string{"/garbage"}},
		},
		"apiextensions.k8s.io/CustomResourceDefinition": {
			IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{JSONPointers: []string{"/spec/additionalPrinterColumns/0/priority"}},
		},
	})
	assert.NoError(t, err)

	deployment := test.NewDeployment()

	err = normalizer.Normalize(deployment)
	assert.NoError(t, err)

	crd := unstructured.Unstructured{}
	err = yaml.Unmarshal([]byte(testCRDYAML), &crd)
	assert.NoError(t, err)

	err = normalizer.Normalize(&crd)
	assert.NoError(t, err)
}

func TestNormalizeGlobMatch(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{}, map[string]v1alpha1.ResourceOverride{
		"*/*": {
			IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{JSONPointers: []string{"/spec/template/spec/containers"}},
		},
	})

	assert.Nil(t, err)

	deployment := test.NewDeployment()

	_, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.Nil(t, err)
	assert.True(t, has)

	err = normalizer.Normalize(deployment)
	assert.Nil(t, err)
	_, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	assert.Nil(t, err)
	assert.False(t, has)
}

func TestNormalizeJQPathExpression(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:             "apps",
		Kind:              "Deployment",
		JQPathExpressions: []string{".spec.template.spec.initContainers[] | select(.name == \"init-container-0\")"},
	}}, make(map[string]v1alpha1.ResourceOverride))

	assert.Nil(t, err)

	deployment := test.NewDeployment()

	var initContainers []interface{}
	initContainers = append(initContainers, map[string]interface{}{"name": "init-container-0"})
	initContainers = append(initContainers, map[string]interface{}{"name": "init-container-1"})
	err = unstructured.SetNestedSlice(deployment.Object, initContainers, "spec", "template", "spec", "initContainers")
	assert.Nil(t, err)

	actualInitContainers, has, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "initContainers")
	assert.Nil(t, err)
	assert.True(t, has)
	assert.Len(t, actualInitContainers, 2)

	err = normalizer.Normalize(deployment)
	assert.Nil(t, err)
	actualInitContainers, has, err = unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "initContainers")
	assert.Nil(t, err)
	assert.True(t, has)
	assert.Len(t, actualInitContainers, 1)

	actualInitContainerName, has, err := unstructured.NestedString(actualInitContainers[0].(map[string]interface{}), "name")
	assert.Nil(t, err)
	assert.True(t, has)
	assert.Equal(t, actualInitContainerName, "init-container-1")
}

func TestNormalizeIllegalJQPathExpression(t *testing.T) {
	_, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:             "apps",
		Kind:              "Deployment",
		JQPathExpressions: []string{".spec.template.spec.containers[] | select(.name == \"missing-quote)"},
		// JSONPointers: []string{"no-starting-slash"},
	}}, make(map[string]v1alpha1.ResourceOverride))

	assert.Error(t, err)
}

func TestNormalizeJQPathExpressionWithError(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:             "apps",
		Kind:              "Deployment",
		JQPathExpressions: []string{".spec.fakeField.foo[]"},
	}}, make(map[string]v1alpha1.ResourceOverride))

	assert.Nil(t, err)

	deployment := test.NewDeployment()
	originalDeployment, err := deployment.MarshalJSON()
	assert.Nil(t, err)

	err = normalizer.Normalize(deployment)
	assert.Nil(t, err)

	normalizedDeployment, err := deployment.MarshalJSON()
	assert.Nil(t, err)
	assert.Equal(t, originalDeployment, normalizedDeployment)
}
