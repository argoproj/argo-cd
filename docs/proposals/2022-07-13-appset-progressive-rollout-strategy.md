---
title: ApplicationSet Progressive Rollout Strategy
authors:
  - "@wmgroot"
  - "@cnmcavoy"
sponsors:
  - indeed.com
reviewers:
  - "@alexmt"
  - TBD
approvers:
  - "@alexmt"
  - TBD

creation-date: 2022-07-13
last-updated: 2022-08-11
---

# ApplicationSet Progressive Rollout Strategy

## Summary

Enhance the ArgoCD ApplicationSet resource to embed a rollout strategy for a progressive application resource update after the ApplicationSet spec or Application templates are modified.
Further discussion and interest has been communicated here: https://github.com/argoproj/argo-cd/issues/9437

## Motivation

As cluster operators, we would like to make changes to ApplicationSets which may target multiple environments, pre-defined staging areas, or other configurations, and have these changes rolled out in a declarative, defined manner rather than all at once as ApplicationSets currently behave. A progressive ApplicationSet rollout would prevent mistakes in configuration from having a larger blast radius than intended and give cluster operators a chance to verify and have confidence in their changes.

### Goals

Users are able to make a single change to ApplicationSet that is updated across the generated Applications in a controlled manner. When this enhancement is enabled, Applications are updated in a declaractive order, instead of simultaneously.

### Non-Goals

Handling controlled rollouts for changes to a helm chart or raw manifests referenced by the Applications managed by the ApplicationSet. We understand this would be valuable, but we would like to implement the rollout implementation handling only changes to the ApplicationSet initially.

## Proposal

This is where we get down to details of what the proposal is about.

### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

#### Use case 1:
As a user, I would like to declaratively control the rollout order of ApplicationSet changes to its generated Application resources.

We propose adding a `RollingUpdate` and `RollingSync` strategy spec (taking inspiration from other controllers).

The rolling update strategy deterministically chooses applications to update following a maxUpdate value. If maxUpdate is set to 1, then applications are updated one by one, proceeding each step only if the previous application syncs completed successfully. If set to more than 1, then applications are updated in parallel up to that number.
Steps for the rolling update are defined by a list of matchExpression label selectors. Each step must finish updating before the next step advances. If steps are left undefined the application update order is deterministic.

Complete ApplicationSet spec example.
```
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
        env: dev
      - cluster: engineering-prod
        url: https://2.4.6.8
        env: prod
      - cluster: engineering-qa
        url: https://9.8.7.6/
        env: qa
  strategy:
    type: RollingUpdate
    rollingUpdate:
      steps:
        - matchExpressions:
            - key: env
              operator: In
              values:
                - dev
          maxUpdate: 0 # if undefined or 0, all applications matched are updated together
        - matchExpressions:
            - key: env
              operator: In
              values:
                - qa
        - matchExpressions:
            - key: env
              operator: In
              values:
                - us-east-2
                - eu-west-1
                - ap-southeast-1
          maxUpdate: 1 # maxUpdate supports both integer and percentage string values
  template:
    metadata:
      name: '{{cluster}}-guestbook'
      labels:
        env: "{{env}}"                                # label can be provided explicitly from a list generator
        region: "{{metadata.labels.cluster/region}}"  # or pulled from labels on the argo cluster secrets
    spec:
      source:
        repoURL: https://github.com/infra-team/cluster-deployments.git
        targetRevision: HEAD
        path: guestbook/{{cluster}}
      destination:
        server: '{{url}}'
        namespace: guestbook
```

In the above example, when the guestbook ApplicationSet is created or modified, the Application resources are each updated in the order defined in `strategy.rollingUpdate`. In this case, all generated Applications (applied or not) with a label that matches the expression `env: dev` are updated to match the template. All Applications in this step are updated in parallel, because the `maxUpdate` is set to zero. The rolling update strategy progresses after the first set of Applications has successfully progressed and become healthy again. Progress towards the next step starts only after the current step has completely finished, regardless of the `maxUpdate` value. The `maxUpdate` field only throttles the total number of matching Applications updating in the current step. After the first step completes, the ApplicationSet updates all Application resources with label `env: qa` at the same time, because `maxUpdate` is undefined. Finally, during the third step, the Application resources labeled `region: us-east-2`, `region: eu-west-1`, or `region: ap-southeast-1` are updated, one by one, as the `maxUpdate` for the final step is 1.

An Application rollout is considered “complete” when the Application resource has been:
- Synced successfully.
- Moved into a “Progressing” state.
- Moved out of a “Progressing” state and into a “Healthy” state.

