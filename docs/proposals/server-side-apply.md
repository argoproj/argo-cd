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

## Open Questions [optional]

* Should Server-Side Apply support in ArgoCD be implemented allowing multiple
  managers for the same controller?

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
- Diffing needs to support strategic merge patch ([ISSUE-2268][7])
- Allow Admission Controllers to execute even when there is no diff for a
  particular resource. (Needs investigation)
- ArgoCD should respect field ownership and provide a configuration to allow
  users to define the behavior in case of conflicts

## Non-Goals
What is out of scope for this proposal?
Listing non-goals helps to focus discussion and make progress

## Proposal
This is where we get down to details of what the proposal is about.
This is where we get down to details of what the proposal is about.

### Use cases
Add a list of detailed use cases this enhancement intends to take
care of. Add a list of detailed use cases this enhancement intends to take care
of.

#### Use case 1: As a user, I would like to understand the drift. (This is an
example)

#### Use case 2: As a user, I would like to take an action on the
deviation/drift. (This is an example)

### Implementation Details/Notes/Constraints [optional] What are the caveats to
the implementation? What are some important details that didn't come across
above. Go in to as much detail as necessary here. This might be a good place to
talk about core concepts and how they relate. concepts and how they relate. You
may have a work-in-progress Pull Request to demonstrate the functioning of the
enhancement you are proposing. You may have a work-in-progress Pull Request to
demonstrate the functioning of the enhancement you are proposing.

### Detailed examples

### Security Considerations
* How does this proposal impact the security aspects of Argo CD workloads ?
* Are there any unresolved follow-ups that need to be done to make the
  enhancement more robust ?  
    * Are there any unresolved follow-ups that need to be done to make the
      enhancement more robust ?  

### Risks and Mitigations
What are the risks of this proposal and how do we mitigate. Think broadly. What
are the risks of this proposal and how do we mitigate. Think broadly. For
example, consider both security and how this will impact the larger Kubernetes
ecosystem. both security and how this will impact the larger Kubernetes
ecosystem. Consider including folks that also work outside your immediate
sub-project. Consider including folks that also work outside your immediate
sub-project.


### Upgrade / Downgrade
Strategy If applicable, how will the component be upgraded and downgraded? Make
sure this is in the test plan.
Plan.
Consider the following in developing an
upgrade/downgrade strategy for this enhancement:
Consider the following in developing an upgrade/downgrade strategy for this
enhancement:
- What changes (in invocations, configurations, API use, etc.) is an existing
  cluster required to make on upgrade in order to keep previous behavior?
- What changes (in invocations, configurations, API use, etc.) is an existing
  cluster required to make on upgrade in order to make use of the enhancement?
  make on upgrade in order to make use of the enhancement?

## Drawbacks
The idea is to find the best form of an argument why this enhancement should
_not_ be implemented. The idea is to find the best form of an argument why this
enhancement should _not_ be implemented.

## Alternatives
Similar to the `Drawbacks` section the `Alternatives` section is used to
highlight and record other possible approaches to delivering the value proposed
by an enhancement. possible approaches to delivering the value proposed by an
enhancement.

[1]: https://kubernetes.io/docs/reference/using-api/server-side-apply/
[2]: https://github.com/argoproj/argo-cd/issues/2267#issuecomment-920445236
[3]: https://github.com/prometheus-community/helm-charts/issues/1500#issuecomment-1017961377
[4]: https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/
[5]: https://github.com/argoproj/argo-cd/issues/804
[6]: https://github.com/argoproj/argo-cd/issues/2267
[7]: https://github.com/argoproj/argo-cd/issues/2268
