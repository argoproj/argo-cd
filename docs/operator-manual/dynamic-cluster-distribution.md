# Dynamic Cluster Distribution

*Current Status: [Alpha][1] (Since v2.9.0)*


Sharding in Argo CD uses StatefulSet for the application controller. Although the application controller does not have any state to preserve, StatefulSets are used to get predictable hostnames and the serial number in the hostname is used to get the shard id of a particular instance.

But using StatefulSet has the following limitations:

* Any change done to the StatefulSet would cause all the child pods to restart in a serial fashion. This makes scaling up/down of the application controller slow as even existing healthy instances need to be restarted as well. 

* Each shard replica knows about the total number of available shards by evaluating the environment variable `ARGOCD_CONTROLLER_REPLICAS`, which needs to be kept up-to-date with the actual number of available replicas. If the number of replicas in the StatefulSet does not equal the number set in environment variable `ARGOCD_CONTROLLER_REPLICAS`, sharding will not work as intended, leading to both, unused and overused replicas. As this environment variable is set on the StatefulSet and propagated to the pods, all the pods in the StatefulSet need to be restarted in order to pick up the new number of total shards.


Starting v2.9, ArgoCD supports a dynamic cluster distribution feature. In this mechanism, Argo CD sharding uses Deployments for the application controller. 


## Enabling Dynamic Distribution of Clusters

In order to utilize the feature, the StatefulSet of Application Controller needs to be replaced with the Deployment Configuration of Application Controller. To do so, set the number of replicas of StatefulSet and the environment variable `ARGOCD_CONTROLLER_REPLICAS` to 0 disabling all current application controllers and deploy application controller as a deployment.

The manifests `manifests/ha/base/controller-deployment/` can be applied to set the StatefulSet replicas to `0` and deploy application controller as a deployment.

Note the introduction of new environment variable `ARGOCD_CONTROLLER_HEARTBEAT_TIME`. The environment variable is explained in [working of Dynamic Distribution Heartbeat Process](#working-of-dynamic-distribution)


## Working of Dynamic Distribution

Along with the new sharding strategy using Deployments for the Application Controller there is a new mechanism to dynamically associate clusters to specific Controller replicas in the shard.

Application Controller will create a new ConfigMap named `argocd-app-controller-shard-cm` to store the Controller <-> Shard mapping. The mapping would look like below for each shard:

```yaml
ShardNumber    : 0
ControllerName : "argocd-application-controller-hydrxyt"
HeartbeatTime  : "2009-11-17 20:34:58.651387237 +0000 UTC"
```

* `ControllerName`: Stores the hostname of the Application Controller pod
* `ShardNumber` : Stores the shard number managed by the controller pod
* `HeartbeatTime`: Stores the last time this heartbeat was updated.


Controller Shard Mapping is updated in the configMap during each readiness probe check of the pod, that is every 10 seconds (otherwise as configured). The controller will acquire the shard during every iteration of readiness probe check and try to update the ConfigMap with the `HeartbeatTime`. The default `HeartbeatDuration` after which the heartbeat should be updated is `10` seconds. If the ConfigMap was not updated for any controller pod for more than `3 * HeartbeatDuration`, then the readiness probe for the application pod is marked as `Unhealthy`. To increase the default `HeartbeatDuration`, you can set the environment variable `ARGOCD_CONTROLLER_HEARTBEAT_TIME` with the desired value.

The new sharding mechanism does not monitor the environment variable `ARGOCD_CONTROLLER_REPLICAS` but instead reads the replica count directly from the Application Controller Deployment. The controller identifies the change in the number of replicas by comparing the replica count in the Application Controller Deployment and the number of mappings in the `argocd-app-controller-shard-cm` ConfigMap.

In the scenario when the number of Application Controller replicas increases, a new entry is added to the list of mappings in the `argocd-app-controller-shard-cm` ConfigMap and the cluster distribution is triggered to re-distribute the clusters.

In the scenario when the number of Application Controller replicas decreases, the mappings in the `argocd-app-controller-shard-cm` ConfigMap are reset and every controller acquires the shard again thus triggering the re-distribution of the clusters.

