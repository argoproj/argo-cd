# Reconcile Optimization

By default, an Argo CD Application is refreshed everytime a resource that belongs to it changes.

Kubernetes controllers often update the resources they watch periodically, causing continuous reconcile operation on the Application
and a high CPU usage on the `argocd-application-controller`. Argo CD allows you to optionally ignore resource updates on specific fields
for [tracked resources](../user-guide/resource_tracking.md).

When a resource update is ignored, if the resource's [health status](./health.md) does not change, the Application that this resource belongs to will not be reconciled.

## System-Level Configuration

Argo CD allows ignoring resource updates at a specific JSON path, using [RFC6902 JSON patches](https://tools.ietf.org/html/rfc6902) and [JQ path expressions](https://stedolan.github.io/jq/manual/#path(path_expression)). It can be configured for a specified group and kind
in `resource.customizations` key of the `argocd-cm` ConfigMap.

The feature is behind a flag. To enable it, set `resource.ignoreResourceUpdatesEnabled` to `"true"` in the `argocd-cm` ConfigMap.

Following is an example of a customization which ignores the `refreshTime` status field of an [`ExternalSecret`](https://external-secrets.io/main/api/externalsecret/) resource:

```yaml
data:
  resource.customizations.ignoreResourceUpdates.external-secrets.io_ExternalSecret: |
    jsonPointers:
    - /status/refreshTime
```

It is possible to configure `ignoreResourceUpdates` to be applied to all tracked resources in every Application managed by an Argo CD instance. In order to do so, resource customizations can be configured like in the example below:

```yaml
data:
  resource.customizations.ignoreResourceUpdates.all: |
    jsonPointers:
    - /status
```

### Using ignoreDifferences to ignore reconcile

It is possible to use existing system-level `ignoreDifferences` customizations to ignore resource updates as well. Instead of copying all configurations,
the `ignoreDifferencesOnResourceUpdates` setting can be used to add all ignored differences as ignored resource updates:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
data:
  resource.compareoptions: |
    ignoreDifferencesOnResourceUpdates: true
```

## Default Configuration

By default, the metadata fields `generation`, `resourceVersion` and `managedFields` are always ignored for all resources.

## Finding Resources to Ignore

The application controller logs when a resource change triggers a refresh. You can use these logs to find
high-churn resource kinds and then inspect those resources to find which fields to ignore.

To find these logs, search for `"Requesting app refresh caused by object update"`. The logs include structured
fields for `api-version` and `kind`.  Counting the number of refreshes triggered, by api-version/kind should
reveal the high-churn resource kinds.

Note that these logs are at the `debug` level. Configure the application-controller's log level to `debug`.
