# ArgoCDConfiguration Singleton

Argo CD controllers have historically derived much of their configuration from a few main sources:

* `argocd-cm` ConfigMap - mostly dynamic settings hot-reloaded at runtime
* `argocd-cmd-params-cm` ConfigMap - mostly static settings loaded once at startup via env vars
* `argocd-rbac-cm` ConfigMap - mostly dynamic RBAC settings hot-loaded at runtime
* CLI flags - these may be set directly in manifests
* Environment variables - these may be set directly in manifests

These config sources have downsides for users:

* They're scattered: it's sometimes unclear where to find a particular setting
* They're unstructured: ConfigMap values, flags, and env vars are all strings, and structured data must be encoded and embedded as a string
* It's not always clear which settings are hot-reloaded and which require restarts
* It's not always clear what default values and validations apply to any given setting
* There's not strong protection against misconfiguration - the process is often just "set the value and see if anything breaks"

They also have downsides for maintainers:

* When adding a new config item, it can be unclear where to put it: env var, ConfigMap, which ConfigMap, etc.
* It's difficult to assess which config items actually exist, since they're scattered across multiple sources
* It can be difficult for a particular code path to gain access to a config item without wiring it through a series of struct initializations and function calls

Users and maintainers would benefit from a more unified and standardized configuration system.

## ArgoCDConfiguration CRD

The ArgoCDConfiguration CRD provides a much stronger configuration experience for users and maintainers. It follows the
"config singleton CRD" pattern that is increasingly popular with cloud native projects.

The advantages of a CRD over ConfigMaps/flags/env vars are:

1. The data is fully structured and typed: no YAML embedded in strings
2. Input can be validated at apply time according to declarative rules
3. All config lives in one place that's easily discoverable and readable

Moving to a CRD singleton will require a transition period so the legacy config sources still work while users migrate.
The requirements for the transitional period are:

* The singleton CR must not be a required resource. Users can continue to use the old config sources with no immediate migration steps.
* Users must be allowed to migrate incrementally: once the singleton CR is created, fields may be added as overrides over time instead of all at once.

These migration requirements imply a few restrictions on the singleton CRD design:

* Fields that represent "atomic migration units" must be optional and nullable
* "Not configured" must be distinguishable from "configured as null" - the former falls back to the legacy config sources, and the latter overrides (this may require nesting fields that may be "configured as null" under a nullable field to represent "not configured")
* Migrateable fields must not have declarative defaults: if a field has a default value, the null (unconfigured) value becomes invisible to Argo, making it impossible to fall back to legacy config sources

Even with these limitations, a singleton CRD offers strong advantages. And once legacy sources are deprecated and removed, a new version of the CRD can drop unnecessary nulls and set declarative defaults.

### CRD Structure

A CRD allows field nesting, an organizational structure that ConfigMaps don't offer.

Everyone's mental model of Argo CD's configuration will be unique. We can build a reasonably-intuitive structure by
nesting fields under a top-level field named after the **primary consumer of that config**. Fields that are shared
equally across multiple components may be placed at the top level.

Beyond this basic rule, organizational decisions will always be somewhat subjective. We'll just have to do our best.

Example:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCDConfiguration
spec:
  applicationNamespaces: ['argocd-*']  # Shared
  server:
    authEnabled: true                  # Primarily consumed by API server
```

## The Transition, Under the Hood

Writing and maintaining the code to migrate from legacy config sources to a singleton CRD is nontrivial. But once we
take stock of existing categories and access patterns, we can establish a set of code patterns to make the process
systemic, predictable, and reliable instead of confusing and ad-hoc.

### Config Load Patterns

There are three categories of config load patterns:

1. Direct pulls from ConfigMaps
2. Struct fields populated at startup (usually from CLI flags with env var fallbacks - env vars may be mounted from ConfigMaps)
3. Direct pulls from env vars (not CLI flag fallbacks)

### Config Usage Patterns

Usage patterns cluster around how the config is loaded.

#### Direct ConfigMap pulls

Pulling values directly from ConfigMaps is always done via the existing `SettingsManager` struct. This struct uses a
ConfigMap informer to maintain a copy of the latest state of the ConfigMap. Each config value is retrieved via a
dedicated, field-specific method.

#### CLI flags

Values populated at startup come from Cobra-populated CLI flags. When unset, the flags generally fall back to env vars
which are almost all loaded via `envFrom` references in the deployment manifests. Regardless of which value "wins"
(flag or env var), the effect is the same: the value is loaded into a variable which is generally set on a long-lived,
component-specific struct. For example:

```go
var serverSideDiff bool

command.Flags().BoolVar(&serverSideDiff, "server-side-diff-enabled", env.ParseBoolFromEnv(common.EnvServerSideDiff, false), "Feature flag to enable ServerSide diff. Default (\"false\")")

ctrl := NewController(
	// ... other flags
	serverSideDiff
)
```

#### Direct env var pulls

Values loaded directly from env vars are loaded from arbitrary locations in the code base instead of being concentrated
in CLI code. Exact patterns vary, but this is a representative example:

```go
var timeout time.Duration

