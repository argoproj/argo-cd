# Orphaned Resources Monitoring

Orphaned Kubernetes resource is a top-level namespaced resource which does not belong to any Argo CD Application. The Orphaned Resources Monitoring feature allows detecting
orphaned resources, inspect/remove resources using Argo CD UI and generate a warning.

The Orphaned Resources monitoring is enabled in [Project](projects.md) settings, 
and the below is an example of enabling the feature using the AppProject custom resource.

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

Once the feature is enabled, each project application which has any orphaned resources in its target namespace
will get a warning. The orphaned resources can be located using the application details page:

![orphaned resources](../assets/orphaned-resources.png)

When enabling the feature, you might want to consider disabling warning at first.

```yaml
spec:
  orphanedResources:
    warn: false # Disable warning
```

While warning disabled, application users can still view orphaned resources in the UI.

## Exceptions

Not every resource in the Kubernetes cluster is controlled by the end user. Following resources are never considered as orphaned:

* Namespaced resources denied in the project. Usually, such resources are managed by cluster administrators and not supposed to be modified by namespace user.
* `ServiceAccount` with name `default` ( and corresponding auto-generated `ServiceAccountToken` ).
* `Service` with name `kubernetes` in the `default` namespace.
* `ConfigMap` with name `kube-root-ca.crt` in all namespaces.

Also, you can configure to ignore resources by providing a list of resource Group, Kind and Name.

```yaml
spec:
  orphanedResources:
    ignore:
    - kind: ConfigMap
      name: orphaned-but-ignored-configmap
```
