---
name: new-resource-customization
description: Scaffold a new Lua resource customization (health check or action) for a Kubernetes resource type. Usage /new-resource-customization <api-group>/<Kind>
args: "<api-group>/<Kind>"
---

Scaffold Lua resource customization files for a Kubernetes resource type in the Argo CD codebase.

## Parse the Argument

The argument should be in format `<api-group>/<Kind>`, e.g.:
- `apps/Deployment`
- `batch/CronJob`
- `argoproj.io/Rollout`

The base directory is `resource_customizations/<api-group>/<Kind>/`.

## Ask What to Scaffold

Ask the user which files to create:
1. **Health check** (`health.lua`) — Determines resource health status
2. **Custom action** (`actions/`) — Adds actions to the resource in the UI
3. **Both**

## Health Check Template (health.lua)

```lua
local hs = {}
if obj.status ~= nil then
  -- TODO: Implement health logic based on resource status fields
  -- Available statuses: Healthy, Degraded, Progressing, Suspended, Missing, Unknown
  hs.status = "Progressing"
  hs.message = "Waiting for resource to become ready"
end
return hs
```

Ask the user about the resource's status fields and expected health conditions to fill in the logic.

## Action Discovery Template (actions/discovery.lua)

```lua
local actions = {}
-- TODO: Define available actions
actions["action-name"] = {
  ["disabled"] = false,
  ["iconClass"] = "fa fa-undo",
}
return actions
```

## Action Implementation Template (actions/<name>/action.lua)

```lua
-- TODO: Implement the action
-- Modify obj and return it
return obj
```

## Reference Existing Customizations

Before creating, check existing patterns:
- Look at `resource_customizations/apps/Deployment/` for a canonical health + actions example
- Look at similar resources in the same api-group for consistency
- Match the status field patterns from the actual Kubernetes resource spec

## After Scaffolding

Tell the user to:
1. Run `make build-local` to verify no compilation issues
2. Add test cases if the customization has complex logic
3. Check the Argo CD docs for the Lua scripting API reference
