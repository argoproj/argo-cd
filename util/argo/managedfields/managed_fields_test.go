package managedfields_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	arv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/util/argo/managedfields"
	"github.com/argoproj/argo-cd/v2/util/argo/testdata"
	"github.com/argoproj/gitops-engine/pkg/utils/kube/scheme"
)

func TestNormalize(t *testing.T) {

	parser := scheme.StaticParser()
	t.Run("will remove conflicting fields if managed by trusted managers", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := StrToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml)
		trustedManagers := []string{"kube-controller-manager", "revision-history-manager"}
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		liveResult, desiredResult, err := managedfields.Normalize(liveState, desiredState, trustedManagers, &pt)

		// then
		require.NoError(t, err)
		require.NotNil(t, liveResult)
		require.NotNil(t, desiredResult)
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
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		liveResult, desiredResult, err := managedfields.Normalize(liveState, desiredState, trustedManagers, &pt)

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
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		liveResult, desiredResult, err := managedfields.Normalize(nil, desiredState, trustedManagers, &pt)

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
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		liveResult, desiredResult, err := managedfields.Normalize(liveState, nil, trustedManagers, &pt)

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
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		liveResult, desiredResult, err := managedfields.Normalize(liveState, desiredState, []string{}, &pt)

		// then
		assert.NoError(t, err)
		assert.Nil(t, liveResult)
		assert.Nil(t, desiredResult)
		validateNestedFloat64(t, float64(3), desiredState, "spec", "replicas")
		validateNestedFloat64(t, float64(1), desiredState, "spec", "revisionHistoryLimit")
		validateNestedFloat64(t, float64(2), liveState, "spec", "replicas")
		validateNestedFloat64(t, float64(3), liveState, "spec", "revisionHistoryLimit")
	})
	t.Run("will normalize successfully inside a list", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredValidatingWebhookYaml)
		liveState := StrToUnstructured(testdata.LiveValidatingWebhookYaml)
		trustedManagers := []string{"external-secrets"}
		pt := parser.Type("io.k8s.api.admissionregistration.v1.ValidatingWebhookConfiguration")

		// when
		liveResult, desiredResult, err := managedfields.Normalize(liveState, desiredState, trustedManagers, &pt)

		// then
		require.NoError(t, err)
		require.NotNil(t, liveResult)
		require.NotNil(t, desiredResult)

		var vwcLive arv1.ValidatingWebhookConfiguration
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(liveResult.Object, &vwcLive)
		require.NoError(t, err)
		assert.Equal(t, 1, len(vwcLive.Webhooks))
		assert.Equal(t, "", string(vwcLive.Webhooks[0].ClientConfig.CABundle))

		var vwcConfig arv1.ValidatingWebhookConfiguration
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(desiredResult.Object, &vwcConfig)
		require.NoError(t, err)
		assert.Equal(t, 1, len(vwcConfig.Webhooks))
		assert.Equal(t, "", string(vwcConfig.Webhooks[0].ClientConfig.CABundle))
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