func init() {
    timeout, err: = time.ParseDuration(os.Getenv("ARGOCD_EXEC_TIMEOUT"))
    if err != nil {
        timeout = 90 * time.Second
    }
}
```

### New, Unified Config Bus

The first step towards supporting unified, CRD-based configuration is unifying all the relevant config access through a
single interface. This interface, called the "config bus," will serve as the "switch," flipping from the legacy value
to the override CRD-based value once it's set to a non-null value by the user.

The bus is defined as a single Go interface:

```go
package configbus

type Provider interface {
	ServerSideDiffEnabled(ctx context.Context) (bool, error)
	SomeOtherConfig(ctx context.Context) (string, error)
	// etc.
}
```

The fields are added manually and sorted alphabetically for easy scanning.

Every method returns a concrete type and an error. Even when a config value is guaranteed to be available (like with a
Cobra CLI flag), the signature contains an error to prepare for a `CRDProvider` implementation which may always return
an error if, for example, an auth error prevents fetching the config singleton.

Legacy config sources are implemented by the combination of three structs: `SettingsManagerProvider`, `StaticProvider`, and
`EnvProvider`. Each implements _only the methods relevant to that config source_. For example, `SettingsManagerProvider`
implements `ApplicationDeepLinks`, since that value comes from argocd-cm, and `EnvProvider` implements
`GitRequestTimeout`, since that value comes directly from an env var.

`StaticProvider` fills three roles: handling static values from CLI flags/env overrides, overriding server-side config
with local CLI flags for commands that allow overrides, and providing an easy way for tests to override config values.
We'll show momentarily how these overrides are applied via the `ChainProvider`.

Each of these three legacy config providers implements only the config getters relevant to that source type. They're
combined into a single `LegacyProvider` which implements the full `Provider` interface.

```go
var _ Provider = (*LegacyProvider)(nil)

type LegacyProvider struct {
	SettingsManagerProvider
	StaticProvider
	EnvProvider
}
```

> [!WARNING]
> Go allows multiple embedded structs to implement the same methods. If there is a collision, the compiler forces the
> developer to specify which embedded struct's method to use. When a new config method is added to any legacy provider,
> reviewers should be vigilant of conflicts and ensure the providers' methods are disjoint before merging. The conflict
> should be obvious, because the consuming code will have to call `provider.<SpecificProvider>.Value()` instead of just
> `provider.Value()`.

Two implementations of the `Provider` interface are generated automatically: `ChainProvider` and `mocks.Provider`.

`ChainProvider` allows multiple `Provider` implementations to be chained with a precedence order. This satisfies three
use cases. The primary use case is ArgoCDConfiguration overriding legacy providers, detailed [below](#introducing-the-crd-config-singleton).

The other two use cases are client-side CLI overrides of server-side config, and overrides for unit tests.

Certain CLI commands allow the user to override server-side config values for a one-off task. For example,
`argocd app diff` may set `--server-side`, even if server-side diff is disabled in argocd-cm. The CLI may override the
ConfigMap value by using a ChainProvider with a StaticProvider overriding a LegacyProvider.

```go
configProvider := NewChainConfigProvider(
	NewStaticProvider(Fields{
		ServerSideDiff: new(serverSideDiff)
    }),
	NewLegacyProvider(settingsMgr)
	)
}
```

`StaticProvider` returns a sentinel `ErrNotConfigured` error for any field that is not explicitly set, and
`ChainProvider` falls back to the next link in the chain for any encountered `ErrNotConfigured`. This allows the
maintainer to override a single value while still using the rest of the values as configured.

The same technique can be applied in unit tests which require one or a few override values.

`mocks.Provider` may also be used in unit tests. Generated by Mockery, it provides a lot of useful testing features like
asserting that certain methods are or are not called. But it has the downside that every called function must be
explicitly set up in the test. So `StaticProvider` might be more suitable for unit tests that involve a lot of different
config values being pulled during test execution, since only the fields relevant to the test must be explicitly
configured.

### Incremental configbus Provider Implementation

One important feature of the `Provider` interface is that it can be implemented incrementally, a few fields at a time.

The main config consumers are Argo CD components: the app controller, the API server, the repo server, etc. One way to
migrate to the unified config bus is to migrate one component at a time. Components generally have a long-lived struct
instance which holds a `settingsMgr` field for `SettingsManager` (ConfigMap) access and a lot of fields holding static
config values that were loaded from the CLI flags and env vars. Direct env var accesses are scattered throughout the
component's code.

To migrate a component, the maintainer can delete the `settingsMgr` field and instead pass the value to
`configbus.NewSettingsManagerProvider`, updating all former `settingsMgr.GetFieldName()` calls with
`configBus.FieldName()` calls. They can then delete all the static fields and instead pass their values to
`configbus.NewStaticProvider(configbus.Fields{FieldName: fieldName, etc...})`. Finally, the maintainer can grep the
component code for any `os.Getenv` calls and replace them with calls to the `configbus.EnvProvider`. Note that none of
the component's calls will go directly to any of these providers. They'll instead go through a long-lived
`configProvider` field of type `Provider` on the component's struct that previously held legacy config access fields.

Each config item used by the component must be added to the `Provider` interface and implemented on the relevant legacy
provider struct.

Following this pattern, each component can be migrated to pull the config it needs from the `configbus` rather than from
the variety of legacy sources. This paves the way for the `CRDProvider` to be added with precedence over the legacy
provider.

### Ensuring Hot-Reload Capability

Some fields will likely never be hot-reloadable. They're the fields that set configuration that will live as long as the
process, such as the listening port of the API server. It's possible that eventually even those values will be made
hot-reloadable, but that's outside the scope of this effort.

For other fields, it's important to define access patterns that unblock hot-reloading when the `CRDProvider` is added.
There are places in the code base where, even values that are pulled from ConfigMaps via `SettingsManager`, values are
cached on struct fields that live a long time (or indefinitely) without being updated. In these cases, it's best to
instead add a reference to `configProvider` to the struct and allow the consumer to pull it immediately before it's
needed.

For example:

```go
// InitAppCache gets called once at process start.
func (ctrl *Controller) InitAppCache() error {
	enabledNamespaces, err := ctrl.configBus.EnabledNamespaces()
	if err != nil {
		return fmt.Errorf("failed to get enabled namespaces config: %w", err)
    }
	ctrl.appCache = NewAppCache(enabledNamespaces)
	return nil
}

