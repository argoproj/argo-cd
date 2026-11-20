# Lua Scripting

Argo CD uses Lua scripts for [custom health checks](health.md#custom-health-checks) and
[resource actions](resource_actions.md#custom-resource-actions). Scripts run in
[GopherLua](https://github.com/yuin/gopher-lua), a Go implementation of Lua 5.1 with support for the
Lua 5.2 `goto` statement. Use Lua 5.1 syntax and libraries when writing scripts.

## Script inputs and outputs

Argo CD provides the following global variables:

| Variable | Contents |
| --- | --- |
| `obj` | The Kubernetes resource being evaluated or acted on, represented as a Lua table. For example, `obj.metadata.name` is its name. |
| `actionParams` | A table of parameter names and string values supplied to a resource action. It is empty for health checks, action discovery, and actions invoked without parameters. |

JSON objects become tables with string keys, and arrays become tables indexed from **1**.
For example, the first status condition is `obj.status.conditions[1]`. Strings, numbers, and
booleans retain their respective Lua types. Missing or null fields evaluate to `nil`; check
that `obj.status` and `obj.status.conditions` exist before accessing their children.
Use bracket notation for keys containing punctuation, such as
`obj.metadata.annotations["example.com/status"]`.

Action parameter values are strings even when a parameter is declared as a number or boolean.
Use `tonumber(actionParams["replicas"])` for numeric parameters, and compare a boolean parameter
to `"true"` explicitly. In Lua, the string `"false"` is truthy. See
[action parameters](resource_actions.md#action-parameters) for their definitions.

Each type of script has a different return value:

| Script | Return value |
| --- | --- |
| Health check (`health.lua`) | A table containing `status` and optional `message` and `deletionMessage` fields. See [custom health checks](health.md#custom-health-checks) and [terminating resources](health.md#terminating-resources). |
| Action discovery (`discovery.lua`) | A table keyed by action name, with the action's properties, such as `disabled`. |
| Resource action (`action.lua`) | The modified `obj`, or a list of resources and operations. See [resource action types](resource_actions.md#custom-resource-action-types). |

## Available libraries

By default, scripts have access to these libraries:

| Library | Examples |
| --- | --- |
| Base | `pairs`, `ipairs`, `type`, `tonumber`, `tostring`, `error` |
| Package | `require` |
| Table | `table.insert`, `table.remove`, `table.sort`, `table.concat` |
| Argo CD's limited OS module | `time` and `date`, available through `local os = require("os")` |

Other standard libraries, such as `string`, `math`, and `io`, are not loaded by default.
With standard libraries disabled, the limited OS module provides only `time` and `date`;
it does not provide functions such as `execute` or `getenv`. Import it explicitly with
`require("os")`. For example, this resource action records a UTC timestamp in an annotation:

```lua
local os = require("os")
obj.metadata.annotations = obj.metadata.annotations or {}
obj.metadata.annotations["example.com/action-time"] = os.date("!%Y-%m-%dT%H:%M:%SZ")
return obj
```

### Enabling standard libraries for health checks

The `resource.customizations.useOpenLibs.<group>_<kind>` setting in `argocd-cm` enables the
GopherLua standard libraries for a custom health check. For a resource in the core API group,
omit `<group>_`, as in `resource.customizations.useOpenLibs.ConfigMap`.
ConfigMap `data` values must be strings, so quote `"true"`.

Enabling standard libraries exposes GopherLua's full global `os` library, including
`os.execute` and `os.getenv`. The module returned by `require("os")` remains Argo CD's
limited module containing only `time` and `date`.

| Script source | Standard library availability |
| --- | --- |
| Health check configured in `argocd-cm` | Default libraries above, unless `useOpenLibs` is enabled for the resource type. |
| Health check bundled with Argo CD | Standard libraries are enabled. |
| Resource action or discovery script, whether configured or bundled | Default libraries above. The health check's `useOpenLibs` setting does not apply. |

Bundled scripts are embedded from the repository's `resource_customizations` directory when
Argo CD is built. Adding or mounting a script file in a running container does not register a
customization. Use `argocd-cm` to configure scripts in an existing installation, or follow the
contribution guides to include them in an Argo CD build.

For a local example, save this configuration as `argocd-cm.yaml`. Its health check uses the
`string.lower` function to inspect a ConfigMap's `ready` value:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
data:
  resource.customizations.useOpenLibs.ConfigMap: "true"
  resource.customizations.health.ConfigMap: |
    local hs = {status = "Progressing", message = "Waiting for ready=true"}
    if obj.data ~= nil and string.lower(obj.data.ready or "") == "true" then
      hs.status = "Healthy"
      hs.message = "ConfigMap is ready"
    end
    return hs
```

Save the resource to evaluate as `configmap.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: example
data:
  ready: "TRUE"
```

Run the health check locally:

```bash
argocd admin settings resource-overrides health configmap.yaml --argocd-cm-path argocd-cm.yaml
```

Expected output:

```text
STATUS: Healthy
MESSAGE: ConfigMap is ready
```

Without `useOpenLibs`, this script fails when it tries to access `string.lower`.

## Execution and testing

Each script invocation receives a fresh Lua state. Global variables set in one invocation
are not preserved for the next. Argo CD sets a one-second execution timeout on the Lua VM;
keep scripts short and avoid unbounded loops.

Use the local CLI commands to evaluate scripts against resource YAML:

* [`health`](../user-guide/commands/argocd_admin_settings_resource-overrides_health.md) evaluates a health check.
* [`list-actions`](../user-guide/commands/argocd_admin_settings_resource-overrides_list-actions.md) evaluates action discovery.
* [`run-action`](../user-guide/commands/argocd_admin_settings_resource-overrides_run-action.md) evaluates an action, with optional `--param name=value` arguments.

For contributions, follow the test fixture instructions for
[health checks](health.md#way-2-contribute-a-custom-health-check) and
[resource actions](resource_actions.md#contributing-a-custom-resource-action).
