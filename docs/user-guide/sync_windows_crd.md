# Sync Window CRD (`SyncWindow`)

The `SyncWindow` CRD lets you define sync windows as **standalone, reusable Kubernetes
objects** and reference them from `AppProject`s and `Application`s, instead of embedding them
inline in `AppProject.spec.syncWindows`.

> [!NOTE]
> This is additive and fully backward compatible. Inline `AppProject.spec.syncWindows` continue
> to work unchanged. CRD-based windows are merged with inline windows and evaluated by the same
> engine (`SyncWindows.CanSync`), so all existing semantics, including **deny always wins over
> allow** and `manualSync`, apply identically.

The `SyncWindow` CRD must be installed in the cluster before the Argo CD application controller and
API server start. The informer cache for this resource is part of their startup readiness checks.
Inline project sync windows still use their existing fields, but the CRD itself is required by the
current implementation.

## The CRD

| Property | Value |
|---|---|
| Group | `argoproj.io` |
| Version | `v1alpha1` |
| Kind | `SyncWindow` |
| Plural | `syncwindows` |
| Singular | `syncwindow` |
| Short name | `sw` |
| Scope | Namespaced |

```yaml
apiVersion: argoproj.io/v1alpha1
kind: SyncWindow
metadata:
  name: nightly-freeze
  namespace: argocd
spec:
  windows:
    - kind: deny                 # "allow" or "deny"
      schedule: "0 22 * * *"     # cron
      duration: 8h               # e.g. "1h", "30m"
      timeZone: Asia/Kolkata     # IANA tz, defaults to UTC
      manualSync: false          # do not allow manual syncs during a deny window
      applications: ["prod-*"]   # glob filters (see note on where they apply)
      namespaces: ["prod"]
      clusters: ["in-cluster"]
      andOperator: false         # AND across apps/namespaces/clusters instead of OR
      syncOverrun: false         # let ongoing syncs continue past the window boundary
      description: "No deploys overnight"
```

A single `SyncWindow` can hold **multiple** window definitions under `spec.windows`.

```bash
kubectl get sw -n argocd
kubectl get syncwindows -n argocd
```

## Referencing windows

There are two reference sites, and **the site, not the lookup method, determines how filters
behave.** This is the single most important rule in this document.

### From an `AppProject` (filtered)

Field: `AppProject.spec.syncWindowRefs`, type `SyncWindowProjectRef` (a `ref` plus optional
`applications`/`namespaces`/`clusters`).

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
spec:
  syncWindowRefs:
    - ref:
        name: nightly-freeze         # by name
      applications: ["springboot-*"] # optional: narrow which apps this ref applies to
    - ref:
        selector:                    # OR by label selector
          matchLabels:
            team: platform
```

Windows resolved from a project ref are **filtered**: they are passed through
`SyncWindows.Matches(app)` before `CanSync`. An app in the project is gated **only if it matches**
a window's filters.

### From an `Application` (direct)

Field: `Application.spec.syncWindowRefs`, type `SyncWindowRef` (`name` or `selector` only, **no**
filter fields).

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  syncWindowRefs:
    - name: nightly-freeze           # by name
    - selector:                      # OR by label selector
        matchLabels:
          team: platform
```

Windows resolved from an app ref are **direct**: their `applications`/`namespaces`/`clusters`
filters are **cleared** and the window applies to the referencing application **unconditionally**.
The app already selected itself by referencing the window, so intrinsic filters are meaningless.

### ApplicationSet

`ApplicationSet` has **no** `syncWindowRefs` field of its own. Put the ref in
`spec.template.spec.syncWindowRefs`; every generated `Application` inherits it (behaving as an app
ref directly).

> [!WARNING]
> The ApplicationSet controller owns its generated apps and overwrites their spec from the template
> on every reconcile. `syncWindowRefs` **must** be declared in the template. A ref added directly
> to a generated `Application` will be stripped on the next reconcile.