`RollingSync` operates using the same spec, but is a re-implementation of the https://github.com/Skyscanner/applicationset-progressive-sync tool. It notices that Applications become OutOfSync, and triggers sync operations on those Applications following the order declared in the Application strategy spec.


#### Use case 2:
As a user, I would like to continue to use the current simultaneous Application update behavior of the ApplicationSet controller.

If no strategy is provided, we propose defaulting to an `AllAtOnce` strategy, which maintains the current behavior.


### Implementation Details/Notes/Constraints [optional]

#### Initial ApplicationSet Creation
Application resource creation from an ApplicationSet with a defined strategy looks much like the update process. When a brand new ApplicationSet is first created with a rollout strategy specified, the desired Application resource metadata labels are used to determine when each Application resource is created. Each Application created will be created in the order defined by the steps, if any, and advance to the next step only when a step completes successfully. The same applies if an ApplicationSet is modified to target a different set of destination clusters or namespaces, Applications are created or updated in the order defined by their desired state and the defined step order in the strategy.

#### ApplicationSet Rollout Failure
In the event that an ApplicationSet spec or template is modified and a target Application resource fails to “complete” a sync in any of the steps, the ApplicationSet rollout is stalled. The ApplicationSet resource will ensure the status field for “ApplicationSetUpToDate” is False. If the maxUpdate allows it, the ApplicationSet will continue updating Applications in the current step, but otherwise, no further changes will be propagated to Application resources by the ApplicationSet, and no steps will advance until each Application can successfully complete a sync. If the ApplicationSet is modified while still in the midst of an ApplicationSet rollout, stalled or otherwise, then the existing rollout is abandoned, the application resources are left in their present state, and the new rollout begins.

#### "Pausing" Application Changes During Rollout
To implement the “paused” functionality of Applications that are not yet ready to be updated, we have a few options.
* Disable auto-sync.
** Potentially conflicts with user provided auto-sync settings.
** Provides the benefit of being able to see the full diff of the ApplicationSet change.
* “Pause” the Application.
** Not Yet Implemented: https://github.com/argoproj/argo-cd/issues/4808
* Prevent any updates at all to the live Applications via the rolling update strategy defined.
** This is likely the initial implementation method we'll target.

#### Draft Pull Request
This PR is now functional and ready for comment. We are actively working on unit tests and documentation.
https://github.com/wmgroot/argo-cd/pull/1

### Security Considerations
We do not believe this proposal results in any new security considerations for the ApplicationSet controller.

### Risks and Mitigations

If this proposal is implemented, I believe the next logical step would be to solve the case where users would like to control rollout order for Application resources with a consistent specification, but changes being pushed to the upstream `source` of the Application. A common use case is an update to an unversioned "wrapper" helm chart that depends on a versioned upstream chart. The wrapper chart is often used to apply simple supplementary resources in a gitops pattern, such as company specific RBAC configuration, or ExternalSecrets configuration. These supplementary resources do not typically warrant publishing a versioned wrapper chart, making it difficult to implement changes to the chart's templates or value files and roll them out in an ordered way with the ApplicationSet changes discussed here.

Implementing progressive rollout stragies to handled changes upstream of the generated Application source could be difficult, since the applicationset controller would need to intercept the sync operation of the Application to prevent the changes from syncing automatically.

Added maintenance burden on the ArgoCD team is always a risk with the addition of new features.

### Upgrade / Downgrade Strategy

We are introducing new fields to the ApplicationSet CRD, however no existing fields are being changed. We believe this means that a new ApplicationSet version is unnecessary, and that upgrading to the new spec with extra fields will be a clean operation.

Downgrading would risk users receiving K8s API errors if they continue to try to apply the `strategy` field to a downgraded version of the ApplicationSet resource.
Downgrading the controller while keeping the upgraded version of the CRD should cleanly downgrade/revert the behavior of the controller to the previous version without requiring users to adjust their existing ApplicationSet specs.

## Drawbacks

The idea is to find the best form of an argument why this enhancement should _not_ be implemented.

## Alternatives

One alternative we considered was to create an extra CRD specifically to govern the rollout process for an ApplicationSet. We ultimately decided against this approach because all other rollout strategy specs we looked at were implemented in the same CRD resource (K8s Deployments, Argo Rollouts, CAPI MachineDeployments, etc).

Another alternative is to implement Application Dependencies through the application-controller instead. This is a far more complicated approach that requires implementing and maintaining an Application DAG.
https://github.com/argoproj/argo-cd/issues/7437
