# High Availability

Argo CD is largely stateless, all data is persisted as Kubernetes objects, which in turn is stored in Kubernetes' etcd. Redis is only used as a throw-away cache and can be lost. When lost, it will be rebuilt without loss of service.

A set HA of manifests are provided for users who wish to run Argo CD in a highly available manner. This runs more containers, and run Redis in HA mode.

[Manifests ⧉](https://github.com/argoproj/argo-cd/tree/master/manifests) 

!!! note
    The HA installation will require at least three different nodes due to pod anti-affinity roles in the specs.
 
## Scaling Up

You might scale up some Argo CD services in the following circumstances:

* The `argocd-repo-server` can scale up when there is too much contention on a single git repo (e.g. many apps defined in a single git repo).
* The `argocd-server` can scale up to support more front-end load.

All other services should run with their pre-determined number of replicas. The `argocd-application-controller` must not be increased because multiple controllers will fight. The `argocd-dex-server` uses an in-memory database, and two or more instances would have inconsistent data. `argocd-redis` is pre-configured with the understanding of only three total redis servers/sentinels.
