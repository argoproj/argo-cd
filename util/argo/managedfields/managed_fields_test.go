package managedfields_test

import (
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/util/argo/managedfields"
	"github.com/argoproj/argo-cd/v2/util/argo/testdata"
)

func TestNormalize(t *testing.T) {
	t.Run("will remove conflicting fields if managed by trusted managers", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := StrToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml)
		trustedManagers := []string{"kube-controller-manager", "revision-history-manager"}

		// when
		liveResult, desiredResult, err := managedfields.Normalize(liveState, desiredState, trustedManagers)

		// then
		assert.NoError(t, err)
		desiredReplicas, ok, err := unstructured.NestedFloat64(desiredResult.Object, "spec", "replicas")
		assert.False(t, ok)
		assert.NoError(t, err)
		liveReplicas, ok, err := unstructured.NestedFloat64(liveResult.Object, "spec", "replicas")
		assert.False(t, ok)
		assert.NoError(t, err)
		assert.Equal(t, liveReplicas, desiredReplicas)
		liveRevisionHistory, ok, err := unstructured.NestedFloat64(liveResult.Object, "spec", "revisionHistoryLimit")
		assert.False(t, ok)
		assert.NoError(t, err)
		desiredRevisionHistory, ok, err := unstructured.NestedFloat64(desiredResult.Object, "spec", "revisionHistoryLimit")
		assert.False(t, ok)
		assert.NoError(t, err)
		assert.Equal(t, liveRevisionHistory, desiredRevisionHistory)

	})
	t.Run("will keep conflicting fields if not from trusted manager", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := StrToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml)
		trustedManagers := []string{"another-manager"}

		// when
		liveResult, desiredResult, err := managedfields.Normalize(liveState, desiredState, trustedManagers)

		// then
		assert.NoError(t, err)
		validateNestedFloat64(t, float64(3), desiredResult, "spec", "replicas")
		validateNestedFloat64(t, float64(1), desiredResult, "spec", "revisionHistoryLimit")
		validateNestedFloat64(t, float64(2), liveResult, "spec", "replicas")
		validateNestedFloat64(t, float64(3), liveResult, "spec", "revisionHistoryLimit")
	})
	t.Run("no-op if live state is nil", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		trustedManagers := []string{"kube-controller-manager"}

		// when
		liveResult, desiredResult, err := managedfields.Normalize(nil, desiredState, trustedManagers)

		// then
		assert.NoError(t, err)
		assert.Nil(t, liveResult)
		assert.Nil(t, desiredResult)
		validateNestedFloat64(t, float64(3), desiredState, "spec", "replicas")
		validateNestedFloat64(t, float64(1), desiredState, "spec", "revisionHistoryLimit")

	})
	t.Run("no-op if desired state is nil", func(t *testing.T) {
		// given
		liveState := StrToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml)
		trustedManagers := []string{"kube-controller-manager"}

		// when
		liveResult, desiredResult, err := managedfields.Normalize(liveState, nil, trustedManagers)

		// then
		assert.NoError(t, err)
		assert.Nil(t, liveResult)
		assert.Nil(t, desiredResult)
		validateNestedFloat64(t, float64(2), liveState, "spec", "replicas")
		validateNestedFloat64(t, float64(3), liveState, "spec", "revisionHistoryLimit")
	})
	t.Run("no-op if trusted manager list is empty", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := StrToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml)

		// when
		liveResult, desiredResult, err := managedfields.Normalize(liveState, desiredState, []string{})

		// then
		assert.NoError(t, err)
		assert.Nil(t, liveResult)
		assert.Nil(t, desiredResult)
		validateNestedFloat64(t, float64(3), desiredState, "spec", "replicas")
		validateNestedFloat64(t, float64(1), desiredState, "spec", "revisionHistoryLimit")
		validateNestedFloat64(t, float64(2), liveState, "spec", "replicas")
		validateNestedFloat64(t, float64(3), liveState, "spec", "revisionHistoryLimit")
	})
}

func validateNestedFloat64(t *testing.T, expected float64, obj *unstructured.Unstructured, fields ...string) {
	t.Helper()
	current := getNestedFloat64(t, obj, fields...)
	assert.Equal(t, expected, current)
}

func getNestedFloat64(t *testing.T, obj *unstructured.Unstructured, fields ...string) float64 {
	t.Helper()
	current, ok, err := unstructured.NestedFloat64(obj.Object, fields...)
	assert.True(t, ok, "nested field not found")
	assert.NoError(t, err)
	return current
}

func StrToUnstructured(jsonStr string) *unstructured.Unstructured {
	obj := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(jsonStr), &obj)
	if err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{Object: obj}
}