## Filter behavior: the key distinction

| | **AppProject ref** | **Application ref** |
|---|---|---|
| Field type | `SyncWindowProjectRef` (ref + filters) | `SyncWindowRef` (ref only) |
| Resolver | `ResolveProjectRefs` | `ResolveAppRefs` |
| Definition's `applications`/`namespaces`/`clusters` | honored (or overridden by ref filters) | **cleared / ignored** |
| Enforced via | `Matches(app)` then `CanSync` | appended directly, then `CanSync` |
| A window with **no** filters | **dropped**, never applies (`Matches` needs ≥1 filter) | **applies** to the app |
| Scoping question | "which apps in the project?" | already answered, this app |

> [!IMPORTANT]
> A filter-less window referenced from a **project** effectively won't apply, because
> `SyncWindows.Matches(app)` requires at least one filter to be set. To mean "all apps in the
> project," use a wildcard such as `applications: ["*"]`. From an **application**, a filter-less
> window applies unconditionally, so no wildcard is needed.

## Where to put filters

The same reusable window can be scoped in two ways.

**Option A: filter in the CR** (intrinsic scope; every referencer gets the same scope):

```yaml
# SyncWindow
spec:
  windows:
    - kind: deny
      applications: ["springboot-*"]
---
# AppProject
spec:
  syncWindowRefs:
    - ref:
        name: nightly-freeze          # no filters here → CR's filter is used
```

**Option B: filter on the project ref** (per-project scoping of a shared window):

```yaml
# SyncWindow
spec:
  windows:
    - kind: deny                      # no filters in the CR
---
# AppProject
spec:
  syncWindowRefs:
    - ref:
        name: nightly-freeze
      applications: ["springboot-*"]  # scope applied at reference time
```

Option B is the reusability pattern: a platform team owns a generic `nightly-freeze` CR and each
product team narrows it to their own apps.

## Combining filters: CR **and** project ref

When both the CR definition and the project ref set filters, the project ref **replaces** the CR's
value, **per axis** (not merged, not intersected):

```go
sw := definitionToSyncWindow(def)      // starts with the CR's filters
if len(ref.Applications) > 0 { sw.Applications = ref.Applications } // REPLACES
if len(ref.Namespaces)   > 0 { sw.Namespaces   = ref.Namespaces   } // REPLACES
if len(ref.Clusters)     > 0 { sw.Clusters     = ref.Clusters     } // REPLACES
```

| Axis | CR definition | Project ref | Effective |
|---|---|---|---|
| applications | `["prod-*"]` | `["springboot-*"]` | **`["springboot-*"]`** (ref wins) |
| namespaces | `["prod"]` | *(unset)* | **`["prod"]`** (CR kept) |
| clusters | *(unset)* | `["in-cluster"]` | **`["in-cluster"]`** (ref) |

> [!WARNING]
> Override is **replacement, not narrowing**. If the CR says `applications: ["prod-*"]` and the ref
> says `applications: ["springboot-*"]`, the result is exactly `["springboot-*"]`, so the `prod-*`
> filter is discarded, **not** "prod-\* AND springboot-\*". Each axis is independent, and an **empty**
> list on the ref does **not** override (the trigger is `len() > 0`), so you cannot use a project ref
> to *clear* a filter the CR set, only to replace it with a non-empty one.

## `name` vs `selector`

Within a **single** ref, `name` and `selector` are **mutually exclusive**.

```yaml
# Rejected: both set in ONE ref
syncWindowRefs:
  - ref:
      name: nightly-freeze
      selector:
        matchLabels: { team: platform }
```

At resolve time this returns an error
(`sync window ref cannot specify both name and selector`), and no CRD windows apply for that object
that cycle (the controller logs a warning and continues). Nothing is silently preferred.

Using `name` in one ref and `selector` in a **different** ref is fine. Exclusivity is per-ref, not
per-list:

