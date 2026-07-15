package normalizers

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"

	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/resource_customizations"
)

// ignoreDifferencesFileName is the name of the file shipped inside a resource
// customization directory that declares built-in ignoreDifferences rules.
const ignoreDifferencesFileName = "ignoreDifferences.yaml"

var (
	builtinIgnoreDifferencesOnce sync.Once
	builtinIgnoreDifferences     map[string]v1alpha1.OverrideIgnoreDiff
	builtinIgnoreDifferencesErr  error
)

// loadBuiltinIgnoreDifferences parses ignoreDifferences.yaml files shipped in
// the embedded resource_customizations filesystem. The returned map is keyed by
// "<group>/<Kind>", matching the ResourceOverrides key format.
//
// The embedded filesystem is static, so the result is parsed once and cached.
// This mirrors how built-in health.lua scripts are loaded on demand by their
// consumer (see util/lua/lua.go).
func loadBuiltinIgnoreDifferences() (map[string]v1alpha1.OverrideIgnoreDiff, error) {
	builtinIgnoreDifferencesOnce.Do(func() {
		result := make(map[string]v1alpha1.OverrideIgnoreDiff)
		builtinIgnoreDifferencesErr = fs.WalkDir(resource_customizations.Embedded, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || d.Name() != ignoreDifferencesFileName {
				return nil
			}
			// p is "<group>/<Kind>/ignoreDifferences.yaml"; path.Dir is the override key.
			data, err := resource_customizations.Embedded.ReadFile(p)
			if err != nil {
				return fmt.Errorf("error reading built-in ignoreDifferences %q: %w", p, err)
			}
			var ignoreDiff v1alpha1.OverrideIgnoreDiff
			if err := yaml.Unmarshal(data, &ignoreDiff); err != nil {
				return fmt.Errorf("error parsing built-in ignoreDifferences %q: %w", p, err)
			}
			result[path.Dir(p)] = ignoreDiff
			return nil
		})
		builtinIgnoreDifferences = result
	})
	return builtinIgnoreDifferences, builtinIgnoreDifferencesErr
}

// getBuiltinResourceIgnoreDifferences returns built-in ignoreDifferences as
// ResourceIgnoreDifferences entries, skipping any group/kind already configured
// by ConfigMap overrides (ConfigMap takes precedence, consistent with built-in
// health.lua).
func getBuiltinResourceIgnoreDifferences(overrides map[string]v1alpha1.ResourceOverride) ([]v1alpha1.ResourceIgnoreDifferences, error) {
	builtins, err := loadBuiltinIgnoreDifferences()
	if err != nil {
		return nil, err
	}
	result := make([]v1alpha1.ResourceIgnoreDifferences, 0, len(builtins))
	for key, ignoreDiff := range builtins {
		// Convert wildcard markers ("_") to glob patterns ("*"), matching how built-in
		// health.lua wildcard overrides are resolved (see util/lua/lua.go). The normalizer
		// matches group/kind via glob, where "*" spans "." (e.g. "*.crossplane.io").
		globKey := strings.ReplaceAll(key, "_", "*")
		if override, ok := overrides[globKey]; ok && hasIgnoreDifferences(override.IgnoreDifferences) {
			continue
		}
		group, kind, err := getGroupKindForOverrideKey(globKey)
		if err != nil {
			continue
		}
		result = append(result, v1alpha1.ResourceIgnoreDifferences{
			Group:                 group,
			Kind:                  kind,
			JSONPointers:          ignoreDiff.JSONPointers,
			JQPathExpressions:     ignoreDiff.JQPathExpressions,
			ManagedFieldsManagers: ignoreDiff.ManagedFieldsManagers,
		})
	}
	return result, nil
}

func hasIgnoreDifferences(d v1alpha1.OverrideIgnoreDiff) bool {
	return len(d.JSONPointers) > 0 || len(d.JQPathExpressions) > 0 || len(d.ManagedFieldsManagers) > 0
}
