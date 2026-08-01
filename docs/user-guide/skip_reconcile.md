# Skip Application Reconcile

> [!WARNING]
> **Alpha Feature (Since v2.7.0)**
>
> This is an experimental, [alpha-quality](https://github.com/argoproj/argoproj/blob/main/community/feature-status.md#alpha) feature.
> The primary use case is to provide integration with third party projects.
> This feature may be removed in future releases or modified in backwards-incompatible ways.

Argo CD allows users to stop an Application from reconciling.
The skip reconcile option is configured with the `argocd.argoproj.io/skip-reconcile: "true"` annotation.
When the Application is configured to skip reconcile,
all processing is stopped for the Application.
During the period of time when the Application is not processing,
the Application `status` field will not be updated.
If an Application is newly created with the skip reconcile annotation,
then the Application `status` field will not be present.
To resume the reconciliation or processing of the Application,
remove the annotation or set the value to `"false"`.

See the below example for enabling an Application to skip reconcile:

```yaml
metadata:
  annotations:
    argocd.argoproj.io/skip-reconcile: "true"
```

See the below example for an Application that is newly created with the skip reconcile enabled:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/skip-reconcile: "true"
  name: guestbook
  namespace: argocd
spec:
  destination:
    namespace: guestbook
    server: https://kubernetes.default.svc
  project: default
  source:
    path: guestbook
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
```

The `status` field is not present.

## ApplicationSet

The same `argocd.argoproj.io/skip-reconcile: "true"` annotation is honored by the
ApplicationSet controller. When set on an ApplicationSet, the controller stops
reconciling it: the generated Applications are not created, updated, or pruned, and the
ApplicationSet `status` is not updated. Remove the annotation (or set it to `"false"`)
to resume.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  annotations:
    argocd.argoproj.io/skip-reconcile: "true"
  name: guestbook
  namespace: argocd
spec:
  # ...
```

This is used internally by `argocd app rename` when renaming an Application that is
managed by an ApplicationSet (or an app-of-apps parent): the managing owner is frozen
with this annotation for the duration of the rename so it cannot recreate or prune the
old-named child mid-swap. After updating the source of truth (the generator or the
app-of-apps Git manifest) to the new name, remove the annotation to let the owner
re-adopt the renamed child.

## Primary Use Case

The skip reconcile option is intended to be used with third party projects that wishes 
to make updates to the Application status without having the changes being overwritten by the Application controller.
An example of this usage is the [Open Cluster Management (OCM)](https://github.com/open-cluster-management-io/) project using 
[pull-integration](https://github.com/open-cluster-management-io/argocd-pull-integration) controller.
In the example, the hub cluster Application is not meant to be reconciled by the Argo CD Application controller.
Instead, the OCM pull-integration controller will populate the primary/hub cluster Application status 
using the collected Application status from the remote/spoke/managed cluster.

## Alternative Use Cases

There are other alternative use cases for this skip reconcile option. 
It's important to note that this is an experimental, alpha-quality feature 
and the following use cases are generally not recommended.

* Ease of debugging when the Application reconcile is skipped.
* Orphan resources without deleting the Application might provide a safer way to migrate applications.
* ApplicationSet can generate dry-run like Applications that don't reconcile automatically. 
* Pause and resume Applications reconcile during a disaster recovery process.
* Provide another alternative approval flow by not allowing an Application to start reconciling right away.
