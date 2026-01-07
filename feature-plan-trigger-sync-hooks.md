# Implementation Plan: Trigger Sync Hooks During Partial Syncs

This plan outlines the steps required to implement the "Trigger Sync Hooks During Partial Syncs" feature in Argo CD. This feature introduces an opt-in sync option `RunHooksOnPartialSync=true` to allow hooks to execute even when only a subset of resources is being synced.

## 1. Define Constants

- Add `SyncOptionRunHooksOnPartialSync = "RunHooksOnPartialSync=true"` to `common/common.go`.

## 2. Update Application Controller

### 2.1 Update Hook Filtering Logic in `controller/sync.go`

Currently, `controller/sync.go` filters resources during partial syncs (when `len(syncOp.Resources) > 0`) and excludes most hooks.

- Update `WithResourcesFilter` in `controller/sync.go` to include all hooks if the `RunHooksOnPartialSync=true` sync option is present.

```go
// Proposed change in controller/sync.go
runHooksOnPartialSync := syncOp.SyncOptions.HasOption(common.SyncOptionRunHooksOnPartialSync)

sync.WithResourcesFilter(func(key kube.ResourceKey, target *unstructured.Unstructured, live *unstructured.Unstructured) bool {
    return (len(syncOp.Resources) == 0 ||
        (runHooksOnPartialSync && hook.IsHook(target)) ||
        isPostDeleteHook(target) ||
        isPreDeleteHook(target) ||
        argo.ContainsSyncResource(key.Name, key.Namespace, schema.GroupVersionKind{Kind: key.Kind, Group: key.Group}, syncOp.Resources)) &&
        m.isSelfReferencedObj(live, target, app.GetName(), v1alpha1.TrackingMethod(trackingMethod), installationID)
}),
```

### 2.2 Verify Hook Eligibility in `controller/hook.go` (if necessary)

Ensure that `hook.IsHook(target)` correctly identifies all resources that should be considered hooks during this phase.

## 3. Update CLI

### 3.1 Update `argocd app sync` Command

- Modify `NewApplicationSyncCommand` in `cmd/argocd/commands/app.go` to add the `--sync-option` flag.
- Update the command logic to parse these options and include them in the `ApplicationSyncRequest`.

## 4. Documentation Updates

- Update `docs/user-guide/sync-options.md` to include the new `RunHooksOnPartialSync` option.
- Ensure CLI help text for `argocd app sync --help` reflects the new capability.

## 5. Testing Strategy (TDD)

### 5.1 Unit Tests – Sync Engine (`controller/sync_test.go`)

Add the following test cases:
1. `TestPartialSync_DoesNotRunHooks_ByDefault`: Verify hooks are skipped during partial sync when no option is set.
2. `TestPartialSync_RunsHooks_WhenOptionEnabled`: Verify hooks are included when `RunHooksOnPartialSync=true` is set.
3. `TestFullSync_RunsHooks_IrrespectiveOfOption`: Verify hooks still run during full syncs.

### 5.2 Hook Eligibility Tests (`controller/hook_test.go`)

- Add tests to ensure `RunHooksOnPartialSync` correctly influences hook inclusion in the resource tree.

### 5.3 RBAC and Permission Tests (`server/application/application_test.go`)

- Ensure that permissions are still correctly enforced and that enabling hooks doesn't bypass any project-level resource restrictions.

### 5.4 End-to-End Tests (`test/e2e/`)

- Create an E2E test that performs a partial sync with `RunHooksOnPartialSync=true` and verifies that hook pods are created and executed.
