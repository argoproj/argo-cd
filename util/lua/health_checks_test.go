package lua

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestEnumerateHealthChecks_Builtins(t *testing.T) {
	defs, err := EnumerateHealthChecks(ResourceHealthOverrides{})
	require.NoError(t, err)
	assert.NotEmpty(t, defs)

	// Check Go built-ins present
	foundDeployment := false
	foundService := false
	for _, def := range defs {
		if def.Key == "apps/Deployment" {
			foundDeployment = true
			assert.Equal(t, HealthCheckOriginBuiltinGo, def.Origin)
			assert.False(t, def.IsWildcard)
			assert.Empty(t, def.LuaScript)
		}
		if def.Key == "Service" {
			foundService = true
			assert.Equal(t, HealthCheckOriginBuiltinGo, def.Origin)
			assert.Empty(t, def.Group)
			assert.Equal(t, "Service", def.Kind)
		}
	}
	assert.True(t, foundDeployment, "expected apps/Deployment in built-in Go health checks")
	assert.True(t, foundService, "expected Service in built-in Go health checks")
}

func TestEnumerateHealthChecks_EmbeddedLua(t *testing.T) {
	defs, err := EnumerateHealthChecks(ResourceHealthOverrides{})
	require.NoError(t, err)

	foundRollout := false
	foundWildcard := false

	for _, def := range defs {
		if def.Key == "argoproj.io/Rollout" {
			foundRollout = true
			assert.Equal(t, HealthCheckOriginBuiltinLua, def.Origin)
			assert.NotEmpty(t, def.LuaScript)
			assert.True(t, def.UseOpenLibs)
			assert.False(t, def.IsWildcard)
		}
		if def.IsWildcard && def.Origin == HealthCheckOriginBuiltinLua {
			foundWildcard = true
			assert.Contains(t, def.Key, "*")
		}
	}

	assert.True(t, foundRollout, "expected argoproj.io/Rollout in embedded Lua checks")
	assert.True(t, foundWildcard, "expected wildcard embedded Lua check")
}

func TestEnumerateHealthChecks_CustomAndOverride(t *testing.T) {
	overrides := ResourceHealthOverrides{
		// Custom check for unknown GVK
		"custom.io/Widget": appv1.ResourceOverride{
			HealthLua:   "hs = {status = 'Healthy'}\nreturn hs",
			UseOpenLibs: false,
		},
		// Override check for built-in Go check
		"apps/Deployment": appv1.ResourceOverride{
			HealthLua:   "hs = {status = 'Healthy', message = 'Custom Deployment'}\nreturn hs",
			UseOpenLibs: true,
		},
		// Override check for embedded Lua check
		"argoproj.io/Rollout": appv1.ResourceOverride{
			HealthLua:   "hs = {status = 'Healthy', message = 'Custom Rollout'}\nreturn hs",
			UseOpenLibs: true,
		},
		// Override check for embedded wildcard pattern (*.cnrm.cloud.google.com)
		"foo.cnrm.cloud.google.com/Bar": appv1.ResourceOverride{
			HealthLua:   "hs = {status = 'Healthy', message = 'Custom GCP Bar'}\nreturn hs",
			UseOpenLibs: true,
		},
	}

	defs, err := EnumerateHealthChecks(overrides)
	require.NoError(t, err)

	var widgetDef, deployDef, rolloutDef, gcpBarDef *HealthCheckDefinition
	for i := range defs {
		d := &defs[i]
		switch d.Key {
		case "custom.io/Widget":
			widgetDef = d
		case "apps/Deployment":
			deployDef = d
		case "argoproj.io/Rollout":
			rolloutDef = d
		case "foo.cnrm.cloud.google.com/Bar":
			gcpBarDef = d
		}
	}

	require.NotNil(t, widgetDef, "expected custom.io/Widget def")
	assert.Equal(t, HealthCheckOriginCustomLua, widgetDef.Origin)
	assert.Equal(t, "custom.io", widgetDef.Group)
	assert.Equal(t, "Widget", widgetDef.Kind)
	assert.False(t, widgetDef.UseOpenLibs)

	require.NotNil(t, deployDef, "expected apps/Deployment def")
	assert.Equal(t, HealthCheckOriginOverrideLua, deployDef.Origin)
	assert.Equal(t, "hs = {status = 'Healthy', message = 'Custom Deployment'}\nreturn hs", deployDef.LuaScript)

	require.NotNil(t, rolloutDef, "expected argoproj.io/Rollout def")
	assert.Equal(t, HealthCheckOriginOverrideLua, rolloutDef.Origin)
	assert.Equal(t, "hs = {status = 'Healthy', message = 'Custom Rollout'}\nreturn hs", rolloutDef.LuaScript)

	require.NotNil(t, gcpBarDef, "expected foo.cnrm.cloud.google.com/Bar def")
	assert.Equal(t, HealthCheckOriginOverrideLua, gcpBarDef.Origin)
	assert.Equal(t, "hs = {status = 'Healthy', message = 'Custom GCP Bar'}\nreturn hs", gcpBarDef.LuaScript)
}

