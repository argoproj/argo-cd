# Sync Options

## No Prune Resources

You may wish to prevent an object from being pruned:

```yaml
metadata:
  annotations:
    argocd.argoproj.io/sync-options: NoPrune
```

!!! note
    The app will be out of sync if Argo CD expects a resource to be pruned. You may wish to use this along with [compare options](compare-options.md).

