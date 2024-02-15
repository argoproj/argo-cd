# Sync Phases and Waves

>v1.1

<iframe width="560" height="315" src="https://www.youtube.com/embed/zIHe3EVp528" frameborder="0" allow="accelerometer; autoplay; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

Argo CD executes a sync operation in a number of steps. At a high-level, there are three phases *pre-sync*, *sync* and *post-sync*.  

Within each phase you can have one or more waves, that allows you to ensure certain resources are healthy before subsequent resources are synced.   

## How Do I Configure Phases?

Pre-sync and post-sync can only contain hooks. Apply the hook annotation:

```yaml
metadata:
  annotations:
    argocd.argoproj.io/hook: PreSync
```

[Read more about hooks](resource_hooks.md).

## How Do I Configure Waves?

Specify the wave using the following annotation:

```yaml
metadata:
  annotations:
    argocd.argoproj.io/sync-wave: "5"
```

Hooks and resources are assigned to wave zero by default. The wave can be negative, so you can create a wave that runs before all other resources.

## How Does It Work?

When Argo CD starts a sync, it orders the resources in the following precedence:

* The phase
* The wave they are in (lower values first for creation & updation and higher values first for deletion)
* By kind (e.g. [namespaces first and then other Kubernetes resources, followed by custom resources](https://github.com/argoproj/gitops-engine/blob/bc9ce5764fa306f58cf59199a94f6c968c775a2d/pkg/sync/sync_tasks.go#L27-L66))
* By name 

It then determines the number of the next wave to apply. This is the first number where any resource is out-of-sync or unhealthy.
 
It applies resources in that wave. 

It repeats this process until all phases and waves are in-sync and healthy.

Because an application can have resources that are unhealthy in the first wave, it may be that the app can never get to healthy.

During pruning of resources, resources from higher waves are processed first before moving to lower waves. If, for any reason, a resource isn't removed/pruned in a wave, the resources in next waves won't be processed. This is to ensure proper resource cleanup between waves.

Note that there's currently a delay between each sync wave in order give other controllers a chance to react to the spec change
that we just applied. This also prevent Argo CD from assessing resource health too quickly (against the stale object), causing
hooks to fire prematurely. The current delay between each sync wave is 2 seconds and can be configured via environment
variable `ARGOCD_SYNC_WAVE_DELAY`.
