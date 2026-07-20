package configbus

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Descriptor is the type-erased view of a registered setting. Provider.Resolve
// looks up descriptors by Name and calls resolveAny.
type Descriptor interface {
	// Name is the stable registry key (e.g. "reconciliationTimeout").
	Name() string
	// CMKeyExact is the former ConfigMap data key this setting corresponds to, if any.
	// Used for identity/introspection (DescriptorCoversCMKey); Gets do not read it.
	CMKeyExact() string
	// CMKeyPrefix is the key prefix for a dynamic (multi-key) family, if any.
	CMKeyPrefix() string
	// EnvVar is the process environment variable this setting corresponds to, if any.
	// Used for identity/introspection (DescriptorCoversEnv). Some Gets also read this
	// env var directly; the field itself is not consulted by resolveAny.
	EnvVar() string
	// CoversCMKey reports whether this entry claims the given ConfigMap data key.
	CoversCMKey(key string) bool
	// CoversEnv reports whether this entry claims the given environment variable.
	CoversEnv(env string) bool
	// resolveAny resolves the setting via its Get callback.
	resolveAny(ctx *ResolveContext) (any, error)
}

// KeyFunc parses a concrete ConfigMap key under a dynamic prefix into an element
// identity and optional sub-field (e.g. "resource.customizations.health.apps_Deployment"
// → element "apps_Deployment", sub-field "health").
type KeyFunc func(key string) (elementID, subField string, ok bool)

// Setting is a flat (single-value) registry entry of type T.
//
// Get is the sole resolution path. It may read Legacy values, SettingsManager,
// environment, and (on later layers) ArgoCDConfiguration — whatever sources that
// setting still supports. There is no separate metadata flag for "which source";
// the Get implementation encodes that.
type Setting[T any] struct {
	// Name is required and must be unique across the registry.
	Name string
	// CMKeyExact optionally records the historical ConfigMap data key.
	CMKeyExact string
	// EnvVar optionally records the historical / transport environment variable.
	EnvVar string
	// Get resolves the live value. Required.
	Get func(*ResolveContext) (T, error)
}

// DynamicSetting is a multi-key family (prefix + KeyFunc) of type T.
// Get resolves the whole family (e.g. all resource overrides).
type DynamicSetting[T any] struct {
	// Name is required and must be unique across the registry.
	Name string
	// CMKeyPrefix is the ConfigMap key prefix for this family (required).
	CMKeyPrefix string
	// EnvVar optionally records a related environment variable.
	EnvVar string
	// KeyFunc maps a concrete CM key under the prefix to an element id (required).
	KeyFunc KeyFunc
	// Get resolves the live family value. Required.
	Get func(*ResolveContext) (T, error)
}

type settingDesc[T any] struct {
	s Setting[T]
}

func (d settingDesc[T]) Name() string        { return d.s.Name }
func (d settingDesc[T]) CMKeyExact() string  { return d.s.CMKeyExact }
func (d settingDesc[T]) CMKeyPrefix() string { return "" }
func (d settingDesc[T]) EnvVar() string      { return d.s.EnvVar }
func (d settingDesc[T]) CoversCMKey(key string) bool {
	return d.s.CMKeyExact != "" && d.s.CMKeyExact == key
}

func (d settingDesc[T]) CoversEnv(env string) bool {
	return d.s.EnvVar != "" && d.s.EnvVar == env
}

func (d settingDesc[T]) resolveAny(ctx *ResolveContext) (any, error) {
	if d.s.Get == nil {
		return nil, fmt.Errorf("config: setting %q has no Get", d.s.Name)
	}
	v, err := d.s.Get(ctx)
	return v, err
}

type dynamicDesc[T any] struct {
	s DynamicSetting[T]
}

func (d dynamicDesc[T]) Name() string        { return d.s.Name }
func (d dynamicDesc[T]) CMKeyExact() string  { return "" }
func (d dynamicDesc[T]) CMKeyPrefix() string { return d.s.CMKeyPrefix }
func (d dynamicDesc[T]) EnvVar() string      { return d.s.EnvVar }
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
	if d.s.Get == nil {
		return nil, fmt.Errorf("config: dynamic setting %q has no Get", d.s.Name)
	}
	v, err := d.s.Get(ctx)
	return v, err
}

var (
	registryMu sync.RWMutex
	registry   []Descriptor
)

// Register adds a flat setting to the process-global registry.
func Register[T any](s Setting[T]) error {
	if s.Name == "" {
		return errors.New("config: setting name is required")
	}
	if s.Get == nil {
		return fmt.Errorf("config: setting %q requires Get", s.Name)
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

// RegisterDynamic adds a dynamic setting family to the process-global registry.
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
