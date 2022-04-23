# Sync Options

ArgoCD allows users to customize some aspects of how it syncs the desired state in the target cluster. Some Sync Options can defined as annotations in a specific resource. Most of the Sync Options are configured in the Application resource `spec.syncPolicy.syncOptions` attribute.

Bellow you can find details about each available Sync Option:

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

The app will be out of sync if ArgoCD expects a resource to be pruned. You may wish to use this along with [compare options](compare-options.md).

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
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
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
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  syncPolicy:
    syncOptions:
    - PrunePropagationPolicy=foreground
```

## Prune Last

This feature is to allow the ability for resource pruning to happen as a final, implicit wave of a sync operation, 
after the other resources have been deployed and become healthy, and after all other waves completed successfully. 

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  syncPolicy:
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

By default, ArgoCD executes `kubectl apply` operation to apply the configuration stored in Git. In some cases
`kubectl apply` is not suitable. For example, resource spec might be too big and won't fit into
`kubectl.kubernetes.io/last-applied-configuration` annotation that is added by `kubectl apply`. In such cases you
might use `Replace=true` sync option:


```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  syncPolicy:
    syncOptions:
    - Replace=true
```

If the `Replace=true` sync option is set the ArgoCD will use `kubectl replace` or `kubectl create` command to apply changes.

This can also be configured at individual resource level.
```yaml
metadata:
  annotations:
    argocd.argoproj.io/sync-options: Replace=true
```

## Fail the sync if a shared resource is found

By default, ArgoCD will apply all manifests found in the git path configured in the Application regardless if the resources defined in the yamls are already applied by another Application. If the `FailOnSharedResource` sync option is set, ArgoCD will fail the sync whenever it finds a resource in the current Application that is already applied in the cluster by another Application.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  syncPolicy:
    syncOptions:
    - FailOnSharedResource=true
```

## Respect ignore difference configs

This sync option is used to enable ArgoCD to consider the configurations made in the `spec.ignoreDifferences` attribute also during the sync stage. By default, ArgoCD uses the `ignoreDifferences` config just for computing the diff between the live and desired state which defines if the application is synced or not. However during the sync stage, the desired state is applied as-is. The patch is calculated using a 3-way-merge between the live state the desired state and the `last-applied-configuration` annotation. This sometimes leads to an undesired results. This behavior can be changed by setting the `RespectIgnoreDifferences=true` sync option like in the example bellow:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:

  ignoreDifferences:
  - group: "apps"
    kind: "Deployment"
    jsonPointers:
    - /spec/replicas

  syncPolicy:
    syncOptions:
    - RespectIgnoreDifferences=true
```

The example above shows how an ArgoCD Application can be configured so it will ignore the `spec.replicas` field from the desired state (git) during the sync stage. This is achieve by calculating and pre-patching the desired state before applying it in the cluster. Note that the `RespectIgnoreDifferences` sync option is only effective when the resource is already created in the cluster. If the Application is being created and no live state exists, the desired state is applied as-is.
