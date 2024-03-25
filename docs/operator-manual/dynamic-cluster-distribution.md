# Dynamic Cluster Distribution

*Current Status: [Alpha][1] (Since v2.9.0)*

By default, clusters are assigned to shards indefinitely. For users of the default, hash-based sharding algorithm, this 
static assignment is fine: shards will always be roughly-balanced by the hash-based algorithm. But for users of the 
[round-robin](high_availability.md#argocd-application-controller) or other custom shard assignment algorithms, this 
static assignment can lead to unbalanced shards when replicas are added or removed.

Starting v2.9, Argo CD supports a dynamic cluster distribution feature. When replicas are added or removed, the sharding
algorithm is re-run to ensure that the clusters are distributed according to the algorithm. If the algorithm is 
well-balanced, like round-robin, then the shards will be well-balanced.

Previously, the shard count was set via the `ARGOCD_CONTROLLER_REPLICAS` environment variable. Changing the environment 
variable forced a restart of all application controller pods. Now, the shard count is set via the `replicas` field of the deployment, 
which does not require a restart of the application controller pods. 

## Enabling Dynamic Distribution of Clusters

This feature is disabled by default while it is in alpha. In order to utilize the feature, the manifests `manifests/ha/base/controller-deployment/` can be applied as a Kustomize overlay. This overlay sets the StatefulSet replicas to `0` and deploys the application controller as a Deployment. Also, you must set the environment `ARGOCD_ENABLE_DYNAMIC_CLUSTER_DISTRIBUTION` to true when running the Application Controller as a deployment.

!!! important
    The use of a Deployment instead of a StatefulSet is an implementation detail which may change in future versions of this feature. Therefore, the directory name of the Kustomize overlay may change as well. Monitor the release notes to avoid issues.

Note the introduction of new environment variable `ARGOCD_CONTROLLER_HEARTBEAT_TIME`. The environment variable is explained in [working of Dynamic Distribution Heartbeat Process](#working-of-dynamic-distribution)

## Working of Dynamic Distribution

To accomplish runtime distribution of clusters, the Application Controller uses a ConfigMap to associate a controller 
pod with a shard number and a heartbeat to ensure that controller pods are still alive and handling their shard, in 
effect, their share of the work.

The Application Controller will create a new ConfigMap named `argocd-app-controller-shard-cm` to store the Controller <-> Shard mapping. The mapping would look like below for each shard:

```yaml
ShardNumber    : 0
ControllerName : "argocd-application-controller-hydrxyt"
HeartbeatTime  : "2009-11-17 20:34:58.651387237 +0000 UTC"
```

* `ControllerName`: Stores the hostname of the Application Controller pod
* `ShardNumber` : Stores the shard number managed by the controller pod
* `HeartbeatTime`: Stores the last time this heartbeat was updated.

Controller Shard Mapping is updated in the ConfigMap during each readiness probe check of the pod, that is every 10 seconds (otherwise as configured). The controller will acquire the shard during every iteration of readiness probe check and try to update the ConfigMap with the `HeartbeatTime`. The default `HeartbeatDuration` after which the heartbeat should be updated is `10` seconds. If the ConfigMap was not updated for any controller pod for more than `3 * HeartbeatDuration`, then the readiness probe for the application pod is marked as `Unhealthy`. To increase the default `HeartbeatDuration`, you can set the environment variable `ARGOCD_CONTROLLER_HEARTBEAT_TIME` with the desired value.

The new sharding mechanism does not monitor the environment variable `ARGOCD_CONTROLLER_REPLICAS` but instead reads the replica count directly from the Application Controller Deployment. The controller identifies the change in the number of replicas by comparing the replica count in the Application Controller Deployment and the number of mappings in the `argocd-app-controller-shard-cm` ConfigMap.

In the scenario when the number of Application Controller replicas increases, a new entry is added to the list of mappings in the `argocd-app-controller-shard-cm` ConfigMap and the cluster distribution is triggered to re-distribute the clusters.

In the scenario when the number of Application Controller replicas decreases, the mappings in the `argocd-app-controller-shard-cm` ConfigMap are reset and every controller acquires the shard again thus triggering the re-distribution of the clusters.

[1]: https://github.com/argoproj/argoproj/blob/master/community/feature-status.md
