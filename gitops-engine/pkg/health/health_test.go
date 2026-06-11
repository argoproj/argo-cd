/*
Package provides functionality that allows assessing the health state of a Kubernetes resource.
*/

package health

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestIsWorse(t *testing.T) {
	assert.True(t, IsWorse(HealthStatusHealthy, HealthStatusProgressing))
	assert.True(t, IsWorse(HealthStatusHealthy, HealthStatusDegraded))
	assert.False(t, IsWorse(HealthStatusDegraded, HealthStatusProgressing))
}

func TestPendingDeletionHealth(t *testing.T) {
	yamlBytes, err := os.ReadFile("../../../resource_customizations/Pod/testdata/pod-deletion.yaml")
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)

	health, err := GetResourceHealth(&obj, nil)
	require.NoError(t, err)
	require.NotNil(t, health)
	assert.Equal(t, HealthStatusProgressing, health.Status)
	assert.Equal(t, "Pending deletion", health.Message)
}

func TestNoHealthOverrideReturnsNil(t *testing.T) {
	yamlBytes, err := os.ReadFile("../../../resource_customizations/Service/testdata/svc-clusterip.yaml")
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)

	health, err := GetResourceHealth(&obj, nil)
	require.NoError(t, err)
	assert.Nil(t, health)
}
