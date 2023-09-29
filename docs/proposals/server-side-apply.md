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

* [Open Questions](#open-questions)
    * [[Q-1] How to handle conflicts?](#q-1-how-to-handle-conflicts)
    * [[Q-2] Should we support multiple managers?](#q-2-should-we-support-multiple-managers)
* [Summary](#summary)
* [Motivation](#motivation)
    * [Better interoperability with Admission Controllers](#better-interoperability-with-admission-controllers)
    * [Better resource conflict management](#better-resource-conflict-management)
    * [Better CRD support](#better-crd-support)
* [Goals](#goals)
    * [[G-1] Fine grained configuration](#g-1-fine-grained-configuration)
    * [[G-2] Strategic merge patch while diffing](#g-2-strategic-merge-patch-while-diffing)
    * [[G-3] Admission Controllers compatibility](#g-3-admission-controllers-compatibility)
    * [[G-4] Conflict management](#g-4-conflict-management)
    * [[G-5] Register a proper manager](#g-5-register-a-proper-manager)
* [Non-Goals](#non-goals)
* [Proposal](#proposal)
    * [Non-Functional Requirements](#non-functional-requirements)
    * [Use cases](#use-cases)
        * [[UC-1]: enable SSA at the controller level](#uc-1-as-a-user-i-would-like-enable-ssa-at-the-controller-level-so-all-application-are-applied-server-side)
        * [[UC-2]: enable SSA at the Application level](#uc-2-as-a-user-i-would-like-enable-ssa-at-the-application-level-so-all-resources-are-applied-server-side)
        * [[UC-3]: enable SSA at the resource level](#uc-3-as-a-user-i-would-like-enable-ssa-at-the-resource-level-so-only-a-single-manifest-is-applied-server-side)
    * [Security Considerations](#security-considerations)
    * [Risks and Mitigations](#risks-and-mitigations)
        * [[R-1] Supported K8s version check](#r-1-supported-k8s-version-check)
        * [[R-2] Alternating Server-Side Client-Side syncs](#r-2-alternating-server-side-client-side-syncs)
    * [Upgrade / Downgrade](#upgrade--downgrade)
* [Drawbacks](#drawbacks)

---

## Open Questions

### [Q-1] How to handle conflicts?
When SSA is enabled, the server may return field conflicts with other managers.
What ArgoCD controller should do in case of conflict? Just force the sync and
log warnings (like some other controllers do?)

#### Conclusion
The first version should use the force flag and override even if there are
conflicts. We could improve and add other options once there is a use case.

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

Kubernetes SSA Proposal ([KEP-555][13]) has more details about how it works.

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

### Better CRD support

By not having to rely on the `last-applied-configuration` annotation, SSA would
help with failing syncs caused by exceeded annotation size limit. This is a
common issue when syncing CRDs with large schemas.

## Goals

All following goals should be achieve in order to conclude this proposal:

#### [G-1] Fine grained configuration

- Provide the ability for users to define if they want to use SSA during syncs
- Users should be able to enable SSA at the controller level (via binary flag)
(see [UC-1](#uc-1-as-a-user-i-would-like-enable-ssa-at-the-controller-level-so-all-application-are-applied-server-side))
- Users should be able to enable SSA for a given Application (via syncOptions)
(see [UC-2](#uc-2-as-a-user-i-would-like-enable-ssa-at-the-application-level-so-all-resources-are-applied-server-side))
- Users should be able to enable SSA at resource level (via annotation) (see
[UC-3](#uc-3-as-a-user-i-would-like-enable-ssa-at-the-resource-level-so-only-a-single-manifest-is-applied-server-side)
- Relates to [ISSUE-2267][6]

#### [G-2] Strategic merge patch while diffing

- Diffing needs to support strategic merge patch (see [ISSUE-2268][7])
- Make sure Services can be patched correctly ([more details][14])

#### [G-3] Admission Controllers compatibility

- Allow Admission Controllers to execute even when there is no diff for a
  particular resource. (Needs investigation) ([more details][2])

#### [G-4] Conflict management

- ArgoCD should respect field ownership and provide a configuration to allow
  users to define the behavior in case of conflicts (see
  [Q-1](#q-1-how-to-handle-conflicts) outcome)

#### [G-5] Register a proper manager

- ArgoCD must register itself with a pre-defined manager (suggestion:
  `argocd-controller`). It shouldn't rely on the default value defined in the
  kubectl code. ([more details][11])

## Non-Goals

TBD

## Proposal

Change ArgoCD controller to accept new parameter to enable Server-Side Apply
during syncs. Changes are necessary in ArgoCD as well as in
gitops-engine library.

### Use cases

The following use cases should be implemented:

#### [UC-1]: As a user, I would like enable SSA at the controller level so all Application are applied server-side

Implement a binary flag to configure ArgoCD to run all syncs using SSA.
(suggestion: `--server-side-apply=true`). Default value should be `false`.

#### [UC-2]: As a user, I would like enable SSA at the Application level so all resources are applied server-side

Implement a new syncOption to allow users to enable SSA at the application
level (Suggestion `ServerSideApply=true`). UI needs to be updated to support
this new sync option. If not informed, the controller should keep the current
behaviour (client-side).

#### [UC-3]: As a user, I would like enable SSA at the resource level so only a single manifest is applied server-side

Leverage the existing `argocd.argoproj.io/sync-options` annotation allowing the
`ServerSideApply=true` to be informed at the resource level. Must not impact
other sync-options informed in the annotation (make sure this annotation
supports providing multiple options).

### Security Considerations

TBD

### Risks and Mitigations

#### [R-1] Supported K8s version check

ArgoCD must check if the target Kubernetes cluster has full support for SSA. The
feature turned [GA in Kubernetes 1.22][8]. Full support for managed fields was
introduced as [beta in Kubernetes 1.18][9]. The implementation must check that
the target kubernetes cluster is running at least version 1.18. If SSA is
enabled and target cluster version < 1.18 ArgoCD should log warning and fallback
to client sync.

#### [R-2] Alternating Server-Side Client-Side syncs

Kubernetes SSA proposal ([KEP-555][13]) mentions about alternating between
server-side and client-side applies in the [Upgrade/Downgrade Strategy][12]
section. It is stated that Kubernetes will verify the incoming apply request
validating if the user-agent is `kubectl` to decide if the
`last-applied-configuration` annotation should be updated. ArgoCD relies on this
annotation and the implementation must make sure that this agent is correctly
informed when changing to server-side apply and specifying a manager different
than `kubectl`. This is mainly to make sure that
[G-5](#g-5-register-a-proper-manager) isn't impacting the
client-side/server-side compatibility.

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
[11]: https://github.com/argoproj/gitops-engine/pull/363#issuecomment-1013289982
[12]: https://github.com/kubernetes/enhancements/blob/master/keps/sig-api-machinery/555-server-side-apply/README.md#upgrade--downgrade-strategy
[13]: https://github.com/kubernetes/enhancements/blob/master/keps/sig-api-machinery/555-server-side-apply/README.md
[14]: https://github.com/argoproj/argo-cd/pull/8812#discussion_r849140565
