package lua

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
	"github.com/argoproj/argo-cd/v3/resource_customizations"
	argoglob "github.com/argoproj/argo-cd/v3/util/glob"
)

type HealthCheckOrigin string

const (
	HealthCheckOriginBuiltinGo   HealthCheckOrigin = "BuiltinGo"
	HealthCheckOriginBuiltinLua  HealthCheckOrigin = "BuiltinLua"
	HealthCheckOriginCustomLua   HealthCheckOrigin = "CustomLua"
	HealthCheckOriginOverrideLua HealthCheckOrigin = "OverrideLua"
)

// HealthCheckDefinition represents an internal summary of a health check
type HealthCheckDefinition struct {
	Group       string            `json:"group,omitempty"`
	Kind        string            `json:"kind,omitempty"`
	Key         string            `json:"key,omitempty"`
	Origin      HealthCheckOrigin `json:"origin,omitempty"`
	LuaScript   string            `json:"luaScript,omitempty"`
	UseOpenLibs bool              `json:"useOpenLibs,omitempty"`
	IsWildcard  bool              `json:"isWildcard,omitempty"`
}

// EnumerateHealthChecks aggregates and returns all built-in and custom health check definitions
func EnumerateHealthChecks(overrides ResourceHealthOverrides) ([]HealthCheckDefinition, error) {
	builtinKeys := make(map[string]bool)
	defsMap := make(map[string]HealthCheckDefinition)

	// 1. Gather Go Built-in Health Checks from gitops-engine
	for _, gvk := range health.GetBuiltinHealthCheckGVKs() {
		key := GetConfigMapKey(gvk)
		builtinKeys[key] = true
		defsMap[key] = HealthCheckDefinition{
			Group:       gvk.Group,
			Kind:        gvk.Kind,
			Key:         key,
			Origin:      HealthCheckOriginBuiltinGo,
			LuaScript:   "",
			UseOpenLibs: false,
			IsWildcard:  false,
		}
	}

	// 2. Gather Embedded Lua Health Checks from resource_customizations
	err := fs.WalkDir(resource_customizations.Embedded, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != healthScriptFile {
			return nil
		}

		dirPath := filepath.Dir(path)
		scriptBytes, err := resource_customizations.Embedded.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading embedded health script %s: %w", path, err)
		}

		isWildcard := strings.Contains(dirPath, "_")
		key := dirPath
		if isWildcard {
			key = strings.ReplaceAll(dirPath, "_", "*")
		}

		group, kind := parseGroupAndKind(key)
		builtinKeys[key] = true

		defsMap[key] = HealthCheckDefinition{
			Group:       group,
			Kind:        kind,
			Key:         key,
			Origin:      HealthCheckOriginBuiltinLua,
			LuaScript:   string(scriptBytes),
			UseOpenLibs: true,
			IsWildcard:  isWildcard,
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error enumerating embedded health scripts: %w", err)
	}

	// 3. Process Custom / Override Health Checks from argocd-cm
	for key, override := range overrides {
		if strings.TrimSpace(override.HealthLua) == "" {
			continue
		}

		isWildcard := strings.Contains(key, "*")
		group, kind := parseGroupAndKind(key)

		// Determine if key overrides a built-in check
		isOverride := builtinKeys[key]
		if !isOverride {
			if isWildcard {
				isOverride = true
			} else {
				for bKey := range builtinKeys {
					if strings.Contains(bKey, "*") && argoglob.Match(bKey, key) {
						isOverride = true
						break
					}
				}
			}
		}

		origin := HealthCheckOriginCustomLua
		if isOverride {
			origin = HealthCheckOriginOverrideLua
		}

		defsMap[key] = HealthCheckDefinition{
			Group:       group,
			Kind:        kind,
			Key:         key,
			Origin:      origin,
			LuaScript:   override.HealthLua,
			UseOpenLibs: override.UseOpenLibs,
			IsWildcard:  isWildcard,
		}
	}

	// 4. Sort deterministically by Group, Kind, Key
	var result []HealthCheckDefinition
	for _, def := range defsMap {
		result = append(result, def)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Group != result[j].Group {
			return result[i].Group < result[j].Group
		}
		if result[i].Kind != result[j].Kind {
			return result[i].Kind < result[j].Kind
		}
		return result[i].Key < result[j].Key
	})

	return result, nil
}

func parseGroupAndKind(key string) (string, string) {
	parts := strings.Split(key, "/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", key
}
