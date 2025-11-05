# App Deletion

Apps can be deleted with or without a cascade option. A **cascade delete**, deletes both the app and its resources, rather than only the app.

## Deletion Using `argocd`

To perform a non-cascade delete:

```bash
argocd app delete APPNAME --cascade=false
```

To perform a cascade delete:

```bash
argocd app delete APPNAME --cascade
```

or

```bash
argocd app delete APPNAME
```

## Deletion Using `kubectl`

To perform a non-cascade delete, make sure the finalizer is unset and then delete the app:

```bash
kubectl patch app APPNAME  -p '{"metadata": {"finalizers": null}}' --type merge
kubectl delete app APPNAME
```

To perform a cascade delete set the finalizer, e.g. using `kubectl patch`:

```bash
kubectl patch app APPNAME  -p '{"metadata": {"finalizers": ["resources-finalizer.argocd.argoproj.io"]}}' --type merge
kubectl delete app APPNAME
```

## About The Deletion Finalizer

```yaml
metadata:
  finalizers:
    # The default behaviour is foreground cascading deletion
    - resources-finalizer.argocd.argoproj.io
    # Alternatively, you can use background cascading deletion
    # - resources-finalizer.argocd.argoproj.io/background
```

When deleting an Application with this finalizer, the Argo CD application controller will perform a cascading delete of the Application's resources.

Adding the finalizer enables cascading deletes when implementing [the App of Apps pattern](../operator-manual/cluster-bootstrapping.md#cascading-deletion).

The default propagation policy for cascading deletion is [foreground cascading deletion](https://kubernetes.io/docs/concepts/architecture/garbage-collection/#foreground-deletion).
Argo CD performs [background cascading deletion](https://kubernetes.io/docs/concepts/architecture/garbage-collection/#background-deletion) when `resources-finalizer.argocd.argoproj.io/background` is set.

When you invoke `argocd app delete` with `--cascade`, the finalizer is added automatically.
You can set the propagation policy with `--propagation-policy <foreground|background>`.

## Deleting Applications in the UI

Argo CD provides a consistent deletion experience across different views in the UI. When deleting applications, you can access the delete functionality from:

- **Applications List View**: The main applications page showing all applications
- **Application Details View - Resource Tree**: When viewing an application's resource tree that contains child applications

### Consistent Deletion Behaviour

Starting in Argo CD 3.2, deletion behavior is now **consistent** across all UI views. Whether you delete an application from the Applications List or from the Resource Tree view, the same deletion mechanism and options are used.

Previously, deleting an application from the Resource Tree treated it as a generic Kubernetes resource, which could lead to unexpected behaviour with non-cascading deletes. Now, Argo CD properly detects Application resources and uses the standard Application deletion API in all contexts.

### Deleting Child Applications in App of Apps Pattern

When using the [App of Apps pattern](../operator-manual/cluster-bootstrapping.md), parent applications can contain child applications as resources. Argo CD automatically detects child applications and provides improved dialog messages to help you understand what you're deleting.

#### Child Application Detection

Argo CD identifies a child application by checking for the `app.kubernetes.io/part-of` label. If this label is present and has a non-empty value, the application is considered a child application.

#### Delete Dialog Differences

**When deleting a child application:**

- Dialog title: "Delete child application"
- Confirmation prompt references "child application" to make it clear you're deleting a managed application
- Additional warning note appears when deleting from the Resource Tree

**When deleting a regular application:**

- Dialog title: "Delete application"
- Standard confirmation prompt

**When deleting from the Resource Tree:**

An additional informational note appears:

> ⚠️ **Note:** You are about to delete an Application from the resource tree. This uses the same deletion behaviour as the Applications list page.

This note clarifies that the deletion will use the proper Application deletion API, not generic Kubernetes resource deletion.

### Deletion Options (Propagation Policies)

When deleting an application through the UI, you can choose from three propagation policies:

#### 1. Foreground (Default)

- Deletes the application and all its managed resources
- Waits for all managed resources to be deleted before the Application is removed
- **Use case**: When you want to ensure all resources are cleaned up before the Application disappears

#### 2. Background

- Deletes the application and all its managed resources
- The Application is removed immediately, and resources are deleted in the background
- **Use case**: When you want faster Application deletion and don't need to wait for resource cleanup

#### 3. Non-Cascading (Orphan)

- Deletes **only** the Application resource
- All managed resources (Deployments, Services, ConfigMaps, etc.) are **preserved** in the cluster
- The finalizer is removed automatically before deletion
- **Use case**: When you want to stop managing resources through Argo CD but keep them running

> [!WARNING]
> **Important for Non-Cascading Deletes**
>
> When you select **Non-Cascading**, Argo CD will:
> - Remove the `resources-finalizer.argocd.argoproj.io` finalizer from the Application
> - Delete only the Application resource
> - Leave all managed resources (Pods, Services, Deployments, etc.) running in the cluster
>
> This behaviour is now **consistent** whether you delete from the Applications List or from the Resource Tree view.

### Best Practices for App of Apps Pattern

When working with the App of Apps pattern:

1. **Understand the impact**: Deleting a child application with Foreground or Background propagation will delete all of its managed resources
2. **Review before deleting**: Always verify what resources are managed by the application before performing a cascading delete
3. **Use Non-Cascading cautiously**: If you only want to remove the Application resource but keep the deployed workloads, use Non-Cascading delete
4. **Consider finalizers**: Ensure child applications have appropriate finalizers set based on your deletion strategy (see [Cascading Deletion](../operator-manual/cluster-bootstrapping.md#cascading-deletion))

### Example Scenarios

#### Scenario 1: Deleting a child application and all its resources

1. Navigate to the parent application's Resource Tree
2. Click the kebab menu (button with the three vertical dots) on a child Application resource
3. Select "Delete"
4. Choose **Foreground** or **Background** propagation policy
5. Confirm the deletion

**Result**: The child Application and all its managed resources (Deployments, Services, etc.) are deleted.

#### Scenario 2: Removing Argo CD management but keeping resources

1. Navigate to the Applications List or the parent application's Resource Tree
2. Click the kebab menu (button with the three vertical dots) on a child Application resource
3. Select "Delete"
4. Choose **Non-Cascading** propagation policy
5. Confirm the deletion

**Result**: Only the Application resource is deleted. All managed resources continue running in the cluster.

#### Scenario 3: Deleting from Resource Tree with context awareness

When you delete a child application from the Resource Tree view:

- Argo CD recognizes it as an Application resource (not just a generic Kubernetes resource)
- Shows "Delete child application" dialog if it detects the `app.kubernetes.io/part-of` label
- Displays an informational note explaining you're using the same behaviour as the Applications List
- Provides the same three propagation policy options

This ensures predictable and consistent deletion behaviour regardless of where you initiate the deletion.
