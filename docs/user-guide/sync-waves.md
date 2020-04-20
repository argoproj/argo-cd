# Sync Phases and Waves

>v1.1

<iframe width="560" height="315" src="https://www.youtube.com/embed/zIHe3EVp528" frameborder="0" allow="accelerometer; autoplay; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

Argo CD executes a sync operation in a number of steps. At a high-level, there are three phases *pre-sync*, *sync* and *post-sync*.  

Within each phase you can have one or more waves, than allows you to ensure certain resources are healthy before subsequent resources are synced.   

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
* The wave they are in (lower values first)
* By kind in the following order
  1. Namespace
  1. ResourceQuota
  1. LimitRange
  1. PodSecurityPolicy
  1. PodDisruptionBudget
  1. Secret
  1. ConfigMap
  1. StorageClass
  1. PersistentVolume
  1. PersistentVolumeClaim
  1. ServiceAccount
  1. CustomResourceDefinition
  1. ClusterRole
  1. ClusterRoleBinding
  1. Role
  1. RoleBinding
  1. Service
  1. DaemonSet
  1. Pod
  1. ReplicationController
  1. ReplicaSet
  1. Deployment
  1. StatefulSet
  1. Job
  1. CronJob
  1. Ingress
  1. APIService
* By name 

It then determines which the number of the next wave to apply. This is the first number where any resource is out-of-sync or unhealthy.
 
It applies resources in that wave. 

It repeats this process until all phases and waves are in in-sync and healthy.

Because an application can have resources that are unhealthy in the first wave, it may be that the app can never get to healthy.