// GetApplication gets called often at runtime.
func (a *AppCache) GetApplication(name string) {
	enabledNamespace := a.enabledNamespaces
	// Code using that var
}
```

This code can be modified to get the config value just in time.

```go
func (ctrl *Controller) InitAppCache() {
	ctrl.appCache = NewAppCache(configBus)
}

func (a *AppCache) GetApplication(name string) {
	enabledNamespace, err := a.configBus.EnabledNamespaces()
}
```

> [!NOTE]
> Pulling config values "just in time" will not always be practical, especially in extremely
> performance-sensitive code. In those cases, it's fine to cache config values. Just document the limitation, and
> maintainers can come back later and try to find a performant way to hot-reload the value.

### Introducing the CRD Config Singleton

Everything up to this point has focused on paving the way for configuration backed by a CRD singleton. Now we turn to
the details of implementing the CRD and wiring it into the config infrastructure.

Before even implementing `CRDProvider`, we can imagine how to wire it into components that have been set up to use
`configbus.Provider`:

```diff
- configProvider := NewLegacyProvider(settingsMgr, Fields{...})
+ configProvider := NewChainProvider(
+ 	NewCRDProvider(client),
+ 	NewLegacyProvider(settingsMgr, Fields{...})
+ )
ctrl := NewController(configProvider)
```

`CRDProvider` is then free to implement the `Provider` interface. It could be implemented incrementally, leaving some
methods to return `ErrNotConfigured` until the relevant fields were added to the CRD. But since fields added later might
inform decisions about how to structure the CRD, it's best to implement as many fields as possible (ideally all) up
front.

Each method on `CRDProvider` should 1) get the latest CR state from the ArgoCDConfiguration informer, 2) return
`ErrNotConfigured` if the target field is null, and 3) return the CR field, transforming its structure into the expected
structure if necessary.

> [!NOTE]
> Modifying config consumer code to use newer CRD-native structures is left as an exercise for later refactors. It would
> probably be best to wait until after the legacy config provider is deprecated and removed entirely so that no
> legacy -> new structure transformation logic is necessary.

## Migrating - Users' Perspective

Migrating to the config CR must be made reasonably easy for the user.

### Generating the Config

The user will run a CLI command (something like `argocd admin generate-config`) that loads as much of the instance's
config as possible (ConfigMaps will be easier than CLI flags) and produces a CR manifest.

> [!NOTE]
> The CLI should probably implement bidirectional conversion, so that it can generate a diff at the end to help the user
understand if the conversion was lossy.
>
> The implementation will be imperfect, and we'll have to work with users to identify bugs, add test cases, and fix
> conversion logic over time.

After generating their initial config, the user should comment everything out and deploy the empty CR. This is their
starting point to begin migration.

### Identifying Un-migrated Fields

The first time an Argo CD component accesses a legacy config value, it should log a warning telling the user they're
using a deprecated input, identifying the deprecated input (ConfigMap field, env var, etc.), and specifying exactly
which field in the CR overrides the legacy value.

The user can either use these logs or simply use ArgoCDConfiguration documentation, to identify individual fields to
uncomment in their CR.

### Verifying Migrated Fields

The first time an Argo CD component accesses a CR config value, it should log the access so that the user can confirm
the new config has taken effect. They can then manually check the behavior of the relevant features to make sure
everything works as expected.

In practice users can probably migrate multiple fields at once, especially relatively simple ones.

### Removing Legacy Config

Once the user has successfully migrated everything and confirmed there are no more deprecation warnings in logs, they
can optionally remove legacy config sources such as ConfigMaps and env vars.

