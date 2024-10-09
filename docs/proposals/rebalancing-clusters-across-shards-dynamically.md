---
title: Neat-enhancement-idea
authors:
  - "@ishitasequeira" # Authors' github accounts here.
sponsors:
  - TBD        # List all interested parties here.
reviewers:
  - "@alexmt"
  - TBD
approvers:
  - "@alexmt"
  - TBD

creation-date: yyyy-mm-dd
last-updated: yyyy-mm-dd
---

# Neat Enhancement Idea

Rebalance clusters across shards automatically on changes to the number of available shards.


## Open Questions [optional]

This is where to call out areas of the design that require closure before deciding to implement the
design.


## Summary

Current implementation of sharding uses StatefulSet for the application controller and the goal is to move towards an agile stateless Deployment. Although the application controller does not have any state to preserve, stateful sets were used to get predictable hostnames and the serial number in the hostname was used to get the shard id of a particular instance.  Using StatefulSet has the following limitations:

Any change done to the StatefulSet would cause all the child pods to restart in a serial fashion. This makes scaling up/down of the application controller slow as even existing healthy instances need to be restarted as well.
Scaling up or down happens one at a time. If there are 10 instances and if scaled to 20, then the scaling happens one at a time, causing considerable delay for the scaling to complete.

Each shard replica knows about the total number of available shards by evaluating the environment variable ARGOCD_CONTROLLER_REPLICAS, which needs to be kept up-to-date with the actual number of available replicas (shards). If the number of replicas does not equal the number set in ARGOCD_CONTROLLER_REPLICAS, sharding will not work as intended, leading to both, unused and overused replicas. As this environment variable is set on the StatefulSet and propagated to the pods, all the pods in the StatefulSet need to be restarted in order to pick up the new number of total shards.

The current sharding mechanism relies on predictable pod names for the application controller to determine which shard a given replica should impersonate, e.g. the first replica of the StatefulSet (argocd-application-controller-0) will be the first shard, the second replica (argocd-application-controller-1) will be the second and so forth. 

## Motivation

If the number of available shards is changed (i.e. one or more application controller replicas are added or removed), all pods in the statefulset have to be restarted so that the managed clusters are redistributed over the available shards. Additionally, the application controller workload is deployed as a StatefulSet, which is not intended for dynamic horizontal scaling.

### Goals

- Improve the application controller’s ability to scale horizontally with a growing number of clusters
- Remove the need to run application controller as a StatefulSet workload

### Non-Goals

- Expand the scope of sharding to other assets than clusters (e.g. applications)
- Make a single shard highly available (e.g. by having 2 or more replicas by shard)

## Proposal

### Why use Deployments instead of StatefulSet:
StatefulSet is a Kubernetes resource that manages multiple pods that have unique identities, and are not interchangeable (unlike a regular Kubernetes Deployment, in which pods are stateless and can be destroyed and recreated as often as needed). 

Stateless applications scale horizontally very easily as compared to stateful applications due to the fact that infrastructure allows adding as many computing resources as needed. Changing the StatefulSet to Deployments for Application Controller will allow us to dynamically scale the replicas without restarting existing application controller pods. Also, the shard to application controller assignment would help in making sure the shards are scaled and distributed across the available healhty replicas of application controllers.

### Distributing shards among Application Controllers:

Inorder to be able to accurately know which shards are being managed by which application-controller, especially in scenarios of redistribution of load, addition/removal of `application controller`, etc., we would need to have a mechanism to assign clusters to the shards. 

In most scenarios, the service account used by the application controller has read access to all the resources in the cluster. Thus, instead of setting the environment variable ARGOCD_CONTROLLER_REPLICAS representing the number of replicas, the number of replicas can be read directly from the number of healthy replicas of the application controller deployment.

For other scenarios, some users install controller with only `argocd-application-controller-role` role and use it to manage remote clusters only. In this case, we would need to update the `argocd-application-controller-role` role and allow controller inspect it's own deployment and find out the number of replicas.

The application controllers will claim one of the available shards by checking which shard is not present in the ConfigMap or is assigned to an unhealthy controller. We will store the assignment list of Application Controller to Shard in ConfigMap. The mapping of Application Controller to Shard will store the below information:

* Name/Id of the shard
* Name of the Application Controller currently managing the shard
* Last time of successful update to ConfigMap (Heartbeat)

The mapping will be updated in ConfigMap every X (heartbeat interval) seconds with the help of heartbeat process performed by every application controller. If the heartbeat was not performed by the application controller for a certain time, the application controller is assumed to be unhealthy and the number of healthy/managed shards would be reduced, that is, the number of healthy replicas of the application controller deployment changes.

