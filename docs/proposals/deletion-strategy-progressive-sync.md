---
title: Neat-enhancement-idea
authors:
  - "@sbose78" # Authors' github accounts here.
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

# Deletion Strategy for Progressive Sync

This proposal is building upon the ideas presented in https://github.com/argoproj/argo-cd/pull/14892 to introduce 
deletion strategy for progressive sync. While the original proposal laid the groundwork, this proposal extends to address
some unanswered sections and changes implementation details.

Introduce a new functionality of ArgoCD ProgressiveSync that will allow users to configure order 
of deletion for applicationSet's deployed applications. The deletion strategies can be:

- parallel (current strategy - can be the default value)
- reverse ( delete applications in the reverse order of deployment, configured in progressiveSync)

## Open Questions [optional]

The original proposal mentions another strategy - custom wherein the user can provide a specific order of deletion. 
Is such a usecase needed?


## Summary

This feature can extend the application dependency from deployment to deletion as well. Ability to provide deletion order 
can complete the ProgressiveSync feature.

## Motivation

Current deletion/removal strategy which ArgoCD use works fine if there aren't any dependencies between the different applications. 
However, it does not work when there are dependencies between the applications. This was noticed when some kubernetes core services 
were deployed in specific order and to be removed in reverse order.

### Goals

Following goals should be achieved in order to conclude this proposal:
1. Deletion strategy `parallel` as default value - deletes all applications at once as the current behavior
2. Deletion strategy `reverse` lets applications be deleted in the reverse order of the steps configured in RollingSync strategy.

### Non-Goals

custom deletion strategy

## Proposal

Ability to provide configuration related to the deletion/removal process when progressive sync is used. Implementation detail provides 
two options of introducing this field in ApplicationSet. The following use cases assumes Option 1 for the yaml file examples.

### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

#### parallel deletionStrategy:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: pricelist
  namespace: argocd
spec:
  generators:
  - list:
      elements:
      - srv: config
        path: applicationsets/rollingsync/apps/pricelist-config
      - srv: db
        path: applicationsets/rollingsync/apps/pricelist-db
      - srv: frontend
        path: applicationsets/rollingsync/apps/pricelist-frontend
  strategy:
    type: RollingSync
    rollingSync:
      steps:
        - matchExpressions:
            - key: pricelist-component # the "key" is based on the label (below as "pricelist-component: {{srv}}")
              operator: In
              values:
                - config
          #maxUpdate: 100%  # if undefined, all applications matched are updated together (default is 100%)
        - matchExpressions:
            - key: pricelist-component
              operator: In
              values:
                - db
          #maxUpdate: 0      # if 0, no matched applications will be updated
        - matchExpressions:
            - key: pricelist-component
              operator: In
              values:
                - frontend
          #maxUpdate: 10%    # all application matched are rollout no more than the percentage indicates
  ### Deletion configuration ###
  deletionSyncStrategy: 
    type: "parallel"  # available options to be parallel/reverse (maybe custom as well)
  ### Deletion configuration ###
  template:
    metadata:
      name: 'pricelist-{{srv}}'
      labels:
        pricelist-component: '{{srv}}'
    spec:
      project: default
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
        retry:
          limit: 5
          backoff:
            duration: 5s
            maxDuration: 3m0s
            factor: 2
      source:
        repoURL: https://github.com/christianh814/gitops-examples
        targetRevision: main
        path: '{{path}}'
      destination:
        server: https://kubernetes.default.svc
        namespace: pricelist
```

#### reverse deletionStrategy:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: pricelist
  namespace: argocd
spec:
  generators:
  - list:
      elements:
      - srv: config
        path: applicationsets/rollingsync/apps/pricelist-config
      - srv: db
        path: applicationsets/rollingsync/apps/pricelist-db
      - srv: frontend
        path: applicationsets/rollingsync/apps/pricelist-frontend
  strategy:
    rollingSync:
      steps:
        - matchExpressions:
            - key: pricelist-component # the "key" is based on the label (below as "pricelist-component: {{srv}}")
              operator: In
              values:
                - config
          #maxUpdate: 100%  # if undefined, all applications matched are updated together (default is 100%)
        - matchExpressions:
            - key: pricelist-component
              operator: In
              values:
                - db
          #maxUpdate: 0      # if 0, no matched applications will be updated
        - matchExpressions:
            - key: pricelist-component
              operator: In
              values:
                - frontend
          #maxUpdate: 10%    # all application matched are rollout no more than the percentage indicates
  ### Deletion configuration ###
  deletionSyncStrategy:
    type: "reverse"  # available options to be parallel/reverse (maybe custom as well)
  ### Deletion configuration ###        
  template:
    metadata:
      name: 'pricelist-{{srv}}'
      labels:
        pricelist-component: '{{srv}}'
    spec:
      project: default
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
        retry:
          limit: 5
          backoff:
            duration: 5s
            maxDuration: 3m0s
            factor: 2
      source:
        repoURL: https://github.com/christianh814/gitops-examples
        targetRevision: main
        path: '{{path}}'
      destination:
        server: https://kubernetes.default.svc
        namespace: pricelist  
```
#### If custom deletionStrategy:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: pricelist
  namespace: argocd
