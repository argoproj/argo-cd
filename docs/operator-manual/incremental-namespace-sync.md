# Incremental Namespace Sync

> [!WARNING]
> **Alpha Feature (Since v3.3.0)**
>
> This is an experimental,
> [alpha-quality](https://github.com/argoproj/argoproj/blob/main/community/feature-status.md#alpha)
> feature. It may be removed in future releases or modified in
> backwards-incompatible ways.

*Current Status: [Alpha][1] (Since v3.3.0)*

By default, when Argo CD detects namespace changes in a cluster configuration,
it invalidates the entire cluster cache and performs a full resync of all
namespaces. For clusters with many namespaces this can be slow and
resource-intensive, as it requires listing all resources across all namespaces
and re-establishing watches. During this time, cluster cache synchronization
blocks all reconciliation operations, preventing applications from reconciling
until the sync completes.

Starting with v3.3.0, Argo CD supports incremental namespace synchronization.
When namespaces are added or removed from the cluster configuration, only the
affected namespaces are synchronized instead of triggering a full cluster cache
rebuild.

## Benefits

- **Faster sync times**: Only sync the changed namespace instead of all namespaces
- **Faster application reconciliation**: Cache sync blocks reconciliation, so faster sync means less downtime for application updates
- **Reduced API calls**: Avoid listing resources in unchanged namespaces (10-50x reduction)
- **Better scalability**: Enables managing clusters with many (100+) namespaces efficiently
- **Reduced API server load**: Fewer LIST requests to the Kubernetes API server

## Enabling Incremental Namespace Sync

This feature is disabled by default while it is in alpha status. You can enable
it using either a ConfigMap or environment variable.

### Method 1: Via ConfigMap (Recommended)

Update the `argocd-cmd-params-cm` ConfigMap:

```bash
kubectl patch configmap argocd-cmd-params-cm -n argocd --type merge \
  -p '{"data":{"controller.incremental.namespace.sync.enabled":"true"}}'
```

Restart the controller:

```bash
kubectl rollout restart statefulset argocd-application-controller -n argocd
```

### Method 2: Via Environment Variable

Edit the controller StatefulSet:

```bash
kubectl edit statefulset argocd-application-controller -n argocd
```

Add the environment variable:

```yaml
spec:
  template:
    spec:
      containers:
      - name: argocd-application-controller
        env:
        - name: ARGOCD_ENABLE_INCREMENTAL_NAMESPACE_SYNC
          value: "true"
```

The controller pods will automatically restart to apply the change.

### Method 3: Using kubectl set env

Set the environment variable directly:

```bash
kubectl set env statefulset/argocd-application-controller \
  ARGOCD_ENABLE_INCREMENTAL_NAMESPACE_SYNC=true -n argocd
```

This will trigger a rolling update of the controller pods.

## How It Works

When incremental namespace sync is enabled, the application controller uses a
diff-based approach to detect namespace changes:

1. **Compare namespace lists**: The controller compares the desired namespace set
   (from cluster configuration) with the currently tracked namespace set (in cache)

2. **Calculate additions and removals**:
   - **Added namespaces**: Namespaces in the desired set but not in the current set
   - **Removed namespaces**: Namespaces in the current set but not in the desired set

3. **Apply incremental updates**:
   - **For added namespaces**: Start watches and sync resources only in the new namespace
   - **For removed namespaces**: Stop watches and remove cached resources only for that namespace
   - **For unchanged namespaces**: No action taken - existing watches and cache remain

This approach avoids the need to rebuild the entire cluster cache, which traditionally
requires re-listing all resources across all namespaces.

## When to Use This Feature

Consider enabling this feature if:

- ✅ You manage many namespaces per cluster
- ✅ You frequently **add or remove namespaces** from cluster configurations
- ✅ You experience **slow cluster sync times**
- ✅ You see **API server rate limiting** or performance issues

## Limitations and Considerations

- **Alpha status**: The feature may change in future versions
- **Namespace changes only**: Only optimizes namespace additions/removals, not other
  cluster configuration changes (e.g., resource exclusions, API groups)
- **Requires restart**: Enabling/disabling the feature requires controller restart
- **Fallback behavior**: If incremental sync fails, the controller falls back to full
    cluster cache invalidation to maintain consistency.

## Monitoring

You can monitor the feature's impact using:

### Log-based Monitoring

Parse controller logs to track namespace sync events:

```bash
kubectl logs -n argocd statefulset/argocd-application-controller | \
  grep -E "syncing namespace|Namespace successfully synced|Failed to sync namespace"
```
