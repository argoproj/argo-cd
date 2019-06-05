# High Availability

Argo CD is largely stateless, all data is stored in Kubernetes objects. We use Redis as a throw away cache (you can loose it without loss of service). The HA version allows you to perform zero-downtime node upgrades.

A set HA of manifests are provided for users who wish to run Argo CD in a highly available manner. This runs more containers and Redis in HA mode.

[Manifests ⧉](https://github.com/argoproj/argo-cd/tree/master/manifests) 

!!! note
    The HA installation will require at least three different nodes due to pod anti-affinity roles in the specs.
 
