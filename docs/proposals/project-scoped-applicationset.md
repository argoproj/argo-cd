---
title: Support project scoped ApplicationSets
authors:
- "@jorturfer"

sponsors:
- TBD

reviewers:
- TBD

approvers:
- TBD

creation-date: 2023-05-17
last-updated: 2024-10-31
---

# Neat Enhancement Idea

This is a proposal to provide support for users to use ApplicationSets inside the scope of a single project.

## Summary

ArgoCD users will be able to use ApplicationSets in the context of the project where they have the required RBAC permissions. 
From an external usage point of view, every time an ApplicationSet is created/updated the controller will validate if the 
user is permitted to create an applicationset within the given project, as well as creating/updating/deleting underlying
applications within the same project. From the controller point of view, during application generation time, the 
controller will ensure that the generated applications are a part of the applicationset project.

## Motivation

We have transversal teams where they are the owners of the application in all stages (code, infra, ux, design,...) 
and our products are deployed in a cluster per product. Apart from this, critical products are deployed into multiple 
regions, which means that a product could be in 3-5 different Kubernetes clusters (including different cloud providers).
In this use case, ApplicationSets are a perfect match for us because they can deploy an appset for each microservice, 
and they are automatically deployed into all clusters with the required configuration (also enabling disaster recovery 
scenarios in other providers/regions thanks to the generators)

As a platform team, we provide the tooling like ArgoCD, Harbor, observability etc. to the teams. 
This means that we manage a shared ArgoCD installation for all teams, managing their clusters from the same place 
(using RBAC). We'd like to allow the usage of appsets for teams, but due to the global scope of them, teams can deploy 
things into other teams clusters because appsets ignores any kind of limitations.

### Goals

Add support for deploying ApplicationSets inside an `AppProject`. This also includes RBAC verification.

### Non-Goals

This doesn't include any changes in the UI/CLI for user-friendliness.

## Proposal

The proposal for achieving this goal is to add an optional `project` field into the ApplicationSet 
spec and based upon that to limit access to the underlying resources during application generation.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: cluster-addons
  namespace: argocd
spec:
  project: sample-project # new field
  generators:
    - #...
```

As we would like to maintain the option of deploying global ApplicationSets (without any restrictions), administrators will 
still be able to deploy appsets without needing to set the `project` field. In order to achieve this, we first check if 
the user has access to **ALL** `applicationsets` across **ALL** projects (*/*) and only then will they be able to deploy 
"project-less" `applicationsets` (otherwise setting a non-empty `project` field would be mandatory):

```
p, role:admin, applicationsets, create, */*, allow
p, role:admin, applicationsets, update, */*, allow
```

Another option is to be more explicit and to have a specific permission for this:

```
p, role:admin, applicationsets, unscoped, *, allow
```

> Note: `unscoped` is a proposal, but perhaps `global` or any other name is better.

Inside the appset controller, the controller will check if the `project` field has been set, which can produce 2 different behaviors:

1. `project` isn't set or is empty: In this case, nothing will change from the current behavior.
2. `project` is set: In this case, we validate that the generated applications are scoped to the project specified in 
the appset. In the case of cluster/git generators the clusters/repos _also_ need to be scoped to the same project 
as the applicationset.

### Use cases

#### Use case 1:

In my company, the platform team manages shared services like ArgoCD, and we have a shared instance for all projects. 
We cannot currently use ApplicationSets with this approach because `team A` could deploy things (voluntarily or not) into 
other teams' clusters. We could use Applications instead of ApplicationSets but in that case, we will lose the option 
of spinning up a production cluster ready to work in a few minutes (kubernetes cluster deployment time + app startup).

If this feature is available, we would deploy all workloads as project-scoped ApplicationSets because we have 
multi-region/multi-provider workloads. In this scenario, we could spin up a cluster wherever we want and by just 
adding a cluster into an AppProject, we could deploy all workloads in a few minutes, without having to depend upon any 
other external tool such as Azure Pipelines or GH Actions.

Thanks to this disaster recovery is much faster than using other options such as creating/restoring backups.

### Implementation Details/Notes/Constraints [optional]

This feature has to be implemented at all levels, on the ApplicationSet CRD, appset-controller, argocd-cli, argo-server,...

### Security Considerations

* How does this proposal impact the security aspects of Argo CD workloads?
  As this adds some security checks and limits appsets in some conditions, I think there is a positive impact.
* Are there any unresolved follow-ups that need to be done to make the enhancement more robust?
  No

### Risks and Mitigations

I don't think that this includes any extra risk

### Upgrade / Downgrade Strategy

We would be introducing a new field to the ApplicationSet CRD, however no existing fields are being changed. We believe 
this means that a new ApplicationSet version is unnecessary, and that upgrading to the new spec with extra fields would 
be a clean operation.

Downgrading would risk users receiving K8s API errors if they continue to try to apply the `project` field to a 
downgraded version of the ApplicationSet resource. Downgrading the controller while keeping the upgraded version of 
the CRD should cleanly downgrade/revert the behavior of the controller to the previous version without requiring users 
to adjust their existing ApplicationSet specs.
