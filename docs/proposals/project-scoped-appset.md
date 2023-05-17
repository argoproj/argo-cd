---
title: Support project scoped ApplicationSet
authors:
- "@jorturfer"

sponsors:
- TBD

reviewers:
- TBD

approvers:
- TBD

creation-date: 2023-05-17
last-updated: 2023-05-17
---

# Neat Enhancement Idea

This is a proposal to provide support for users to use ApplicationSets inside the scope of a single project.


## Open Questions [optional]

- Should we include global resources (resources that aren't scoped to any project) into the available resources for project scoped appset?

## Summary

ArgoCD users will be able to use ApplicationSets in the context of the projects where they have access by RBAC. From external usage point of view, each time when an ApplicationSet is created/updated, the controller will validate if the user has permission for doing it. From controller point of view, during the Applications generation, the controller will ensure that the used resources are part of the project 

## Motivation

We have transversal teams where they are the owners of the application in all the stages (code, infra, ux, design,...) and our products are deployed in a cluster per product. Apart from this, critical products are deployed into multiple regions, which means that a product could be in 3-5 different Kubernetes clusters (including different cloud providers). 
In this use case, ApplicationSet is the perfect match for us because they can deploy an appset for each microservice and automatically they are deployed into all the clusters with the required configuration (also enabling disaster recovery scenarios in other providers/regions thanks to the generators)

As a platform team, we provide to the teams the tooling like ArgoCD, Harbor, observability,... This means that we manage an ArgoCD installation shared for all the teams, managing their clusters from the same place (based on RBAC). We'd like to allow the usage of appsets for the teams, but due to the global scope of them, teams can deploy things into other teams clusters because appsets ignores any kind of limitation.

### Goals

Add support for deploying ApplicationSets inside a AppProject and limit the interaction with elements inside it. 
This includes also the RBAC verification

### Non-Goals

This doesn't include any change in the UI/cli to make them user friendly

## Proposal

The proposal for achieving this goal is to add a "required" `project` field (read more below) into the ApplicationSet and based on it, limit the access to the resources during the Applications generation. 

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: cluster-addons
  namespace: argocd
spec:
  project: sample-project # new field
  generators:
  - ...
```

As we want to maintain the option of deploying global ApplicationSet (without any restriction), administrators will be able to deploy appsets without setting the project field. For achieving this, we will check if the user has access to **ALL** `applicationsets` across **ALL** the projects (*/*) and only that case, they will be able to deploy `applicationsets` without setting the project:

```
p, role:admin, applicationsets, create, */*, allow
p, role:admin, applicationsets, update, */*, allow
```

Other option is to be explicit and create an specific permission for this:

```
p, role:admin, applicationsets, unscoped, *, allow
```

> Note: `unscoped` is a proposal, but maybe `global` or any other name is better.

Inside the appset controller, the controller will check if appset has set the `project` field and it can produce 2 different behaviors:

1. `project` isn't set: In this case, nothing will change from current behavior
2. `project` is set: In this case, **ALL** the resources involved in the Application generation have to be scoped to the project where the appset is deployed. 
In case of cluster/git generators the cluster/repos have to be project scoped, but in other generators, the controller will check if the resource is allowed for the project (for example, using a list generator the controller will block forbidden clusters/repos)

### Use cases

#### Use case 1:
In my company, the platform team manages shared services like ArgoCD and we have a shared instance for all the projects. Currently we cannot use ArgoCD with this approach because the team A could deploy things (voluntarily or not) into other teams clusters. We could use Applications instead of ApplicationSet but in that case, we will lose the option of spinning up a production cluster ready to work in a few minutes (kubernetes cluster deployment time + apps startup).

If this feature is available, we will deploy all the workloads as ApplicationSet scoped to projects, because we have multi-region/multi-provider workloads and we will use generators. In this scenario, we could spin up a cluster wherever we want and just adding the cluster into an AppProject, we could deploy all the workloads in a few minutes, not depending on any other external tool (like Azure Pipelines ort GH Actions).

Thanks to this, the disaster recovery plan is quite faster than using other options like creating/restoring backups.

### Implementation Details/Notes/Constraints [optional]

This feature has to be implemented at all the levels, CRD, appset controller, argocd-cli, argo server,...

### Security Considerations

* How does this proposal impact the security aspects of Argo CD workloads ?
As this adds some security checks and limits the appset in some conditions, I think that we impact positively
* Are there any unresolved follow-ups that need to be done to make the enhancement more robust ?
I don't think so, but I don't have experience developing ArgoCD

### Risks and Mitigations

I don't think that this includes any extra risk

### Upgrade / Downgrade Strategy

We are introducing new fields to the ApplicationSet CRD, however no existing fields are being changed. We believe this means that a new ApplicationSet version is unnecessary, and that upgrading to the new spec with extra fields will be a clean operation.

Downgrading would risk users receiving K8s API errors if they continue to try to apply the `project` field to a downgraded version of the ApplicationSet resource.
Downgrading the controller while keeping the upgraded version of the CRD should cleanly downgrade/revert the behavior of the controller to the previous version without requiring users to adjust their existing ApplicationSet specs.
