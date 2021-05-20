# Resource Actions

## Overview
Argo CD allows operators to define custom actions which users can perform on specific resource types. This is used internally to provide actions like `restart` for a `DaemonSet`, or `retry` for an Argo Rollout.

Operators can add actions to custom resources in form of a Lua script and expand those capabilities.

## Custom Resource Actions

Argo CD supports custom resource actions written in [Lua](https://www.lua.org/). This is useful if you:

    * Have a custom resource for which Argo CD does not provide any built-in actions.
    * Have a commonly performed manual task that might be error prone if executed by users via `kubectl`


You can define your own custom resource actions in the `argocd-cm` ConfigMap.

### Define a Custom Resource Action in `argocd-cm` ConfigMap

Custom resource actions can be defined in `resource.customizations.actions.<group_kind>` field of `argocd-cm`. Following example demonstrates a set of custom actions for `CronJob` resources. 
The customizations key is in the format of `resource.customizations.actions.<apiGroup_Kind>`.

```yaml
resource.customizations.actions.batch_CronJob: |
  discovery.lua: |
    actions = {}
    actions["suspend"] = {["disabled"] = true}
    actions["resume"] = {["disabled"] = true}
  
    local suspend = false
    if obj.spec.suspend ~= nil then
        suspend = obj.spec.suspend
    end
    if suspend then
        actions["resume"]["disabled"] = false
    else
        actions["suspend"]["disabled"] = false
    end
    return actions
  definitions:
  - name: suspend
    action.lua: |
      obj.spec.suspend = true
      return obj
  - name: resume
    action.lua: |
      if obj.spec.suspend ~= nil and obj.spec.suspend then
          obj.spec.suspend = false
      end
      return obj
```

The `discovery.lua` script must return a table where the key name represents the action name. You can optionally include logic to enable or disable certain actions based on the current object state.

Each action name must be represented in the list of `definitions` with an accompanying `action.lua` script to control the resource modifications. The `obj` is a global variable which contains the resource. Each action script must return an optionally modified version of the resource. In this example, we are simply setting `.spec.suspend` to either `true` or `false`.
