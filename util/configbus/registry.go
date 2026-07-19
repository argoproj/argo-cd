package configbus

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Known SourceConfigMap values.
const (
	SourceArgoCDCM    = "argocd-cm"
	SourceCmdParamsCM = "argocd-cmd-params-cm"
	SourceRBACCM      = "argocd-rbac-cm"
	SourceEnvOnly     = "env"
	SourceFlagOnly    = "flag"
)

// Descriptor is the type-erased view of a registry entry used by the drift guard,
// inventory tooling, and documentation generator. Phase 0 descriptors describe
// CURRENT config sources only — no CRDPath or Default.
type Descriptor interface {
	// Name is a stable registry identifier (e.g. "reconciliationTimeout").
	Name() string
	// CMKeyExact is an exact argocd-cm / argocd-cmd-params-cm data key, if any.
	CMKeyExact() string
	// CMKeyPrefix is a dynamic key prefix (e.g. "resource.customizations."), if any.
	CMKeyPrefix() string
	// EnvVar is the environment variable name used as metadata (and by components
	// via env helpers). The provider does not read env itself for flag-bound vars.
	EnvVar() string
	// HotReload is true when changes are expected to take effect without restart.
	HotReload() bool
	// Separator is the list-value separator for this setting ("," by default;
	// e.g. ";" for reposerver.plugin.tar.exclusions). Empty means ",".
	Separator() string
	// Secret is true when values are resolved against argocd-secret.
	Secret() bool
	// Component is the owning Argo CD component (e.g. "controller", "server"), if any.
	Component() string
	// SourceConfigMap is the ConfigMap that owns this key (argocd-cm, argocd-cmd-params-cm,
	// argocd-rbac-cm) or "env" for standalone env vars.
	SourceConfigMap() string
	// FlagName is the cobra flag name when this setting is flag-bound, if any.
	FlagName() string
	// CoversCMKey reports whether this entry claims the given ConfigMap data key.
	CoversCMKey(key string) bool
	// CoversEnv reports whether this entry claims the given environment variable.
	CoversEnv(env string) bool
	// resolveAny resolves the setting through its Get/Parse callback.
	resolveAny(ctx *ResolveContext) (any, error)
}

// KeyFunc parses a concrete ConfigMap key under a dynamic prefix into an element
// identity and optional sub-field (e.g. "resource.customizations.health.apps_Deployment"
// → element "apps_Deployment", sub-field "health").
type KeyFunc func(key string) (elementID, subField string, ok bool)

// Setting describes a flat (non-dynamic) configuration value of type T.
// Supply either Parse (simple string conversion) or Get (custom resolution), not both.
type Setting[T any] struct {
	Name            string
	CMKeyExact      string
	EnvVar          string
	HotReload       bool
	Separator       string
	Secret          bool
	Component       string
	SourceConfigMap string
	FlagName        string
	Parse           func(string) (T, error)
	Get             func(*ResolveContext) (T, error)
	Validate        func(T) error
}

// DynamicSetting describes a multi-key / keyed-list family of type T.
type DynamicSetting[T any] struct {
	Name            string
	CMKeyPrefix     string
	EnvVar          string
	HotReload       bool
	Separator       string
	Secret          bool
	Component       string
	SourceConfigMap string
	FlagName        string
	KeyFunc         KeyFunc
	Get             func(*ResolveContext) (T, error)
	Validate        func(T) error
}

type settingDesc[T any] struct {
	s Setting[T]
}

func (d settingDesc[T]) Name() string            { return d.s.Name }
func (d settingDesc[T]) CMKeyExact() string      { return d.s.CMKeyExact }
func (d settingDesc[T]) CMKeyPrefix() string     { return "" }
func (d settingDesc[T]) EnvVar() string          { return d.s.EnvVar }
func (d settingDesc[T]) HotReload() bool         { return d.s.HotReload }
func (d settingDesc[T]) Separator() string       { return effectiveSeparator(d.s.Separator) }
func (d settingDesc[T]) Secret() bool            { return d.s.Secret }
func (d settingDesc[T]) Component() string       { return d.s.Component }
func (d settingDesc[T]) SourceConfigMap() string { return d.s.SourceConfigMap }
func (d settingDesc[T]) FlagName() string        { return d.s.FlagName }
func (d settingDesc[T]) CoversCMKey(key string) bool {
	return d.s.CMKeyExact != "" && d.s.CMKeyExact == key
}

func (d settingDesc[T]) CoversEnv(env string) bool {
	return d.s.EnvVar != "" && d.s.EnvVar == env
}

func (d settingDesc[T]) resolveAny(ctx *ResolveContext) (any, error) {
	if d.s.Get != nil {
		v, err := d.s.Get(ctx)
		return v, err
	}
	return nil, fmt.Errorf("config: setting %q has no Get (Parse-only; use component wiring)", d.s.Name)
}

type dynamicDesc[T any] struct {
	s DynamicSetting[T]
}