spec:
  generators:
  - list:
      elements:
      - srv: config
        path: applicationsets/rollingsync/apps/pricelist-config
      - srv: db
        path: applicationsets/rollingsync/apps/pricelist-db
      - srv: frontend
        path: applicationsets/rollingsync/apps/pricelist-frontend
  strategy:
    type: RollingSync
    rollingSync:
      steps:
        - matchExpressions:
            - key: pricelist-component # the "key" is based on the label (below as "pricelist-component: {{srv}}")
              operator: In
              values:
                - config
          #maxUpdate: 100%  # if undefined, all applications matched are updated together (default is 100%)
        - matchExpressions:
            - key: pricelist-component
              operator: In
              values:
                - db
          #maxUpdate: 0      # if 0, no matched applications will be updated
        - matchExpressions:
            - key: pricelist-component
              operator: In
              values:
                - frontend
          #maxUpdate: 10%    # all application matched are rollout no more than the percentage indicates
  ### Deletion configuration ###
  deletionSyncStrategy: 
    type: "custom"  # available options to be default/reverse/custom
    rollingSync:
      steps:
        - matchExpressions:
            - key: pricelist-component # the "key" is based on the label (below as "pricelist-component: {{srv}}")
              operator: In
              values:
                - config
          #maxUpdate: 100%  # if undefined, all applications matched are updated together (default is 100%)
        - matchExpressions:
            - key: pricelist-component
              operator: In
              values:
                - frontend
          #maxUpdate: 10%    # all application matched are rollout no more than the percentage indicates
        - matchExpressions:
            - key: pricelist-component
              operator: In
              values:
                - db
          #maxUpdate: 0      # if 0, no matched applications will be updated
  ### Deletion configuration ###
  template:
    metadata:
      name: 'pricelist-{{srv}}'
      labels:
        pricelist-component: '{{srv}}'
    spec:
      project: default
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
        retry:
          limit: 5
          backoff:
            duration: 5s
            maxDuration: 3m0s
            factor: 2
      source:
        repoURL: https://github.com/christianh814/gitops-examples
        targetRevision: main
        path: '{{path}}'
      destination:
        server: https://kubernetes.default.svc
        namespace: pricelist
```

### Implementation Details/Notes/Constraints [optional]

There should be a check that correlates the deletionStrategy to ApplicationSet strategy. For example can only select reverse 
if rollingSync lists out an order of application deployment, otherwise should error out.

Option 1: A draft Pull Request https://github.com/argoproj/argo-cd/pull/22842 adding deletionStrategy in ApplicationSetSpec. 

Option 2: An alternative could be to introduce in ApplicationSetStrategy as follows:
```yaml
type ApplicationSetStrategy struct {
	Type        string                         `json:"type,omitempty" protobuf:"bytes,1,opt,name=type"`
	RollingSync *ApplicationSetRolloutStrategy `json:"rollingSync,omitempty" protobuf:"bytes,2,opt,name=rollingSync"`
	// RollingUpdate *ApplicationSetRolloutStrategy `json:"rollingUpdate,omitempty" protobuf:"bytes,3,opt,name=rollingUpdate"`
	// Add DeletionSync Strategy here
	DeletionSyncType string `json:"deletionSyncStrategy,omitempty" protobuf:"bytes,4,opt,name=deletionSyncStrategy"` // takes value parallel/reverse
}

```

### Detailed examples


### Security Considerations

Since no additional roles or privileges are needed to be able to delete deployed applications in a specific order, 
so no impact on the security aspects of Argo CD workloads.


### Risks and Mitigations

No immediate Risks to consider


### Upgrade / Downgrade Strategy
Introducing new fields to the ApplicationSet CRD, however, no existing fields are being changed. 
This means that a new ApplicationSet version is unnecessary, and upgrading to the new spec with added fields will be a clean operation.

Downgrading could risk users receiving K8s API errors if they continue to try to apply the deletionStrategy field to a downgraded version of the ApplicationSet resource. 
Downgrading the controller while keeping the upgraded version of the CRD should cleanly downgrade/revert the behavior of the controller to the previous version without requiring users to adjust their existing ApplicationSet specs.


## Drawbacks
Slight increase in Argo CD code base complexity

## Alternatives
TBD