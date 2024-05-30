---
title: Neat-enhancement-idea
authors:
- "@alexmt"
  sponsors:
- TBD
  reviewers:
- "@jessesuen"
- TBD
  approvers:
- "@jessesuen"
- TBD

creation-date: 2020-04-19
last-updated: 2020-04-19
---

# Neat Enhancement Idea

Support project scoped Repositories and Clusters to enable self-service end-users onboarding.

## Summary

The Argo CD has two type of users:

* Administrators who configure the Argo CD and manage the Argo CD projects.
* Developers who use Argo CD to manage resources in the Kubernetes clusters.

These two roles enable sharing on the Argo CD instance in a multi-tenant environment. Typically the developer
requests a new project from an administrator. The administrator creates the project, defines which repositories
can and clusters can be used within the project which concludes the onboarding.

The problem is that list of repositories and clusters are often not known during the onboarding process. Developers get
it later and have to again contact an administrator, somehow share repo/cluster credentials. This back and forth
process takes time and creates friction.

We want to streamline the process of adding repositories and clusters to the project and make it self-service. The Argo CD
admins should be able to optionally enable self onboarding of repositories/clusters for some projects.

## Motivation

As long as the developer has the required credentials he/she should be able to add repository/cluster to the project
without involving the administrator. To archive it, we are proposing to introduce project scoped repositories and clusters.

### Goals

The goals of project scoped repositories and clusters are:

#### Allow Self-Registering Repositories/Clusters in a Project

Developer should be able to add a repository/cluster into the project without asking help from Argo CD administrator.

### Non-Goals

#### Simplify Management of Shared Repositories/Clusters in a Project

The repositories and clusters that can be used across multiple projects still have to be managed by Argo CD administrator.

## Proposal

#### Project scoped repository/cluster

The proposal is to introduce project scoped clusters and repositories that can be managed by a developer who has access to the project.
The only difference of project scoped repository/cluster is that it has `project` field with the project name it belongs to. Both repositories
and clusters are stored as Kubernetes Secrets, so a new field could be stored as a Secret data key:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: argocd-example-apps
  labels:
    argocd.argoproj.io/secret-type: repository
type: Opaque
stringData:
  project: my-project1                                     # new project field
  name: argocd-example-apps
  url: https://github.com/argoproj/argocd-example-apps.git
  username: ****
  password: ****
```

* The project scoped repository/cluster is automatically allowed in the project.
  This enables developers to allow new cluster/repository without modifying the project.
* The project scoped repository/cluster still can be used in other project but it has to be allowed by admin (as normal repository/cluster).
* If another team wants to add the same repository/cluster into a different project they would have to ask admin. 

#### Project RBAC Changes

The organization still might want to enforce certain rules so developers won't get permission to add a
project-specific repository/cluster by default. The administrator might use RBAC to control access to
the project scoped repositories cluster. The access to project scope actions will be checked using
`<projectName>/<name>` pattern. For example, to allow users to add project scoped repositories admin would have to add
the following RBAC rules:

```
p, proj:my-project:admin, repositories, create, my-project/*, allow
p, proj:my-project:admin, repositories, delete, my-project/*, allow
p, proj:my-project:admin, repositories, update, my-project/*, allow
```

This provides extra flexibility so that admin can have stricter rules. e.g.:

```
p, proj:my-project:admin, repositories, update, my-project/"https://github.example.com/*", allow
```

#### UI/CLI Changes

Both User interface and CLI should get ability to optionally specify project. If project is specified than cluster/repository
is considered project scoped:

```
argocd repo add --name stable https://charts.helm.sh/stable --type helm --project my-project
```


### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

## Use case 1:
As a developer, I would like to register credentials of a Git repository I own so I can deploy manifests stored in that repository.

## Use case 2:
As a developer, I would like to register credentials of a Kubernetes cluster so I can manage resources in that cluster.

### Implementation Details/Notes/Constraints [optional]

As of v2.0.1 Argo CD stores Repository non-sensitive metadata in `argocd-cm` ConfigMap. This is going to change in https://github.com/argoproj/argo-cd/issues/5436.
So we would have to wait for #5436 implementation.

### Detailed examples

### Security Considerations

The security considerations are explained in `Project RBAC Changes` section.

### Risks and Mitigations

#### Deverlopers Might Overload Argo CD

The developers are typically not responsible for Argo CD health and don't have access to Argo CD metrics. So adding too many clusters might overload Argo CD.
Two improvements are proposed to mitigate that risk:

**Improved Cluster Metrics**

The existing metrics should be improved so that administrators could quickly discover if the project "has" too many clusters and easily discover who added the cluster:

* Add `project` tag to existing cluster metrics: [clustercollector.go](https://github.com/argoproj/argo-cd/blob/bfd0b155eff4212e9354a6958e329dbd64f9a69a/controller/metrics/clustercollector.go#L20).
* Document how administrator can leverage metrics to configure limits per project and get notifications when the limit is exceeded.
* Add `owner` field to the cluster (and repository for consistency ) and use it to store username of the user who added cluster/repository. The administrator can use the `owner` field to contact
  the person who added the cluster and exceeded the limit.

**Project Sharding**

It should be possible to automatically assign project scoped clusters to the specific clusters shard. This way admin can isolate large projects from each other and limit the blast radius.

### Upgrade / Downgrade Strategy

In case of rollback to the previous version, the project scoped clusters/repositories will be treated as normal (non-scoped) clusters/repositories.
So it is safe to rollback and then roll forward.

## Open Issues

If the same cluster or repository required in multiple projects that there is no way to configure it without involving Argo CD admin. The end-user
would still have to reach out to the administrator and request Argo CD config changes.

## Alternatives

Don't introduce first-class support for this feature and instead create optional CRD that manages clusters and repositories.
In this case, the first-class support seems like a very natural fit into the existing design.