func (d dynamicDesc[T]) Name() string            { return d.s.Name }
func (d dynamicDesc[T]) CMKeyExact() string      { return "" }
func (d dynamicDesc[T]) CMKeyPrefix() string     { return d.s.CMKeyPrefix }
func (d dynamicDesc[T]) EnvVar() string          { return d.s.EnvVar }
func (d dynamicDesc[T]) HotReload() bool         { return d.s.HotReload }
func (d dynamicDesc[T]) Separator() string       { return effectiveSeparator(d.s.Separator) }
func (d dynamicDesc[T]) Secret() bool            { return d.s.Secret }
func (d dynamicDesc[T]) Component() string       { return d.s.Component }
func (d dynamicDesc[T]) SourceConfigMap() string { return d.s.SourceConfigMap }
func (d dynamicDesc[T]) FlagName() string        { return d.s.FlagName }
func (d dynamicDesc[T]) CoversCMKey(key string) bool {
	if d.s.CMKeyPrefix == "" {
		return false
	}
	if !strings.HasPrefix(key, d.s.CMKeyPrefix) && key != strings.TrimSuffix(d.s.CMKeyPrefix, ".") {
		return false
	}
	if d.s.KeyFunc == nil {
		return strings.HasPrefix(key, d.s.CMKeyPrefix) || key == strings.TrimSuffix(d.s.CMKeyPrefix, ".")
	}
	_, _, ok := d.s.KeyFunc(key)
	return ok
}

func (d dynamicDesc[T]) CoversEnv(env string) bool {
	return d.s.EnvVar != "" && d.s.EnvVar == env
}

func (d dynamicDesc[T]) resolveAny(ctx *ResolveContext) (any, error) {
	if d.s.Get != nil {
		v, err := d.s.Get(ctx)
		return v, err
	}
	return nil, fmt.Errorf("config: dynamic setting %q has no Get", d.s.Name)
}

func effectiveSeparator(s string) string {
	if s == "" {
		return ","
	}
	return s
}

var (
	registryMu sync.RWMutex
	registry   []Descriptor
)

// Register adds a flat setting descriptor to the process-global registry.
func Register[T any](s Setting[T]) error {
	if s.Name == "" {
		return errors.New("config: setting name is required")
	}
	if s.Parse != nil && s.Get != nil {
		return fmt.Errorf("config: setting %q must not set both Parse and Get", s.Name)
	}
	if s.Parse == nil && s.Get == nil {
		return fmt.Errorf("config: setting %q requires Parse or Get", s.Name)
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	for _, existing := range registry {
		if existing.Name() == s.Name {
			return fmt.Errorf("config: duplicate setting name %q", s.Name)
		}
	}
	registry = append(registry, settingDesc[T]{s: s})
	return nil
}

// RegisterDynamic adds a dynamic setting descriptor to the process-global registry.
func RegisterDynamic[T any](s DynamicSetting[T]) error {
	if s.Name == "" {
		return errors.New("config: dynamic setting name is required")
	}
	if s.CMKeyPrefix == "" {
		return fmt.Errorf("config: dynamic setting %q requires CMKeyPrefix", s.Name)
	}
	if s.Get == nil {
		return fmt.Errorf("config: dynamic setting %q requires Get", s.Name)
	}
	if s.KeyFunc == nil {
		return fmt.Errorf("config: dynamic setting %q requires KeyFunc", s.Name)
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	for _, existing := range registry {
		if existing.Name() == s.Name {
			return fmt.Errorf("config: duplicate setting name %q", s.Name)
		}
	}
	registry = append(registry, dynamicDesc[T]{s: s})
	return nil
}

// MustRegister panics if Register fails.
func MustRegister[T any](s Setting[T]) {
	if err := Register(s); err != nil {
		panic(err)
	}
}

// MustRegisterDynamic panics if RegisterDynamic fails.
func MustRegisterDynamic[T any](s DynamicSetting[T]) {
	if err := RegisterDynamic(s); err != nil {
		panic(err)
	}
}

// panicGet returns a Get callback that panics when invoked. Used for coverage-only
// descriptors that are registered for drift-guard completeness but not yet
// resolved through the provider.
func panicGet[T any](name string) func(*ResolveContext) (T, error) {
	return func(*ResolveContext) (T, error) {
		panic(fmt.Sprintf("config: %q not yet resolved via provider", name))
	}
}

// AllDescriptors returns a snapshot of registered descriptors.
func AllDescriptors() []Descriptor {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]Descriptor, len(registry))
	copy(out, registry)
	return out
}

// DescriptorByName returns the registered descriptor with the given name, or nil.
func DescriptorByName(name string) Descriptor {
	for _, d := range AllDescriptors() {
		if d.Name() == name {
			return d
		}
	}
	return nil
}

// ResetRegistryForTest clears the registry. For tests only.
func ResetRegistryForTest() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = nil
}

// DescriptorCoversCMKey reports whether any registered descriptor claims key.
func DescriptorCoversCMKey(key string) bool {
	for _, d := range AllDescriptors() {
		if d.CoversCMKey(key) {
			return true
		}
	}
	return false
}

// DescriptorCoversEnv reports whether any registered descriptor claims env.
func DescriptorCoversEnv(env string) bool {
	for _, d := range AllDescriptors() {
		if d.CoversEnv(env) {
			return true
		}
	}
	return false
}

// DescriptorCoversFlag reports whether any registered descriptor claims the
// given cobra/pflag name via FlagName metadata.
func DescriptorCoversFlag(flagName string) bool {
	if flagName == "" {
		return false
	}
	for _, d := range AllDescriptors() {
		if d.FlagName() == flagName {
			return true
		}
	}
	return false
}
