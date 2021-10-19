# Sync Options

## No Prune Resources

>v1.1

You may wish to prevent an object from being pruned:

```yaml
metadata:
  annotations:
    argocd.argoproj.io/sync-options: Prune=false
```

In the UI, the pod will simply appear as out-of-sync:

![sync option no prune](../assets/sync-option-no-prune.png)


The sync-status panel shows that pruning was skipped, and why:

![sync option no prune](../assets/sync-option-no-prune-sync-status.png)

The app will be out of sync if Argo CD expects a resource to be pruned. You may wish to use this along with [compare options](compare-options.md).

## Disable Kubectl Validation

>v1.2

For a certain class of objects, it is necessary to `kubectl apply` them using the `--validate=false` flag. Examples of this are kubernetes types which uses `RawExtension`, such as [ServiceCatalog](https://github.com/kubernetes-incubator/service-catalog/blob/master/pkg/apis/servicecatalog/v1beta1/types.go#L497). You can do using this annotations:


```yaml
metadata:
  annotations:
    argocd.argoproj.io/sync-options: Validate=false
```

If you want to exclude a whole class of objects globally, consider setting `resource.customizations` in [system level configuration](../user-guide/diffing.md#system-level-configuration). 
    
## Skip Dry Run for new custom resources types

>v1.6

When syncing a custom resource which is not yet known to the cluster, there are generally two options:

1) The CRD manifest is part of the same sync. Then ArgoCD will automatically skip the dry run, the CRD will be applied and the resource can be created.
2) In some cases the CRD is not part of the sync, but it could be created in another way, e.g. by a controller in the cluster. An example is [gatekeeper](https://github.com/open-policy-agent/gatekeeper),
which creates CRDs in response to user defined `ConstraintTemplates`. ArgoCD cannot find the CRD in the sync and will fail with the error `the server could not find the requested resource`.

To skip the dry run for missing resource types, use the following annotation:

```yaml
metadata:
  annotations:
    argocd.argoproj.io/sync-options: SkipDryRunOnMissingResource=true
```

The dry run will still be executed if the CRD is already present in the cluster.

## Selective Sync

Currently when syncing using auto sync ArgoCD applies every object in the application. 
For applications containing thousands of objects this takes quite a long time and puts undue pressure on the api server.
Turning on selective sync option which will sync only out-of-sync resources. 

You can add this option by following ways

1) Add `ApplyOutOfSyncOnly=true` in manifest

Example:

```yaml
syncPolicy:
  syncOptions:
    - ApplyOutOfSyncOnly=true
``` 

2) Set sync option via argocd cli

Example:

```bash
$ argocd app set guestbook --sync-option ApplyOutOfSyncOnly=true
```

## Resources Prune Deletion Propagation Policy

By default, extraneous resources get pruned using foreground deletion policy. The propagation policy can be controlled
using `PrunePropagationPolicy` sync option. Supported policies are background, foreground and orphan.
More information about those policies could be found [here](https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/#controlling-how-the-garbage-collector-deletes-dependents).

```yaml
syncOptions:
- PrunePropagationPolicy=foreground
```

## Prune Last

This feature is to allow the ability for resource pruning to happen as a final, implicit wave of a sync operation, 
after the other resources have been deployed and become healthy, and after all other waves completed successfully. 

```yaml
syncOptions:
- PruneLast=true
```

This can also be configured at individual resource level.
```yaml
metadata:
  annotations:
    argocd.argoproj.io/sync-options: PruneLast=true
```

## Replace Resource Instead Of Applying Changes

By default, Argo CD executes `kubectl apply` operation to apply the configuration stored in Git. In some cases
`kubectl apply` is not suitable. For example, resource spec might be too big and won't fit into
`kubectl.kubernetes.io/last-applied-configuration` annotation that is added by `kubectl apply`. In such cases you
might use `Replace=true` sync option:


```yaml
syncOptions:
- Replace=true
```

If the `Replace=true` sync option is set the Argo CD will use `kubectl replace` or `kubectl create` command to apply changes.

This can also be configured at individual resource level.
```yaml
metadata:
  annotations:
    argocd.argoproj.io/sync-options: Replace=true
```
