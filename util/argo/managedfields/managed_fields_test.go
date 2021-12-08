package managedfields_test

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/util/argo/managedfields"
	"github.com/argoproj/argo-cd/v2/util/argo/testdata"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNormalize(t *testing.T) {
	t.Run("will remove replicas field if managed by trusted manager", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := StrToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml)
		trustedManager := "kube-controller-manager"

		// when
		err := managedfields.Normalize(liveState, desiredState, []string{trustedManager})

		// then
		assert.NoError(t, err)
		desiredReplicas, ok, err := unstructured.NestedFloat64(desiredState.Object, "spec", "replicas")
		assert.False(t, ok)
		assert.NoError(t, err)
		liveReplicas, ok, err := unstructured.NestedFloat64(liveState.Object, "spec", "replicas")
		assert.False(t, ok)
		assert.NoError(t, err)
		assert.Equal(t, liveReplicas, desiredReplicas)
	})
	t.Run("will keep conflicting fields if not trusted manager", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := StrToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml)

		// when
		err := managedfields.Normalize(liveState, desiredState, []string{})

		// then
		assert.NoError(t, err)
		desiredReplicas, ok, err := unstructured.NestedFloat64(desiredState.Object, "spec", "replicas")
		assert.True(t, ok)
		assert.NoError(t, err)
		liveReplicas, ok, err := unstructured.NestedFloat64(liveState.Object, "spec", "replicas")
		assert.True(t, ok)
		assert.NoError(t, err)
		assert.Equal(t, float64(1), desiredReplicas)
		assert.Equal(t, float64(2), liveReplicas)
	})

}

func StrToUnstructured(jsonStr string) *unstructured.Unstructured {
	obj := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(jsonStr), &obj)
	if err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{Object: obj}
}
