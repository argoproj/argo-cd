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

Support manual approval for pruning and deleting Kubernetes resources during application syncing/deletion.

## Summary

Introduce Kubernetes resource-level annotations that require manual user approval using Argo CD UI/CLI/API before the
resource is pruned or deleted. The annotations should be respected while Argo CD attempts to synchronize or delete the
application.

## Motivation

We’ve seen cases where Argo CD deleted Kubernetes resources due to a bug or misconfiguration.​ Examples include [corrupted
data](https://github.com/argoproj/argo-cd/issues/4423) in Redis, user errors
([1](https://github.com/argoproj/argo-cd/issues/9093), [2](https://github.com/argoproj/argo-cd/issues/4844))
and [bug](https://github.com/argoproj/argo-cd/issues/3473) in the automation on top of Argo CD. These examples don’t
mean Argo CD is not reliable; however, there are cases where misbehavior is catastrophic, and erroneous deletion is not
acceptable. Examples include the app-of-apps pattern where Argo CD is used to manage itself, or namespaces in production
clusters.

### Goals

The goals of a proposal ares:

#### Allow developers to mark resources that require manual approval before application deletion.

Developers should be able to add an annotation to resources that require manual approval before deletion. The annotation
should be respected by Argo CD when it attempts to delete the application.

#### Allow developers to mark resources that require manual approval before pruning

Developers should be able to add an annotation to resources that require manual approval before pruning. The annotation
should be respected by Argo CD when it attempts to prune extra resources while syncing the application.

### Non-Goals

#### Implement automatic self check while deleting resources

We've made our best effort to implement corrected behavior, and as of now, we are not aware of any bugs that cause
erroneous deletion. The goal of this proposal is to provide a safety net for cases where deletion is not acceptable.

## Proposal

It is proposed to introduce two new sync options for Argo CD applications: `Prune=confirm` and `Delete=confirm`. Options would
protect resources from accidental deletion during cascading application deletion as well as during sync operations.

### Introduce `confirm` option for Prune sync option.

Argo CD already supports `argocd.argoproj.io/sync-options: Prune=false` sync option that prevents resource deletion while syncing
the application. This, however, is not ideal since it prevents implementing fully automated workflows that include resource deletion.

In order to improve the situation, we propose to introduce `confirm` option for Prune sync option. When `confirm` option is set, Argo CD should pause the sync operation
**before deleting any app resources** and wait for the user to confirm the deletion. The confirmation can be done in a very friendly way using Argo CD UI, CLI or API.

* **Sync Operation status**. I suggest not to introduce new sync operation states to avoid disturbing the existing automation around syncing (CI pipelines, scripts etc). 
  If Argo CD is waiting for the operation state should remain `Progressing`. Once the user confirms the deletion, the operation should resume.
* **Sync Waves**. The sync wave shuold be "paused" while Argo CD is waiting for the user to confirm the deletion. No difference from waiting for the resource to became healthy.

### Introduce `confirm` option for Delete sync option.

Similarly to `Prune` sync option we need to introduce `confirm` value for `Delete` sync option: `argocd.argoproj.io/sync-options: Delete=confirm`. The `confirm` option
should pause the sync operation **before deleting any app resources** and wait for the user to confirm the deletion. The confirmation can be done in a very friendly way
using Argo CD UI, CLI or API.


### Friendly prunning/deletion manual approval

Since we know Argo CD is often used to implement fully automated developer workflows that include resource deletion, the
deletion approval process should be as painless as possible. This way, platform administrators can instruct end users to
apply the new prune/delete option to resources that require special care without significantly disturbing the developer
experience.

In both cases where Argo CD requires manual approval, the user should be able to approve the deletion using Argo CD UI,
CLI, or API. The approval process should be as simple as possible and should not require the user to understand the
internals of Argo CD.

#### New `requiresDeletionApproval` resource field in application status

A new field `requiresDeletionApproval` should be added to the `status.resources` list items. The field should be set to `true` when the resource deletion approval is required.

```yaml
  - health:
      status: Healthy
    kind: Service
    name: guestbook-ui
    namespace: default
    status: OutOfSync
    version: v1
    requiresPruning: true
    requiresDeletionApproval: true # new field that indicates that deletion approval is required
```

The Argo CD UI, CLI should visualize the `requiresDeletionApproval` field so that the user can easily discover which resources require manual approval.

#### Approve deletion resource action

The Argo CD UI, CLI should bundle the `Approve Deletion` [resource action](https://argo-cd.readthedocs.io/en/stable/operator-manual/resource_actions/)
that would allow the user to approve the deletion. The action should patch the resource with the `argocd.argoproj.io/deletion-approved: true` annotation.
Once annotation is applied the Argo CD should proceed with the deletion.

The main reason to use the action is that we can reuse existing [RBAC](https://argo-cd.readthedocs.io/en/stable/operator-manual/rbac/) to control who can approve the deletion.

#### UI/CLI Convinience to approve all resources

The Argo CD UI should provide a convinient way to approve resources that require manual approval. The existing user interface will provide a button that allows end user
execute the `Approve Deletion` action and approve resources one by one. In addition to the single resource approval, the UI should provide a way to approve all resources
that require manual approval. The new button should execute the `Approve Deletion` action for all resources that require manual approval.

Argo CD CLI would no need changes since existing `argocd app actions run` command allows to execute an action against multiple resources.

#### Require deletion approval notification

The default Argo CD notification catalog should include a trigger and notification template that notifies the user when
deletion approval is required. The notification template should include a list of resources that require approval.


#### Declarative approval

The user should be able to approve resource deletion without using the UI or CLI by manually adding the `argocd.argoproj.io/deletion-approved: true` annotation to the resource.

### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

## Use case 1:

As a developer, I would like to mark resources that require manual pruning approval so I can prevent the accidental deletion of critical resources.

## Use case 2:

As a developer, I would like to mark resources that require manual deletion approval so I can prevent the accidental deletion of critical resources.


### Security Considerations

The resource approval would require a mechanism to control who can approve the deletion. The proposal to use
resource-level actions solves this problem and allows us to reuse the existing RBAC model.

### Risks and Mitigations

None.

### Upgrade / Downgrade Strategy

In case of rollback to the previous version the sync option would be ignored and the resources would be deleted as before.

## Open Issues

The proposal would require end users to learn about the new behavior and adjust their workflows. It includes a set of
enhancements aimed at minimizing the impact on end users.

## Alternatives

None.