func TestEnumerateHealthChecks_WildcardOrigins(t *testing.T) {
	overrides := ResourceHealthOverrides{
		// Custom wildcard with NO built-in overlap -> CustomLua
		"mycustom.io/*": appv1.ResourceOverride{
			HealthLua: "hs = {status = 'Healthy'}\nreturn hs",
		},
		// Custom wildcard overlapping a built-in definition (apps/Deployment) -> OverrideLua
		"apps/*": appv1.ResourceOverride{
			HealthLua: "hs = {status = 'Healthy'}\nreturn hs",
		},
	}

	defs, err := EnumerateHealthChecks(overrides)
	require.NoError(t, err)

	var myCustomWildcard, appsWildcard *HealthCheckDefinition
	for i := range defs {
		d := &defs[i]
		switch d.Key {
		case "mycustom.io/*":
			myCustomWildcard = d
		case "apps/*":
			appsWildcard = d
		}
	}

	require.NotNil(t, myCustomWildcard, "expected mycustom.io/* def")
	assert.Equal(t, HealthCheckOriginCustomLua, myCustomWildcard.Origin, "custom wildcard without built-in overlap must be CustomLua")
	assert.True(t, myCustomWildcard.IsWildcard)

	require.NotNil(t, appsWildcard, "expected apps/* def")
	assert.Equal(t, HealthCheckOriginOverrideLua, appsWildcard.Origin, "custom wildcard overlapping built-in definitions must be OverrideLua")
	assert.True(t, appsWildcard.IsWildcard)
}

func TestEnumerateHealthChecks_DeterministicOrderingAndNoDuplicates(t *testing.T) {
	overrides := ResourceHealthOverrides{
		"zebra.io/Zebra": appv1.ResourceOverride{HealthLua: "return {}"},
		"alpha.io/Alpha": appv1.ResourceOverride{HealthLua: "return {}"},
	}

	defs, err := EnumerateHealthChecks(overrides)
	require.NoError(t, err)

	// Verify no duplicates by key
	seen := make(map[string]bool)
	for _, def := range defs {
		assert.False(t, seen[def.Key], "duplicate key found: %s", def.Key)
		seen[def.Key] = true
	}

	// Verify sorted order (by Group, Kind, Key)
	for i := 0; i < len(defs)-1; i++ {
		curr := defs[i]
		next := defs[i+1]
		if curr.Group == next.Group {
			if curr.Kind == next.Kind {
				assert.LessOrEqual(t, curr.Key, next.Key)
			} else {
				assert.LessOrEqual(t, curr.Kind, next.Kind)
			}
		} else {
			assert.LessOrEqual(t, curr.Group, next.Group)
		}
	}
}

func TestEnumerateHealthChecks_RuntimeEvaluationUnchanged(t *testing.T) {
	overrides := ResourceHealthOverrides{
		"apps/Deployment": appv1.ResourceOverride{
			HealthLua: `
hs = {}
hs.status = "Healthy"
hs.message = "Overridden Deployment"
return hs
`,
		},
	}

	// Enumeration does not break runtime health checking
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "my-deploy",
				"namespace": "default",
			},
		},
	}

	status, err := overrides.GetResourceHealth(obj)
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, "Healthy", string(status.Status))
	assert.Equal(t, "Overridden Deployment", status.Message)
}
