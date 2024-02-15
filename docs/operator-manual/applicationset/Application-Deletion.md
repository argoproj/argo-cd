# Application Pruning & Resource Deletion

All `Application` resources created by the ApplicationSet controller (from an ApplicationSet) will contain:

- A `.metadata.ownerReferences` reference back to the *parent* `ApplicationSet` resource
- An Argo CD `resources-finalizer.argocd.argoproj.io` finalizer in `.metadata.finalizers` of the Application if `.syncPolicy.preserveResourcesOnDeletion` is set to false.

The end result is that when an ApplicationSet is deleted, the following occurs (in rough order):

- The `ApplicationSet` resource itself is deleted
- Any `Application` resources that were created from this `ApplicationSet` (as identified by owner reference)
- Any deployed resources (`Deployments`, `Services`, `ConfigMaps`, etc) on the managed cluster, that were created from that `Application` resource (by Argo CD), will be deleted.
    - Argo CD is responsible for handling this deletion, via [the deletion finalizer](../../../user-guide/app_deletion/#about-the-deletion-finalizer).
    - To preserve deployed resources, set `.syncPolicy.preserveResourcesOnDeletion` to true in the ApplicationSet.

Thus the lifecycle of the `ApplicationSet`, the `Application`, and the `Application`'s resources, are equivalent.

!!! note
    See also the [controlling resource modification](Controlling-Resource-Modification.md) page for more information about how to prevent deletion or modification of Application resources by the ApplicationSet controller.

It *is* still possible to delete an `ApplicationSet` resource, while preventing `Application`s (and their deployed resources) from also being deleted, using a non-cascading delete:
```
kubectl delete ApplicationSet (NAME) --cascade=orphan
```

!!! warning
    Even if using a non-cascaded delete, the `resources-finalizer.argocd.argoproj.io` is still specified on the `Application`. Thus, when the `Application` is deleted, all of its deployed resources will also be deleted. (The lifecycle of the Application, and its *child* objects, are still equivalent.)

    To prevent the deletion of the resources of the Application, such as Services, Deployments, etc, set `.syncPolicy.preserveResourcesOnDeletion` to true in the ApplicationSet. This syncPolicy parameter prevents the finalizer from being added to the Application.