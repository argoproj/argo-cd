# Resource Actions

## Overview
Argo CD allows operators to define custom actions which users can perform on specific resource types. This is used internally to provide actions like `restart` for a `DaemonSet`, or `retry` for an Argo Rollout.

Operators can add actions to custom resources in form of a Lua script and expand those capabilities.

## Built-in Actions

The following are actions that are built-in to Argo CD. Each action name links to its Lua script definition:

{!docs/operator-manual/resource_actions_builtin.md!}

See the [RBAC documentation](rbac.md#the-action-action) for information on how to control access to these actions.

## Custom Resource Actions

Argo CD supports custom resource actions written in [Lua](https://www.lua.org/). This is useful if you:

* Have a custom resource for which Argo CD does not provide any built-in actions.
* Have a commonly performed manual task that might be error prone if executed by users via `kubectl`

The resource actions act on a single object.

You can define your own custom resource actions in the `argocd-cm` ConfigMap.

### Custom Resource Action Types

#### An action that modifies the source resource

This action modifies and returns the source resource.
This kind of action was the only one available till 2.8, and it is still supported.

#### An action that produces a list of new or modified resources

**An alpha feature, introduced in 2.8.**

This action returns a list of impacted resources, each impacted resource has a K8S resource and an operation to perform on.   
Currently supported operations are "create" and "patch", "patch" is only supported for the source resource.   
Creating new resources is possible, by specifying a "create" operation for each such resource in the returned list.  
One of the returned resources can be the modified source object, with a "patch" operation, if needed.   
See the definition examples below.

### Define a Custom Resource Action in `argocd-cm` ConfigMap

Custom resource actions can be defined in `resource.customizations.actions.<group_kind>` field of `argocd-cm`. Following example demonstrates a set of custom actions for `CronJob` resources, each such action returns the modified CronJob. 
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

Each action name must be represented in the list of `definitions` with an accompanying `action.lua` script to control the resource modifications. The `obj` is a global variable which contains the resource. Each action script returns an optionally modified version of the resource. In this example, we are simply setting `.spec.suspend` to either `true` or `false`.

By default, defining a resource action customization will override any built-in action for this resource kind. If you want to retain the built-in actions, you can set the `mergeBuiltinActions` key to `true`. Your custom actions will have precedence over the built-in actions.
```yaml        
resource.customizations.actions.argoproj.io_Rollout: |
  mergeBuiltinActions: true
  discovery.lua: |
    actions = {}
    actions["do-things"] = {}
    return actions
  definitions:
  - name: do-things
    action.lua: |
      return obj		
```

#### Creating new resources with a custom action

!!! important
    Creating resources via the Argo CD UI is an intentional, strategic departure from GitOps principles. We recommend 
    that you use this feature sparingly and only for resources that are not part of the desired state of the 
    application.

The resource the action is invoked on would be referred to as the `source resource`.  
The new resource and all the resources implicitly created as a result, must be permitted on the AppProject level, otherwise the creation will fail.

##### Creating a source resource child resources with a custom action

If the new resource represents a k8s child of the source resource, the source resource ownerReference must be set on the new resource.  
Here is an example Lua snippet, that takes care of constructing a Job resource that is a child of a source CronJob resource - the `obj` is a global variable, which contains the source resource:

```lua
-- ...
ownerRef = {}
ownerRef.apiVersion = obj.apiVersion
ownerRef.kind = obj.kind
ownerRef.name = obj.metadata.name
ownerRef.uid = obj.metadata.uid
job = {}
job.metadata = {}
job.metadata.ownerReferences = {}
job.metadata.ownerReferences[1] = ownerRef
-- ...
```

##### Creating independent child resources with a custom action

If the new resource is independent of the source resource, the default behavior of such new resource is that it is not known by the App of the source resource (as it is not part of the desired state and does not have an `ownerReference`).  
To make the App aware of the new resource, the `app.kubernetes.io/instance` label (or other ArgoCD tracking label, if configured) must be set on the resource.   
It can be copied from the source resource, like this:

```lua
-- ...
newObj = {}
newObj.metadata = {}
newObj.metadata.labels = {}
newObj.metadata.labels["app.kubernetes.io/instance"] = obj.metadata.labels["app.kubernetes.io/instance"]
-- ...
```   

While the new resource will be part of the App with the tracking label in place, it will be immediately deleted if auto prune is set on the App.   
To keep the resource, set `Prune=false` annotation on the resource, with this Lua snippet:

```lua
-- ...
newObj.metadata.annotations = {}
newObj.metadata.annotations["argocd.argoproj.io/sync-options"] = "Prune=false"
-- ...
```

(If setting `Prune=false` behavior, the resource will not be deleted upon the deletion of the App, and will require a manual cleanup).

The resource and the App will now appear out of sync - which is the expected ArgoCD behavior upon creating a resource that is not part of the desired state.

If you wish to treat such an App as a synced one, add the following resource annotation in Lua code:

```lua
-- ...
newObj.metadata.annotations["argocd.argoproj.io/compare-options"] = "IgnoreExtraneous"
-- ...
```

#### An action that produces a list of resources - a complete example:

```yaml
resource.customizations.actions.ConfigMap: |
  discovery.lua: |
    actions = {}
    actions["do-things"] = {}
    return actions
  definitions:
  - name: do-things
    action.lua: |
      -- Create a new ConfigMap
      cm1 = {}
      cm1.apiVersion = "v1"
      cm1.kind = "ConfigMap"
      cm1.metadata = {}
      cm1.metadata.name = "cm1"
      cm1.metadata.namespace = obj.metadata.namespace
      cm1.metadata.labels = {}
      -- Copy ArgoCD tracking label so that the resource is recognized by the App
      cm1.metadata.labels["app.kubernetes.io/instance"] = obj.metadata.labels["app.kubernetes.io/instance"]
      cm1.metadata.annotations = {}
      -- For Apps with auto-prune, set the prune false on the resource, so it does not get deleted
      cm1.metadata.annotations["argocd.argoproj.io/sync-options"] = "Prune=false"	  
      -- Keep the App synced even though it has a resource that is not in Git
      cm1.metadata.annotations["argocd.argoproj.io/compare-options"] = "IgnoreExtraneous"		  
      cm1.data = {}
      cm1.data.myKey1 = "myValue1"
      impactedResource1 = {}
      impactedResource1.operation = "create"
      impactedResource1.resource = cm1

      -- Patch the original cm
      obj.metadata.labels["aKey"] = "aValue"
      impactedResource2 = {}
      impactedResource2.operation = "patch"
      impactedResource2.resource = obj

      result = {}
      result[1] = impactedResource1
      result[2] = impactedResource2
      return result		  
```