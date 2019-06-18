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

If you want to exclude a whole class of objects globally, consider setting `resource.customizations` in [system level configuation](../user-guide/diffing.md#system-level-configuration). 
    