```yaml
# Allowed: two separate refs
syncWindowRefs:
  - ref: { name: nightly-freeze }              # ref #1: name → Get(name)
  - ref:
      selector:                                # ref #2: selector → List(labels)
        matchLabels: { team: platform }
```

> [!NOTE]
> The `name`/`selector` exclusivity is currently enforced only in the controller at resolve time.
> The CRD OpenAPI schema does not yet reject it at `kubectl apply`; you will see a controller
> warning rather than an admission error.

## Worked scenarios

### Scenario 1: two refs on the same object (name + label)

```yaml
# AppProject
spec:
  syncWindowRefs:
    - ref: { name: nightly-freeze }                        # → Get("nightly-freeze")
    - ref:
        selector: { matchLabels: { team: platform } }      # → List(team=platform)
```

Both refs resolve independently; all resolved windows are **concatenated** and evaluated together.

> [!WARNING]
> **No deduplication.** If `nightly-freeze` itself carries the label `team: platform`, it matches
> both refs and its windows are added **twice**. This is harmless for evaluation (a duplicate deny
> is still a deny), but the window is processed more than once.

### Scenario 2: same CR referenced by name (from an app) and by label (from its project)

```yaml
# SyncWindow
metadata:
  name: nightly-freeze
  labels: { team: platform }
spec:
  windows:
    - kind: deny
      schedule: "0 22 * * *"
      duration: 8h
      applications: ["prod-*"]                 # filter in the CR
---
# Application "springboot-1"
spec:
  syncWindowRefs:
    - name: nightly-freeze                      # by name → DIRECT
---
# AppProject for springboot-1
spec:
  syncWindowRefs:
    - ref:
        selector: { matchLabels: { team: platform } }   # by label → FILTERED
```

For app `springboot-1`, **both** references resolve to the same `nightly-freeze` CR but behave
differently because of the reference site:

| Path | Resolver | CR's `applications: ["prod-*"]` | Applies to `springboot-1`? |
|---|---|---|---|
| App ref (by name) → direct | `ResolveAppRefs` | **cleared** | yes, always |
| Project ref (by label) → filtered | `ResolveProjectRefs` | **kept**, run through `Matches(app)` | no (`springboot-1` ≠ `prod-*`) |

Net: `springboot-1` **is** gated by the nightly-freeze deny, solely because of its own app-level
by-name ref, which ignores the CR's `prod-*` filter. The project's by-label ref contributes nothing
here. If the app were named `prod-1`, both copies would apply (again harmless, two denies equal one).

## Evaluation semantics (unchanged from inline windows)

Once resolved, CRD windows are merged with inline `AppProject.spec.syncWindows` and evaluated by the
existing engine:

- **Deny always wins over allow.** If any active window denies, the sync is blocked.
- **`manualSync: true`** permits manual syncs during a deny window.
- Both the **manual sync** path and the **auto-sync** path apply the same logic.
- `andOperator`, `timeZone`, `syncOverrun`, and `description` behave as they do for inline windows.

## Resolution failure behavior

Within `ResolveProjectRefs` / `ResolveAppRefs`, an unresolvable ref (missing name, invalid selector,
or `name`+`selector` together) is reported while resolution continues with the remaining refs. The
controller logs a warning and retains windows resolved from valid refs, so a bad ref does not
silently drop valid deny windows.

> [!NOTE]
> A bad ref contributes no windows, but does not disable valid refs on the same object. Check the
> controller warning and fix the ref to restore the missing windows.

## Enforcement timing

The application controller resolves and enforces CRD windows during reconcile (manual and auto sync).
The controller runs a dedicated informer for `SyncWindow` and waits for its cache to sync alongside
the application and project caches before processing. The API server does the same. Because these
informers are part of startup readiness, the `SyncWindow` CRD must be installed before starting
these components. The standard Argo CD installation manifests include the CRD.
