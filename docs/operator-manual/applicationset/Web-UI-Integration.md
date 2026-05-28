# Web UI Integration

For end-user documentation of the UI itself, see [Managing ApplicationSets in the Web UI](../../user-guide/application-set-ui.md).

The Web UI integrates with ApplicationSets through three layers:

1. **The `ApplicationSetService` API** exposed by the Argo CD API server (defined in [`server/applicationset/applicationset.proto`](https://github.com/argoproj/argo-cd/blob/master/server/applicationset/applicationset.proto)).
2. **The ApplicationSet CR**, read through those endpoints. The UI renders fields across both spec and status — `spec.template`, `status.conditions`, `status.resources`, and `status.health`.
3. **RBAC enforcement**, performed by the API server on every request using the same `applicationsets` resource and actions that the CLI already uses.

## API endpoints

The Web UI consumes the following endpoints. RBAC is enforced on every call; the third column shows the action checked.

| Endpoint                                               | UI usage                                            | RBAC action enforced                  |
|--------------------------------------------------------|-----------------------------------------------------|---------------------------------------|
| `GET /api/v1/applicationsets`                          | The list page (`/applicationsets`)                  | `applicationsets, get` (per item)     |
| `GET /api/v1/applicationsets/{name}`                   | The details page header and slide-out summary       | `applicationsets, get`                |
| `GET /api/v1/applicationsets/{name}/resource-tree`     | The resource-tree visualization                     | `applicationsets, get`                |
| `GET /api/v1/applicationsets/{name}/events`            | The Events tab in the slide-out panel               | `applicationsets, get`                |
| `GET /api/v1/stream/applicationsets`                   | Live updates on the list page and details page      | `applicationsets, get` (per event)    |
| `POST /api/v1/applicationsets/generate`                | The Preview tab                                     | `applicationsets, create`             |

## RBAC

ApplicationSet RBAC objects are scoped by the project of the **template's** target Application — i.e. `Spec.Template.Spec.Project` — together with the ApplicationSet's namespace and name. This is the same scoping the CLI and direct API clients see; the UI simply inherits it.

### Read paths

Every read endpoint listed above (`Get`, `List`, `ResourceTree`, `ListResourceEvents`, `Watch`) checks `applicationsets, get` against each ApplicationSet before returning it. `List` and `Watch` filter the result set per item, so a user sees only the ApplicationSets they have `get` on.

If a user has `get` permission on an ApplicationSet, they can see it in every read view in the UI: the list page, the details page, the resource tree, the events tab, and the live watch stream.

### Preview

The Preview tab is the one operation that requires more than `get`. It calls `Generate`, which renders candidate Applications from a (possibly user-edited) ApplicationSet spec server-side. Because rendering a preview is the same operation the controller would perform when creating Applications, the API server enforces **`applicationsets, create`** on the project of the template — the same permission required to actually create the rendered Applications. 

A user who can view an ApplicationSet but does not have `create` on its project's template will see a permission-denied response from the Preview tab.
See [Security](Security.md) for the full ApplicationSet RBAC model.

## `status.health` on the ApplicationSet CR

The ApplicationSet controller writes a `status.health` field on each ApplicationSet (with `status` and `message`), computed from the ApplicationSet's `status.conditions`. The UI reads this field through the regular `Get`, `List`, and `Watch` endpoints; no separate health-evaluation API call is made.

The rules the controller applies, in order:

1. If `status.conditions` is empty → **Unknown**
   (`"No status conditions found for ApplicationSet"`).
2. If an `ErrorOccurred` condition has `status: True`
   → **Degraded**, with the condition's message.
3. Otherwise, if a `RolloutProgressing` condition has `status: True`
   → **Progressing**, with the condition's message.
4. Otherwise, if a `ResourcesUpToDate` condition has `status: True`
   → **Healthy**, with the condition's message.
5. Otherwise → **Unknown** (`"Waiting for health status to be determined"`).
