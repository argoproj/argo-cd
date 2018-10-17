# Resource Health

## Overview
Argo CD provides built-in health assessment for several standard Kubernetes types, which is then
surfaced to the overall Application health status as a whole. The following checks are made for
specific types of kuberentes resources:

### Deployment, ReplicaSet, StatefulSet DaemonSet
* Observed generation is equal to desired generation.
* Number of **updated** replicas equals the number of desired replicas.

### Service
* If service type is of type `LoadBalancer`, the `status.loadBalancer.ingress` list is non-empty,
with at least one value for `hostname` or `IP`.

### Ingress
* The `status.loadBalancer.ingress` list is non-empty, with at least one value for `hostname` or `IP`.

### PersistentVolumeClaim
* The `status.phase` is `Bound`
