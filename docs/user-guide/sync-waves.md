# Sync Phases and Waves

Argo CD executes a sync operation in a number of steps:

At a high-level, there are three phases *pre-sync*, *sync* and *post-sync*. These are executed in order, but within each phase you can have one or more waves, than allows you to ensure certain resources are healthy before subsequent resources are synced.   

Within a sync phase Argo CD can sync resources in phases and then waves within each phase, waiting until every resource in preceding waves are in-sync and healthy before syncing subsequent waves.

One use case would be that deployment B should not start until deployment A is fully running with a new image. You could put deployment A in the first wave and deployment B in the second wave. 

## How Do I Configure Phases?

By default resources are in the "sync" phase. Pre-sync and post-sync phases only apply to hooks. Therefore you must apply the hook annotation:

```yaml
metadata:
  annotations:
    argocd.argoproj.io/hook: PreSync
```

[Read more about hooks](resource_hooks.md).

## How Do I Configure Waves?

Specify the wave of a resources using the following annotation:

```yaml
metadata:
  annotations:
    argocd.argoproj.io/sync-wave: "5"
```

Hooks and resources are assigned to wave zero by default. The wave can be negative, so you can create a wave that runs before all other resources.

## How Does It Work?

When Argo CD starts a sync, it orders the resources in the following precedence:

* The phase
* The wave they are in (lower values first)
* By kind (e.g. namespaces first)
* By name 

It then determines which the number of the next wave to apply. This is the first number where any resource is out-of-sync or unhealthy.
 
It applies resources in that wave. 

It repeats this process until all phases and waves are in in-sync and healthy.

Because an application can have resources that are unhealthy in the first wave, it may be that the app can never get to healthy.
    
## How Do I Know If An App In Doing A Sync With Waves?

TODO
