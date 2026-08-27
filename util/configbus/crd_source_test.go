package configbus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestStaticCRDSource_Absent(t *testing.T) {
	src := StaticCRDSource{}
	assert.False(t, src.HasReconciliationTimeout())
	assert.Equal(t, time.Duration(0), src.ReconciliationTimeout())
	assert.False(t, src.HasResourceOverrides())
	overrides, err := src.ResourceOverrides()
	require.NoError(t, err)
	assert.Nil(t, overrides)
}

func TestStaticCRDSource_ReconciliationTimeout(t *testing.T) {
	src := StaticCRDSource{Object: &appv1.ArgoCDConfiguration{
		Spec: appv1.ArgoCDConfigurationSpec{
			Controller: &appv1.ControllerConfig{
				Reconciliation: &appv1.ReconciliationConfig{
					Timeout: &metav1.Duration{Duration: 2 * time.Minute},
				},
			},
		},
	}}
	assert.True(t, src.HasReconciliationTimeout())
	assert.Equal(t, 2*time.Minute, src.ReconciliationTimeout())
	assert.False(t, src.HasResourceOverrides())
}

func TestStaticCRDSource_ResourceOverrides(t *testing.T) {
	src := StaticCRDSource{Object: &appv1.ArgoCDConfiguration{
		Spec: appv1.ArgoCDConfigurationSpec{
			Controller: &appv1.ControllerConfig{
				Resource: &appv1.ResourceConfig{
					Health: []appv1.ResourceHealthCustomization{{
						Group: "apps", Kind: "Deployment", HealthLua: "return {status='Healthy'}", UseOpenLibs: true,
					}},
					Actions: []appv1.ResourceActionsCustomization{{
						Group: "apps", Kind: "Deployment", DiscoveryLua: "return {}",
						Definitions: []appv1.ResourceActionDefinition{{Name: "restart", ActionLua: "return obj"}},
					}},
					IgnoreDifferences: []appv1.ResourceIgnoreCustomization{{
						Group: "", Kind: "ConfigMap", JSONPointers: []string{"/data"},
					}},
					KnownTypeFields: []appv1.ResourceKnownTypesCustomization{{
						Group: "apps", Kind: "Deployment",
						Fields: []appv1.KnownTypeField{{Field: "spec.template.spec", Type: "core/v1/PodSpec"}},
					}},
				},
			},
		},
	}}
	assert.True(t, src.HasResourceOverrides())
	overrides, err := src.ResourceOverrides()
	require.NoError(t, err)
	require.Contains(t, overrides, "apps/Deployment")
	dep := overrides["apps/Deployment"]
	assert.Equal(t, "return {status='Healthy'}", dep.HealthLua)
	assert.True(t, dep.UseOpenLibs)
	assert.Contains(t, dep.Actions, "discovery.lua")
	assert.Contains(t, dep.Actions, "restart")
	assert.Len(t, dep.KnownTypeFields, 1)
	assert.Equal(t, "spec.template.spec", dep.KnownTypeFields[0].Field)

	require.Contains(t, overrides, "ConfigMap")
	assert.Equal(t, []string{"/data"}, overrides["ConfigMap"].IgnoreDifferences.JSONPointers)
}

func TestStaticCRDSource_EmptyResourceSection(t *testing.T) {
	src := StaticCRDSource{Object: &appv1.ArgoCDConfiguration{
		Spec: appv1.ArgoCDConfigurationSpec{
			Controller: &appv1.ControllerConfig{
				Resource: &appv1.ResourceConfig{},
			},
		},
	}}
	assert.False(t, src.HasResourceOverrides())
}

func TestResourceOverrideKey(t *testing.T) {
	assert.Equal(t, "*/*", resourceOverrideKey("*", "*"))
	assert.Equal(t, "ConfigMap", resourceOverrideKey("", "ConfigMap"))
	assert.Equal(t, "apps/Deployment", resourceOverrideKey("apps", "Deployment"))
}
