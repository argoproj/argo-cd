# High Availability

Argo CD is largely stateless, all data is stored in Kubernetes objects. We use Redis as a throw away cache (you can lose it without loss of service). The HA version allows you to perform zero-downtime node upgrades.

A set HA of manifests are provided for users who wish to run Argo CD in a highly available manner. This runs more containers, and run Redis in HA mode.

[Manifests ⧉](https://github.com/argoproj/argo-cd/tree/master/manifests) 

!!! note
    The HA installation will require at least three different nodes due to pod anti-affinity roles in the specs.
 
## Scaling Up

To scale up, typcially you need to increase the number of replicas of the `argocd-repo-server` (many apps in few repos. You can increase the number of replicas of the `argocd-server` (e.g. support more UI load). 

The `argocd-application-controller` must not be increased because two servers will fight. The `argocd-dex-server` runs uses an in-memory database, two or more instances would have different databases and fail.

change me
