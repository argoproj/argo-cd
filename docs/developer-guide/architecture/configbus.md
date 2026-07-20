# Config bus

The **config bus** (`util/configbus`) exists to migrate Argo CD’s **durable
product settings** from ConfigMaps (`argocd-cm`, `argocd-cmd-params-cm`, and
related sources) to a **singleton configuration CRD**. The bus’s
`configbus.Provider` is the stable API that component code calls during that
migration: call sites read typed getters instead of reaching into flags, env
vars, or ConfigMaps directly. Backing sources change behind the Provider; the
call sites do not.

> [!NOTE]
> This page is for **contributors** changing how Argo CD reads configuration.
> It describes the bus as of the first consumer cutover (application-controller),
> when the Provider still resolves from Legacy (flags/env) and `SettingsManager`
> (ConfigMaps). Later work wires the same Provider to the singleton CRD; this
> document will grow with those changes.

## Why it exists

The end state is one declarative config object per install. Getting there
requires a single typed read path first—otherwise every binary keeps its own
ConfigMap / flag / env parsing, and a CRD cutover would mean rewriting call
sites again.

Without a shared bus:

- Precedence between flag, env, and ConfigMap differed by binary.
- Call sites often mixed ConfigMap reads, constructor fields, and ad hoc parsing.
- Resolve failures were easy to log-and-ignore, leaving zero/default values in
  effect.

The Provider gives one place to add settings, one place to swap ConfigMap-backed
resolution for CRD-backed resolution later, and a clear error path when a
required value cannot be resolved.


## Architecture (current)

```text
  CLI flags / env          argocd-cm (via SettingsManager)
        │                            │
        ▼                            ▼
  ControllerLegacy            SettingsManager getters
  (legacy_config.go)                 │
        │                            │
        └──────────┬─────────────────┘
                   ▼
               Provider
                   │
                   ▼
         Controller call sites
         (configProvider.SomeMethod())
```

| Piece | Path | Role |
| --- | --- | --- |
| `Provider` | `util/configbus/provider.go` | Typed API for one process (`ReconciliationTimeout()`, `ResourceOverrides()`, …). |
| `LegacyValues` | `util/configbus/provider.go` | Holds component Legacy adapters. Nil field means “not supplied by this binary.” |
| `ControllerLegacy` | `util/configbus/cmd_params_controller.go` | Interface implemented by `*controller.ApplicationController` for flag/env values already stored on the controller. |
| Legacy adapters | `controller/legacy_config.go` | **Sole** allowed readers of deprecated controller struct fields. |
| Settings-backed methods | `util/configbus/provider.go`, `provider_settings.go` | Read durable CM-backed values through `SettingsManager`. |

There is **no** global setting registry. Provider methods call
`SettingsManager` and/or the component Legacy adapter directly.

### What is wired today

| Binary | Status |
| --- | --- |
| Application controller | Wired: `NewProvider(settingsMgr, &LegacyValues{Controller: &ctrl})` in `controller/appcontroller.go` |
| API server, repo-server, ApplicationSet, notifications, commit-server | Not yet on the bus (follow the same pattern when cut over) |

### Sources of truth (controller)

| Kind of setting | How the Provider gets it | Examples |
| --- | --- | --- |
| Flag / env captured at process start | `ControllerLegacy` → deprecated struct fields | Reconciliation timeout, sync timeout, self-heal, metrics cluster labels |
| ConfigMap-backed product config | `SettingsManager` | Resource overrides, app instance label key, tracking method |

Deprecated struct fields stay on the controller for construction/tests, but
product code and tests must read via `configProvider.*`. Mark fields
`Deprecated: use configProvider.…` and confine Legacy readers to
`legacy_config.go`.

## How the controller wires the Provider

In `controller/appcontroller.go` (after settings manager and controller fields
exist):

```go
ctrl.configProvider = configbus.NewProvider(settingsMgr, &configbus.LegacyValues{Controller: &ctrl})
```

Call sites then use:

```go
timeout, err := ctrl.configProvider.SelfHealTimeout()
if err != nil {
	return fmt.Errorf("failed to resolve self heal timeout: %w", err)
}
```

Some Legacy-backed methods still return a bare value (for example
`ReconciliationTimeout() time.Duration`). Prefer `(T, error)` for new methods,
and bubble errors at call sites (return, fatal at startup, or requeue)—do **not**
log-and-ignore and continue with a zero value.

## Common tasks

### Add a controller setting (flag / env)

1. **Store the value** on `ApplicationController` (or a nested manager) at
   construction time, as today.
2. **Mark the field deprecated** toward the Provider:
   `// Deprecated: use configProvider.MySetting.`
3. **Extend `ControllerLegacy`** in `util/configbus/cmd_params_controller.go`
   with `LegacyMySetting() T`.
4. **Implement the Legacy method** in `controller/legacy_config.go` (sole reader
   of the deprecated field; keep the `SA1019` nolint pattern used by siblings).
5. **Add `Provider.MySetting()`** in `cmd_params_controller.go` (or
   `provider.go` / `provider_settings.go` if it is clearly shared). Prefer
   `(T, error)` and `requireControllerLegacy()`.
6. **Update call sites** to use `configProvider.MySetting()` and handle errors.
7. **Tests:** construct the controller as usual; assert behavior through the
   Provider or through controller behavior that already exercises the getter.
8. Run `go test ./util/configbus/ ./controller/`.

### Add a SettingsManager-backed setting

1. Ensure the value is available from `util/settings` (existing or new getter).
2. Add `Provider.MySetting() (T, error)` that calls `requireSettingsMgr()` then
   the settings getter (see `ResourceOverrides()` in `provider.go`).
3. Point controller call sites at the Provider method.
4. Add/adjust unit tests; run `go test ./util/configbus/ ./controller/`.

### Change how an existing setting is resolved

1. Find the Provider method (`rg 'func \(p \*Provider\) Foo' util/configbus`).
2. Follow it to Legacy or `SettingsManager`.
3. Prefer updating that single path over adding a parallel read in the
   controller.
4. Keep `legacy_config.go` as the only deprecated-field reader.

## Error handling

| Context | Prefer |
| --- | --- |
| Constructor / startup | Return `error` or fatal if the process cannot run correctly |
| Reconcile / workqueue | Return error or requeue; do not proceed with zero config |
| Optional best-effort paths | Rare; document why a default is safe |

Anti-pattern: `log.WithError(err).Error(...); /* continue */` for Provider
resolve failures.

## File map

```text
util/configbus/
├── provider.go                 # Provider, LegacyValues, core methods
├── cmd_params_controller.go    # ControllerLegacy + controller Provider methods
├── provider_settings.go        # Additional settings-backed helpers
├── env_settings.go             # Env-only helpers (when present)
└── provider_test.go

controller/
├── appcontroller.go            # Wires NewProvider; call sites use configProvider
└── legacy_config.go            # ControllerLegacy implementation
```

## Related

- Components overview: [Component Architecture](components.md)
- Local checks: [Development Cycle](../development-cycle.md)
