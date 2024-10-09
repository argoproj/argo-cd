# Reconcile Optimization

By default, an Argo CD Application is refreshed every time a resource that belongs to it changes.

Kubernetes controllers often update the resources they watch periodically, causing continuous reconcile operation on the Application
and a high CPU usage on the `argocd-application-controller`. Argo CD allows you to optionally ignore resource updates on specific fields
for [tracked resources](../user-guide/resource_tracking.md). 
For untracked resources, you can [use the argocd.argoproj.io/ignore-resource-updates annotations](#ignoring-updates-for-untracked-resources)

When a resource update is ignored, if the resource's [health status](./health.md) does not change, the Application that this resource belongs to will not be reconciled.

## System-Level Configuration

Argo CD allows ignoring resource updates at a specific JSON path, using [RFC6902 JSON patches](https://tools.ietf.org/html/rfc6902) and [JQ path expressions](https://stedolan.github.io/jq/manual/#path(path_expression)). It can be configured for a specified group and kind
in `resource.customizations` key of the `argocd-cm` ConfigMap.

!!!important "Enabling the feature"
    The feature is behind a flag. To enable it, set `resource.ignoreResourceUpdatesEnabled` to `"true"` in the `argocd-cm` ConfigMap.

Following is an example of a customization which ignores the `refreshTime` status field of an [`ExternalSecret`](https://external-secrets.io/main/api/externalsecret/) resource:

```yaml
data:
  resource.customizations.ignoreResourceUpdates.external-secrets.io_ExternalSecret: |
    jsonPointers:
    - /status/refreshTime
    # JQ equivalent of the above:
    # jqPathExpressions:
    # - .status.refreshTime
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

!!!note 
    These logs are at the `debug` level. Configure the application-controller's log level to `debug`.

Once you have identified some resources which change often, you can try to determine which fields are changing. Here is
one approach:

```shell
kubectl get <resource> -o yaml > /tmp/before.yaml
# Wait a minute or two.
kubectl get <resource> -o yaml > /tmp/after.yaml
diff /tmp/before.yaml /tmp/after
```

The diff can give you a sense for which fields are changing and should perhaps be ignored.

## Checking Whether Resource Updates are Ignored

Whenever Argo CD skips a refresh due to an ignored resource update, the controller logs the following line:
"Ignoring change of object because none of the watched resource fields have changed".

Search the application-controller logs for this line to confirm that your resource ignore rules are being applied.

!!!note
    These logs are at the `debug` level. Configure the application-controller's log level to `debug`.

## Examples

### argoproj.io/Application

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
data:
  resource.customizations.ignoreResourceUpdates.argoproj.io_Application: |
    jsonPointers:
    # Ignore when ownerReferences change, for example when a parent ApplicationSet changes often.
    - /metadata/ownerReferences
    # Ignore reconciledAt, since by itself it doesn't indicate any important change.
    - /status/reconciledAt
    jqPathExpressions:
    # Ignore lastTransitionTime for conditions; helpful when SharedResourceWarnings are being regularly updated but not
    # actually changing in content.
    - .status?.conditions[]?.lastTransitionTime
```

## Ignoring updates for untracked resources

ArgoCD will only apply `ignoreResourceUpdates` configuration to tracked resources of an application. This means dependant resources, such as a `ReplicaSet` and `Pod` created by a `Deployment`, will not ignore any updates and trigger a reconcile of the application for any changes.

If you want to apply the `ignoreResourceUpdates` configuration to an untracked resource, you can add the
`argocd.argoproj.io/ignore-resource-updates=true` annotation in the dependent resources manifest.

## Example

### CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: hello
  namespace: test-cronjob
spec:
  schedule: "* * * * *"
  jobTemplate:
    metadata:
      annotations:
        argocd.argoproj.io/ignore-resource-updates: "true"
    spec:
      template:
        metadata:
          annotations:
            argocd.argoproj.io/ignore-resource-updates: "true"
        spec:
          containers:
          - name: hello
            image: busybox:1.28
            imagePullPolicy: IfNotPresent
            command:
            - /bin/sh
            - -c
            - date; echo Hello from the Kubernetes cluster
          restartPolicy: OnFailure
```

The resource updates will be ignored based on your the `ignoreResourceUpdates` configuration in the `argocd-cm` configMap:

`argocd-cm`:
```yaml
resource.customizations.ignoreResourceUpdates.batch_Job: |
    jsonPointers:
      - /status
resource.customizations.ignoreResourceUpdates.Pod: |
    jsonPointers:
      - /status      
```
