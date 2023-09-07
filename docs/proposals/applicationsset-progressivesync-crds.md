---
title: SyncStragy CRDs for ApplicationSet progressive sync
authors:
  - "@alexymantha" # Authors' github accounts here.
sponsors:
  - "@agaudreault-jive" # List all interested parties here.
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2023-09-07
last-updated: 2023-09-07
---

# SyncStragy CRDs for ApplicationSet progressive sync

## Open Questions [optional]

- Should we find a way to make these CRDs opt-in instead of being bundled with the main CRDs? These CRDs will only be used for the progressive sync feature which is used by a minority of ArgoCD users as of now.

## Summary

Add a way to define a sync strategy that can be reused across multiple ApplicationSets.

## Motivation

The RollingSync strategy for Progressive Sync requires the steps to be defined in the ApplicationSet. This leads to a lot of duplication because there are often common patterns for strategy, especially across a team's ApplicationSets. It would be a good QoL improvement if we could define the strategy in one place and refer to it in other ApplicationSets.

### Goals

- Possibility to define a sync strategy for multiple ApplicationSets

### Non-Goals


## Proposal

A CRD to define the strategy:
```
apiVersion: argoproj.io/v1alpha1
kind: SyncStrategy
metadata:
  name: region-strategy
spec:
  type: RollingSync
  steps:
    - matchExpressions:
      - key: region 
        operator: In
        values:
          - apac
    - matchExpressions:
      - key: region
        operator: In
        values:
          - emea
        maxUpdate: 10%
    - matchExpressions:
      - key: region
        operator: In
        values:
          - na-west
        maxUpdate: 100%
```

In the ApplicationSet, refer to the resource instead:
```
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
spec:
  generators:
  - list:
      elements:
      - cluster: cluster1
        region: apac
      - cluster: cluster2
        region: emea
      - cluster: cluster3 
        region: na-west
  strategyRef:
	name: region-strategy
    namespace: argocd
```

To match what Kubernetes is doing with resource such as the `Role` and `ClusterRole`, there should be both a `SyncStrategy` and a `ClusterSyncStrategy` resource. `ClusterSyncStrategy` being a cluster-wide strategy available to all ApplicationSet from any namespace and `SyncStrategy` being the namepspaced version.

### Use cases

#### Use case 1:
As a team managing multiple applicationsets, I would like be able to define a common strategy that can be reused in all my applicationsets. This would make it easier to manage how the applicationssets are synced as well as remove unnecessary duplication.

#### Use case 2:
As an organization or a group of team, I want to be able to define a default common sync strategy for all applicationsets for that group. This would reove the need to duplicate a strategy across multiple namespaces and would allow easier changes to the general strategy.

### Implementation Details/Notes/Constraints [optional]

What are the caveats to the implementation? What are some important details that didn't come across
above. Go in to as much detail as necessary here. This might be a good place to talk about core
concepts and how they relate.

You may have a work-in-progress Pull Request to demonstrate the functioning of the enhancement you are proposing.

### Detailed examples

Using a SyncStrategy in an ApplicationSet:
```
---
apiVersion: argoproj.io/v1alpha1
kind: SyncStrategy 
metadata:
  name: gradual-environments 
spec:
  type: RollingSync
  rollingSync:
    steps:
      - matchExpressions:
        - key: envLabel
          operator: In
          values:
            - env-dev
        #maxUpdate: 100%  # if undefined, all applications matched are updated together (default is 100%)
      - matchExpressions:
        - key: envLabel
          operator: In
          values:
            - env-qa
        maxUpdate: 0      # if 0, no matched applications will be updated
      - matchExpressions:
        - key: envLabel
          operator: In
          values:
            - env-prod
        maxUpdate: 10%    # maxUpdate supports both integer and percentage string values (rounds down, but floored at 1 Application for >0%)
---
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
spec:
  generators:
  - list:
      elements:
      - cluster: engineering-dev
        url: https://1.2.3.4
        env: env-dev
      - cluster: engineering-qa
        url: https://2.4.6.8
        env: env-qa
      - cluster: engineering-prod
        url: https://9.8.7.6/
        env: env-prod
  strategyRef:
    kind: SyncStrategy
    name: gradual-environments
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  template:
    metadata:
      name: '{{.cluster}}-guestbook'
      labels:
        envLabel: '{{.env}}'
    spec:
      project: my-project
      source:
        repoURL: https://github.com/infra-team/cluster-deployments.git
        targetRevision: HEAD
         path: guestbook/{{.cluster}}
      destination:
        server: '{{.url}}'
        namespace: guestbook
```

### Security Considerations

* How does this proposal impact the security aspects of Argo CD workloads ?

I'm not sure I see a possible security issue with this proposal other than if the RBAC for a ClusterSyncStrategy is not setup properly and is edit, it could impact the deployments of multiple applications.

* Are there any unresolved follow-ups that need to be done to make the enhancement more robust ?

### Risks and Mitigations

What are the risks of this proposal and how do we mitigate. Think broadly.

For example, consider
both security and how this will impact the larger Kubernetes ecosystem.

Consider including folks that also work outside your immediate sub-project.


### Upgrade / Downgrade Strategy

Apart from the new CRDs that will be installed, the upgrade will be transparent, ApplicationSets that define an inline strategy will still work and have priority. There will be no change of behavior, the only change will be the possibility to use the CRDs and the new `strategyRef` field

For a downgrade, if the new `strategyRef` field is used, it will stop working with the old CRDs so it will need to be removed. If it is not used, the downgrad will be transparent

## Drawbacks

- Adds new CRDs that are only used for the progressive sync feature which is a minority of ArgoCD users

## Alternatives

- Find another way to share strategies such as using ConfigMaps
