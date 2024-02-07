# Server-Side Apply with Unique Field Managers

This proposal is a follow up on the [original Server-Side Apply proposal](./server-side-apply.md), and seeks to make Argo more flexible and give users the ability to apply changes to shared Kuberentes resources at a granular level. It also considers how to remove field-level changes when an Application is destroyed.

To quote the [Kubernetes docs][1]:

> Server-Side Apply helps users and controllers manage their resources through declarative configuration. Clients can create and modify objects declaratively by submitting their _fully specified intent_.

> A fully specified intent is a partial object that only includes the fields and values for which the user has an opinion. That intent either creates a new object (using default values for unspecified fields), or is combined, by the API server, with the existing object.

## Example

Consider the case where you're working on a new feature for your app. You'd like to review these changes without disturbing your staging environment, so you use an `ApplicationSet` with a pull request generator to create [review apps][3] dynamically. These review apps configure a central Consul `IngressGateway`. Each review app needs to add a service to the `IngressGateway` upon creation, and remove that service upon deletion.

```yaml
apiVersion: consul.hashicorp.com/v1alpha1
kind: IngressGateway
metadata:
  name: review-app-ingress-gateway
  namespace: default
spec:
  listeners:
  - port: 443
    protocol: http
    services:
    - name: review-app-1
      namespace: my-cool-app
    - name: review-app-2
      namespace: my-groovy-app
    - name: review-app-3
      namespace: my-incredible-app
```

---

## Open Questions

### [Q-1] What should the behavior be for a server-side applied resource upon Application deletion?

The current behavior is to delete all objects "owned" by the Application. A user could choose to leave the resource behind with `Prune=false`, but that would also leave behind any config added to the shared object. Should the default delete behavior for server-side applied resources be to remove any fields that match that Application's [field manager][2]?

### [Q-2] What sync status should the UI display on a shared resource?

If an Application has successfully applied its partial spec to a shared resource, should it display as "in sync"? Or should it show "out of sync" when there are other changes to the shared object?

## Summary

ArgoCD supports server-side apply, but it uses the same field manager, `argocd-controller`, no matter what Application is issuing the sync. Letting users set a field manager, or defaulting to a unique field manager per application would enable users to:

- Manage only specific fields they care about on a shared resource
- Avoid deleting or overwriting fields that are managed by other Applications

## Motivation

There exist situations where disparate Applications need to add or remove configuration from a shared Kubernetes resource. Server-side apply supports this behavior when different field managers are used.

## Goals

All following goals should be achieve in order to conclude this proposal:

#### [G-1] Applications can apply changes to a shared resource without disturbing existing fields

A common use case of server-side apply is the ability to manage only particular fields on a share Kubernetes resource, while leaving everything else in tact. This requires a unique field manager for each identity that shares the resource.

#### [G-2] Applications that are destroyed only remove the fields they manage from shared resources

A delete operation should undo only the additions or updates it made to a shared resource, unless that resource has `Prune=true`.

#### [G-3] Users can define a field manager as a sync option

Some users may rely on the current behavior emanating from the use of the same field manager across all Applications. The current behavior being that each server side apply sync overwrites all fields on a resource. In other words, "latest sync wins."

## Non-Goals

N/A

## Proposal

1. Add a new sync option named `FieldManager` that accepts a string up to 128 characters in length. This can only be set on an individual resource. Don't allow this sync option to be set on the Application: accidentally overriding the resource-level field manager may have undesirable side effects like leaving orphaned fields behind. When a sync is performed for a resource and server side-apply is enabled, it uses the `FieldManager` if it is set, otherwise it defaults to the hard-coded `argocd-controller` field manager.

1. Like other sync options, add a corresponding text field in the ArgoCD UI to let users specify a field manager for a server-side apply sync. This text field should only be visible when a single resource is being synced.

1. Change the removal behavior for shared resources. When a resource with a custom field manager is "removed", it instead removes only the fields managed by its field manager from the shared resource by sending an empty "fully specified intent" using server-side apply. You can fully delete a shared resource by setting `Prune=true` at the resource level.

1. Add documentation suggesting that users might want to consider changing the permissions on the ArgoCD role to disallow the `delete` and `update` verbs on shared resources. Server-side apply will always use `patch`, and removing `delete` and `update` helps prevent users from errantly wiping out changes made from other Applications.

1. Add documentation that references the Kubernetes docs to show users how to properly define their Custom Resource Definition so that patches merge with expected results. For example, should a particular array on the resource be replaced or added to when patching?

### Use cases

The following use cases should be implemented:

#### [UC-1]: As a user, I would like to manage specific fields on a Kubernetes object shared by other ArgoCD Applications.

Change the Server-Side Apply field manager to be set by a sync option, but default to the constant `ArgoCDSSAManager`.

#### [UC-2]: As a user, I would like explict control of which field manager my Application uses for server-side apply.

Add a sync option named `FieldManager` that can be set via annotation on individual resources that controls which field manager is used.

### Security Considerations

TBD

### Risks and Mitigations

#### [R-1] Field manager names are limited to 128 characters

We should trim every field manager to 128 characters.

### Upgrade / Downgrade

TBD

## Drawbacks

- Increase in ArgoCD code base complexity.
- All current sync options are booleans. Adding a `FieldManager` option would be a string.


[1]: https://kubernetes.io/docs/reference/using-api/server-side-apply/
[2]: https://kubernetes.io/docs/reference/using-api/server-side-apply/#managers
[3]: https://docs.gitlab.com/ee/ci/review_apps/
