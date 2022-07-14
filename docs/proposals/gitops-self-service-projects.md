---
title: GitOps Self-Service AppProjects
authors:
  - @crenshaw-dev
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2022-07-14
last-updated: yyyy-mm-dd
---

# GitOps Self-Service AppProjects

## Open Questions

## Summary

The ability to create/update AppProjects currently grants admin privileges by proxy. (If I can edit an AppProject, I can
add the Argo CD namespace as a destination and take control of the argocd-rbac-cm ConfigMap.)

So admins can't let developers create AppProjects. So the work of creating and auditing AppProjects falls to Argo CD
admins.

There should be a way for Argo CD admins to apply basic restrictions _once_ and then let developers create/update 
AppProjects that are within those bounds. Ideally, developers should be able to create properly-restricted AppProjects
via GitOps (by writing the manifest and putting them into a git repository.)

## Motivation

Because AppProject create/update privileges currently effectively grant admin, admins either have to

1) manually create each AppProject needed by developers or
2) manage an App-of-Projects pulling from a git repo where only Argo CD admins are allowed to merge changes.

Both of those options require a lot of Argo CD admin time/effort. And they are error-prone. In a large organization,
Argo CD admins have to either learn the very specific needs of developers, or they have to grant privileges without
really knowing if the privileges match the use case.

### Goals

* Allow developers to safely create AppProjects via GitOps within some admin-defined bounds

### Non-Goals

* Allowing developers to create AppProjects via the UI/CLI (though that should be possible with a follow-up proposal for new RBAC rules)

## Proposal

### Use cases

#### Use case 1: allow AppProjects that operate in a subset of namespaces

As an Argo CD admin, I would like to allow developers to create AppProjects which grant all privileges _except_ 
modifying resources in the `argocd` namespace. For example, I'd like to enforce that their AppProjects can allow any
destinations starting with `dev-`

#### Use case 2: allow AppProjects that can't manage cluster resources

As an Argo CD admin, I would like to allow developers to create AppProjects which let them do anything _except_ edit
cluster level resources.

### Implementation Details/Notes/Constraints 

We should add a new `parentProject` field to the AppProject CRD. When Argo CD evaluates whether an action is allowed by
AppProject restrictions, it should evaluate restrictions for every AppProject in the `parentProject` chain. If any 
restriction in the chain blocks the action, the action should fail.

We should also add a new `allowedParentProjects` field to the Application CRD. When an AppProject is added to (or
modified in) an Application, and when the `allowedParentProjects` field is set, the AppProject should only be allowed 
if it is in the `allowedParentProjects` field. (The items in the field could be either plain strings or patterns.)

(Note: the `allowedParentProjects` field could be added to the AppProject CRD to similar effect.)

The combination of these two features will allow Argo CD admins to grant developers the ability to create their own 
AppProjects. That process would look like this:

### Detailed examples

Allow self-service AppProjects without cluster resource access.

1. Create a new Application to manage the self-service AppProjects. This Application is managed only by Argo CD admins.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: self-service-projects
  namespace: argocd
spec:
  destination:
    namespace: argocd
    server: https://kubernetes.local.svc
  project: default
  source:
    path: .
    repoURL: https://company.github.com/developers/argocd-projects.git
  allowedParentProjects:
    - no-cluster-resources
```

2. Create a parent AppProject to restrict child (self-service) AppProjects.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: no-cluster-resources
  namespace: argocd
spec:
  sourceRepos:
    - '*'
  clusterResourceWhitelist: []
```

3. Instruct developers that they may add AppProjects to the argocd-projects repo as long as the AppProject have
   `parentProject: no-cluster-resources`.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: no-cluster-resources
  namespace: argocd
spec:
  parentProject: no-cluster-resources
  sourceRepos:
    - https://company.github.com/developers/my-app.git
```

The self-service-projects Application will fail to sync if a developer adds an AppProject which is not restricted by
the allowed `parentProject`.

### Security Considerations

### Risks and Mitigations

### Upgrade / Downgrade Strategy

## Drawbacks


## Alternatives

### AppProjectSet controller

Would enforce restrictions by hard-coding protected fields and templating user-configurable fields.

### Global project + AppProject deny list filter

By [adding jq filters to AppProject allow/deny lists](https://github.com/argoproj/argo-cd/issues/7636), we could write
rules to require an App-of-Projects contain only AppProjects which use a 
[global project](https://argo-cd.readthedocs.io/en/stable/user-guide/projects/#configuring-global-projects-v18).

jq filters would be nice because they're general-purpose. But this treads on OPA territory and doesn't have a nice,
native-feeling user experience.
