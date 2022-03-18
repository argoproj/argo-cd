---
title: Server-Side Apply

authors:
- "@leoluz"

sponsors:
- TBD

reviewers:
- TBD

approvers:
- TBD

creation-date: 2022-03-17

---

# Server-Side Apply support for ArgoCD

[Server-Side Apply (SSA)][1] allows calculating the final patch to update
resources in Kubernetes in the server instead of the client. This proposal
describes how ArgoCD can leverage SSA during syncs.

* Open Questions
    * [Q-1] How to handle conflicts?
    * [Q-2] Should we support multiple managers?
* Summary
* Motivation
    * Better interoperability with Admission Controllers 
    * Better resource conflict management
    * Better resource conflict management
* Goals
* Non-Goals
* Proposal
    * Use cases
        * [UC-1]: enable SSA at the controller level
        * [UC-2]: enable SSA at the Application level
        * [UC-3]: enable SSA at the resource level
    * Security Considerations
    * Risks and Mitigations
    * Upgrade / Downgrade
* Drawbacks

---

## Open Questions

### [Q-1] How to handle conflicts?
When SSA is enabled, the server may return field conflicts with other managers.
What ArgoCD controller should do in case of conflict? Just force the sync and
log warnings (like some other controllers do?)

### [Q-2] Should we support multiple managers?
Should Server-Side Apply support in ArgoCD be implemented allowing multiple
managers for the same controller? ([more details][10])

## Summary

ArgoCD can benefit from [Server-Side Apply][1] during syncs. A few
improvements to consider:

- More reliable dry-runs (as admission controller is executed) ([ISSUE-804][5])
- [Syncs always run][2] mutating webhooks (even without diff)
- [Fix big CRD][3] sync issues
- Better interoperability with different controllers

## Motivation

ArgoCD uses kubectl library while syncing resources in the cluster. Kubectl uses
by default a 3-way-merge logic between the live state (in k8s), desired state
(in git) and the previous state (`last-applied-configuration` annotation) to
calculate diffs and patch resources in the cluster. This logic is executed in
the client (ArgoCD) and once the patch is calculated it is then sent to the
server.

This strategy works well in the majority of the use cases. However, there are
some scenarios where calculating patches in the client side can cause problems.

Some of the known problems about using client-side approach:

### Better interoperability with Admission Controllers 

More and more users are adopting and configuring [Admission Controllers][4] in
Kubernetes with different flavors of Validating Webhooks and Mutating Webhooks.
Admission Controllers will only execute in server-side. In cases where users
rely on dry-run executions to decide if they should proceed with a deployment,
having the patch calculated at the client side might provide undesired results.
Having SSA enabled syncs also guaranties that Admission Controllers are always
executed, even when there is no diff in the resource.

### Better resource conflict management

Server-Side Apply will better handle and identify conflicts during syncs by
analyzing the `managedFields` metadata available in all Kubernetes resources
(since kubernetes 1.18). 

### Better resource conflict management

By not having to rely on the `last-applied-configuration` annotation, SSA would
help with failing syncs caused by exceeded annotation size limit when syncing
CRDs with large schemas.

## Goals

- Provide the ability for users to define if they want to use SSA during syncs
  ([ISSUE-2267][6])
  - Users should be able to enable SSA at the controller level (via binary flag)
  - Users should be able to enable SSA for a given Application (via syncOptions)
  - Users should be able to enable SSA at resource level (via annotation)
- Diffing needs to support strategic merge patch ([ISSUE-2268][7])
- Allow Admission Controllers to execute even when there is no diff for a
  particular resource. (Needs investigation)
- ArgoCD should respect field ownership and provide a configuration to allow
  users to define the behavior in case of conflicts
- ArgoCD should register itself with a proper manager.

## Non-Goals

What is out of scope for this proposal?
Listing non-goals helps to focus discussion and make progress

## Proposal

Change ArgoCD controller to accept new parameter to enable Server-Side Apply
during syncs. ArgoCD must register itself with a pre-defined manager
(suggestion: `argocd-controller`).

### Use cases

The following use cases should be implemented:

#### [UC-1]: As a user, I would like enable SSA at the controller level so all Application are applied server-side

#### [UC-2]: As a user, I would like enable SSA at the Application level so all resources are applied server-side

Implement a new syncOption to allow users to enable SSA at the application
level (Suggestion `ServerSideApply=true`). UI needs to be updated to support
this new sync option.

#### [UC-3]: As a user, I would like enable SSA at the resource level so only a single manifest is applied server-side

### Security Considerations
TBD

### Risks and Mitigations
ArgoCD must check if the target Kubernetes cluster has full support for SSA. The
feature turned [GA in Kubernetes 1.22][8]. Full support for managed fields was
introduced as [beta in Kubernetes 1.18][9]. The implementation must check that
the target kubernetes cluster is running at least version 1.18. If SSA is
enabled and target cluster version < 1.18 ArgoCD should log warning and fallback
to client sync.

### Upgrade / Downgrade
No CRD update necessary as `syncOption` field in Application resource is non-typed
(string array). Upgrade will only require ArgoCD controller update.

## Drawbacks
Slight increase in ArgoCD code base complexity.

[1]: https://kubernetes.io/docs/reference/using-api/server-side-apply/
[2]: https://github.com/argoproj/argo-cd/issues/2267#issuecomment-920445236
[3]: https://github.com/prometheus-community/helm-charts/issues/1500#issuecomment-1017961377
[4]: https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/
[5]: https://github.com/argoproj/argo-cd/issues/804
[6]: https://github.com/argoproj/argo-cd/issues/2267
[7]: https://github.com/argoproj/argo-cd/issues/2268
[8]: https://kubernetes.io/blog/2021/08/06/server-side-apply-ga/
[9]: https://kubernetes.io/blog/2020/04/01/kubernetes-1.18-feature-server-side-apply-beta-2/
[10]: https://github.com/argoproj/gitops-engine/pull/363#issuecomment-1013641708