The heartbeat interval will be a configurable parameter initialized while setting up the application controller. This way, users will be able to control the frequency at which they want the heartbeat process to take place.

As part of the readiness probe, we will also add a check whether application controller was able to claim a shard successfully or not. If the shard claim failed, the readiness probe will fail marking the controller as unhealthy. Anytime the number of healthy replicas of application controllers is different from the number of application controllers to shard mappings, we would re-distribute the clusters among the healthy replicas again. We can summarize the above statement using the below formula:

```
Number of Replicas ≠ Count of {Application Controller, Shard} mapping
```

The below logic can be used to perform application controller to shard assignment:

1) If a new application controller is added, that is, a new shard is added, we would perform the re-distribution of clusters among the shards with the existing sharding algorithm being used.

2) In scenarios when one of the application controllers is identified to be unhealthy, we will not trigger the re-ditribution of clusters across shards. The new instance of the application controller will claim this unassigned shard and start managing the shard. 

How will this work? 
* The application controller will query the ConfigMap for the status of all the application controllers and last updated heartbeat timestamps.
* It will check if any application controller is flagged as Unhealthy or has not updated its status in ConfigMap during the heartbeat process for a certain period of time.
* If the status for an application controller was already flagged as Unhealthy, we will not re-trigger the redistribution of clusters across healthy shards. The new application controller will come online and try to claim this unassigned shard.
* If the status is not flagged and an application controller has not updated the last active timestamp in a long time, then we mark the Application Controller as Unhealthy and unassign the shard in the ConfigMap. 

*Note:* We will continue to use the cluster to shard assignment approach being used today.

### Pros
* Every Application Controller would be able to take action on finding the distribution of load.
* Every Application Controller will monitor the status of Unhealthy shard and would be able to take action or flag for action.

### Cons

* ~~Possibility of race conditions while flagging the shard as Unhealthy during the heartbeat process. Although this can be handled using the [distributed locks](https://redis.io/docs/manual/patterns/distributed-locks/) in Redis.~~
As we are using ConfigMap, this Con get's removed. Kubernetes would give conflict errors in case multiple edits are tried on the ConfigMap at the same time. We can leverage this error messages to avoid race conditions.

* ~~In scenarios when Redis becomes unavailable, the heartbeat mechanism will pause working till the redis comes back online again. This will also pause the dynamic redistribution of clusters till Redis comes back online. The redistribution of clusters will be triggered again when Redis comes back online.~~ We would not see this issue by using ConfigMap instead of Redis.


### Security Considerations

* This would be a breaking change of converting StatefulSets to Deployments. Any automation done by customers which is based on the assumption that the controller is modelled as a StatefulSet would break with this change. 

* ~~We would rely on Redis to store the current Application Controller to Shard mapping. In case the Redis is not available, it would not affect the regular working of ArgoCD. The dynamic distribution of clusters among healthy shards would stop working with the heartbeat process till Redis comes back up online, but the application controllers will continue managing their workloads.~~ We would not rely on Redis by using ConfigMap avoiding this issue.


### Upgrade / Downgrade Strategy

* Working ArgoCD itself should not affected. An initial restart of all the application controller pods is expected when we switch from StatefulSet to Deployment or vice-versa.

* There would be some initial delays in the reconciliation process during the transistion from StatefulSet to Deployment. If someone is not using sharding at all, they should not face any issues.

## Alternatives

An alternative approach would be to use Leader Election strategy. By implementing leader election, one of the healthy application controllers will be appointed as leader. The leader controller will be responsible for assigning clusters to the shards and balancing load across the shards.

The leader controller will continue sending heartbeats to every replica controller and monitor the health of the controllers. In case one of the replica controllers crashes, the leader will distribute the shards managed by the unhealthy replica among the healthy replicas. 

If the leader goes down, the leader election process will be initiated among the healthy candidates and one of the candidates will be marked as leader who will perform the heartbeat process and redistribution of resources.

One of the possible examples for selecting the leader is by checking the load handled by each healthy candidate and selecting the candidate which has the least load / number of resources running on it.

### Pros of Leader Election

* We can refrain from performing multiple calls to ConfigMap about the load and status of the shards and store it in a local cache within the leader while updating data in ConfigMap on a timely manner (for e.g. every 10 mins). 
* Single leaders can easily offer clients consistency because they can see and control all the changes made to the state of the system.


### Cons of Leader Election
* A single leader is a single point of failure. If the leader becomes bad, that is, does not distribute clusters properly across shards, it is very difficult to identify or fix the bad behavior and can become a single point of failure
* A single leader means a single point of scaling, both in data size and request rate. When a leader-elected system needs to grow beyond a single leader, it requires a complete re-architecture.
