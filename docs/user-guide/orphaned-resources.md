# Orphaned Resources Monitoring

An [orphaned Kubernetes resource](https://kubernetes.io/docs/concepts/architecture/garbage-collection/#orphaned-dependents) is a top-level namespaced resource that does not belong to any Argo CD Application. The Orphaned Resources Monitoring feature allows detecting
orphaned resources, inspecting/removing resources using the Argo CD UI, and generating a warning.

The Orphaned Resources monitoring is enabled in the [Project](projects.md) settings.
Below is an example of enabling the feature using the AppProject custom resource.

```yaml
kind: AppProject
metadata:
  ...
spec:
  ...
  orphanedResources:
    warn: true
...
```

Once the feature is enabled, each project application that has any orphaned resources in its target namespace
will get a warning. The orphaned resources can be located using the application details page by enabling the "Show Orphaned" filter:

![orphaned resources](../assets/orphaned-resources.png)

When enabling the feature, you might want to consider disabling warnings at first.

```yaml
spec:
  orphanedResources:
    warn: false # Disable warning
```

When warnings are disabled, application users can still view orphaned resources in the UI.

## Exceptions

Not every resource in the Kubernetes cluster is controlled by the end user and managed by Argo CD. Other operators in the cluster can automatically create resources (e.g., the cert-manager creating secrets), which are then considered orphaned.

The following resources are never considered orphaned:

* Namespaced resources denied in the project. Usually, such resources are managed by cluster administrators and are not supposed to be modified by a namespace user.
* `ServiceAccount` with the name `default` (and the corresponding auto-generated `ServiceAccountToken`).
* `Service` with the name `kubernetes` in the `default` namespace.
* `ConfigMap` with the name `kube-root-ca.crt` in all namespaces.

You can prevent resources from being declared orphaned by providing a list of ignore rules, each defining a Group, Kind, and Name.

```yaml
spec:
  orphanedResources:
    ignore:
    - kind: ConfigMap
      name: orphaned-but-ignored-configmap
```

The `name` can be a [glob pattern](https://github.com/gobwas/glob), e.g.:

```yaml
spec:
  orphanedResources:
    ignore:
    - kind: Secret
      name: *.example.com
```
