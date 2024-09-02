# Controlling if/when the ApplicationSet controller modifies `Application` resources

The ApplicationSet controller supports a number of settings that limit the ability of the controller to make changes to generated Applications, for example, preventing the controller from deleting child Applications.

These settings allow you to exert control over when, and how, changes are made to your Applications, and to their corresponding cluster resources (`Deployments`, `Services`, etc).

Here are some of the controller settings that may be modified to alter the ApplicationSet controller's resource-handling behaviour.

## Dry run: prevent ApplicationSet from creating, modifying, or deleting all Applications

To prevent the ApplicationSet controller from creating, modifying, or deleting any `Application` resources, you may enable `dry-run` mode. This essentially switches the controller into a "read only" mode, where the controller Reconcile loop will run, but no resources will be modified.

To enable dry-run, add `--dryrun true` to the ApplicationSet Deployment's container launch parameters.

See 'How to modify ApplicationSet container parameters' below for detailed steps on how to add this parameter to the controller.

## Managed Applications modification Policies

The ApplicationSet controller supports a parameter `--policy`, which is specified on launch (within the controller Deployment container), and which restricts what types of modifications will be made to managed Argo CD `Application` resources.

The `--policy` parameter takes four values: `sync`, `create-only`, `create-delete`, and `create-update`. (`sync` is the default, which is used if the `--policy` parameter is not specified; the other policies are described below).

It is also possible to set this policy per ApplicationSet.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
spec:
  # (...)
  syncPolicy:
    applicationsSync: create-only # create-update, create-delete sync

```

- Policy `create-only`: Prevents ApplicationSet controller from modifying or deleting Applications. **WARNING**: It doesn't prevent Application controller from deleting Applications according to [ownerReferences](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/) when deleting ApplicationSet.
- Policy `create-update`: Prevents ApplicationSet controller from deleting Applications. Update is allowed. **WARNING**: It doesn't prevent Application controller from deleting Applications according to [ownerReferences](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/) when deleting ApplicationSet.
- Policy `create-delete`: Prevents ApplicationSet controller from modifying Applications. Delete is allowed.
- Policy `sync`: Update and Delete are allowed.

If the controller parameter `--policy` is set, it takes precedence on the field `applicationsSync`. It is possible to allow per ApplicationSet sync policy by setting variable `ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_POLICY_OVERRIDE` to argocd-cmd-params-cm `applicationsetcontroller.enable.policy.override` or directly with controller parameter `--enable-policy-override` (default to `false`).

### Policy - `create-only`: Prevent ApplicationSet controller from modifying and deleting Applications

To allow the ApplicationSet controller to *create* `Application` resources, but prevent any further modification, such as *deletion*, or modification of Application fields, add this parameter in the ApplicationSet controller:

**WARNING**: "*deletion*" indicates the case as the result of comparing generated Application between before and after, there are Applications which no longer exist. It doesn't indicate the case Applications are deleted according to ownerReferences to ApplicationSet. See [How to prevent Application controller from deleting Applications when deleting ApplicationSet](#how-to-prevent-application-controller-from-deleting-applications-when-deleting-applicationset)

```
--policy create-only
```

At ApplicationSet level

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
spec:
  # (...)
  syncPolicy:
    applicationsSync: create-only
```

### Policy - `create-update`: Prevent ApplicationSet controller from deleting Applications

To allow the ApplicationSet controller to create or modify `Application` resources, but prevent Applications from being deleted, add the following parameter to the ApplicationSet controller `Deployment`:

**WARNING**: "*deletion*" indicates the case as the result of comparing generated Application between before and after, there are Applications which no longer exist. It doesn't indicate the case Applications are deleted according to ownerReferences to ApplicationSet. See [How to prevent Application controller from deleting Applications when deleting ApplicationSet](#how-to-prevent-application-controller-from-deleting-applications-when-deleting-applicationset)

```
--policy create-update
```

This may be useful to users looking for additional protection against deletion of the Applications generated by the controller.

At ApplicationSet level

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
spec:
  # (...)
  syncPolicy:
    applicationsSync: create-update
```

### How to prevent Application controller from deleting Applications when deleting ApplicationSet

By default, `create-only` and `create-update` policy isn't effective against preventing deletion of Applications when deleting ApplicationSet.
You must set the finalizer to ApplicationSet to prevent deletion in such case, and use background cascading deletion.
If you use foreground cascading deletion, there's no guarantee to preserve applications.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  finalizers:
  - resources-finalizer.argocd.argoproj.io
spec:
  # (...)
```

## Ignore certain changes to Applications

The ApplicationSet spec includes an `ignoreApplicationDifferences` field, which allows you to specify which fields of 
the ApplicationSet should be ignored when comparing Applications.

The field supports multiple ignore rules. Each ignore rule may specify a list of either `jsonPointers` or 
`jqPathExpressions` to ignore.

You may optionally also specify a `name` to apply the ignore rule to a specific Application, or omit the `name` to apply
the ignore rule to all Applications.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
spec:
  ignoreApplicationDifferences:
    - jsonPointers:
        - /spec/source/targetRevision
    - name: some-app
      jqPathExpressions:
        - .spec.source.helm.values
```

### Allow temporarily toggling auto-sync

One of the most common use cases for ignoring differences is to allow temporarily toggling auto-sync for an Application.

For example, if you have an ApplicationSet that is configured to automatically sync Applications, you may want to temporarily
disable auto-sync for a specific Application. You can do this by adding an ignore rule for the `spec.syncPolicy.automated` field.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
spec:
  ignoreApplicationDifferences:
    - jsonPointers:
        - /spec/syncPolicy
```

### Limitations of `ignoreApplicationDifferences`

When an ApplicationSet is reconciled, the controller will compare the ApplicationSet spec with the spec of each Application
that it manages. If there are any differences, the controller will generate a patch to update the Application to match the
ApplicationSet spec.

The generated patch is a MergePatch. According to the MergePatch documentation, "existing lists will be completely 
replaced by new lists" when there is a change to the list.

This limits the effectiveness of `ignoreApplicationDifferences` when the ignored field is in a list. For example, if you
have an application with multiple sources, and you want to ignore changes to the `targetRevision` of one of the sources,
changes in other fields or in other sources will cause the entire `sources` list to be replaced, and the `targetRevision`
field will be reset to the value defined in the ApplicationSet.

For example, consider this ApplicationSet:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
spec:
  ignoreApplicationDifferences:
    - jqPathExpressions:
        - .spec.sources[] | select(.repoURL == "https://git.example.com/org/repo1").targetRevision
  template:
    spec:
      sources:
      - repoURL: https://git.example.com/org/repo1
        targetRevision: main
      - repoURL: https://git.example.com/org/repo2
        targetRevision: main
```

You can freely change the `targetRevision` of the `repo1` source, and the ApplicationSet controller will not overwrite
your change.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  sources:
  - repoURL: https://git.example.com/org/repo1
    targetRevision: fix/bug-123
  - repoURL: https://git.example.com/org/repo2
    targetRevision: main
```

However, if you change the `targetRevision` of the `repo2` source, the ApplicationSet controller will overwrite the entire
`sources` field.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  sources:
  - repoURL: https://git.example.com/org/repo1
    targetRevision: main
  - repoURL: https://git.example.com/org/repo2
    targetRevision: main
```

!!! note
    [Future improvements](https://github.com/argoproj/argo-cd/issues/15975) to the ApplicationSet controller may 
    eliminate this problem. For example, the `ref` field might be made a merge key, allowing the ApplicationSet 
    controller to generate and use a StrategicMergePatch instead of a MergePatch. You could then target a specific 
    source by `ref`, ignore changes to a field in that source, and changes to other sources would not cause the ignored 
    field to be overwritten.

## Prevent an `Application`'s child resources from being deleted, when the parent Application is deleted

By default, when an `Application` resource is deleted by the ApplicationSet controller, all of the child resources of the Application will be deleted as well (such as, all of the Application's `Deployments`, `Services`, etc).

To prevent an Application's child resources from being deleted when the parent Application is deleted, add the `preserveResourcesOnDeletion: true` field to the `syncPolicy` of the ApplicationSet:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
spec:
  # (...)
  syncPolicy:
    preserveResourcesOnDeletion: true
```

More information on the specific behaviour of `preserveResourcesOnDeletion`, and deletion in ApplicationSet controller and Argo CD in general, can be found on the [Application Deletion](Application-Deletion.md) page.


## Prevent an Application's child resources from being modified

Changes made to the ApplicationSet will propagate to the Applications managed by the ApplicationSet, and then Argo CD will propagate the Application changes to the underlying cluster resources (as per [Argo CD Integration](Argo-CD-Integration.md)).

The propagation of Application changes to the cluster is managed by the [automated sync settings](../../user-guide/auto_sync.md), which are referenced in the ApplicationSet `template` field:

- `spec.template.syncPolicy.automated`: If enabled, changes to Applications will automatically propagate to the cluster resources of the cluster. 
    - Unset this within the ApplicationSet template to 'pause' updates to cluster resources managed by the `Application` resource.
- `spec.template.syncPolicy.automated.prune`: By default, Automated sync will not delete resources when Argo CD detects the resource is no longer defined in Git.
    - For extra safety, set this to false to prevent unexpected changes to the backing Git repository from affecting cluster resources.


## How to modify ApplicationSet container launch parameters

There are a couple of ways to modify the ApplicationSet container parameters, so as to enable the above settings.

### A) Use `kubectl edit` to modify the deployment on the cluster

Edit the applicationset-controller `Deployment` resource on the cluster:
```
kubectl edit deployment/argocd-applicationset-controller -n argocd
```

Locate the `.spec.template.spec.containers[0].command` field, and add the required parameter(s):
```yaml
spec:
    # (...)
  template:
    # (...)
    spec:
      containers:
      - command:
        - entrypoint.sh
        - argocd-applicationset-controller
        # Insert new parameters here, for example:
        # --policy create-only
    # (...)
```

Save and exit the editor. Wait for a new `Pod` to start containing the updated parameters.

### Or, B) Edit the `install.yaml` manifest for the ApplicationSet installation

Rather than directly editing the cluster resource, you may instead choose to modify the installation YAML that is used to install the ApplicationSet controller:

Applicable for applicationset versions less than 0.4.0. 
```bash
# Clone the repository

git clone https://github.com/argoproj/applicationset

# Checkout the version that corresponds to the one you have installed.
git checkout "(version of applicationset)"
# example: git checkout "0.1.0"

cd applicationset/manifests

# open 'install.yaml' in a text editor, make the same modifications to Deployment 
# as described in the previous section.

# Apply the change to the cluster
kubectl apply -n argocd -f install.yaml
```

## Preserving changes made to an Applications annotations and labels

!!! note
    The same behavior can be achieved on a per-app basis using the [`ignoreApplicationDifferences`](#ignore-certain-changes-to-applications) 
    feature described above. However, preserved fields may be configured globally, a feature that is not yet available
    for `ignoreApplicationDifferences`.

It is common practice in Kubernetes to store state in annotations, operators will often make use of this. To allow for this, it is possible to configure a list of annotations that the ApplicationSet should preserve when reconciling.

For example, imagine that we have an Application created from an ApplicationSet, but a custom annotation and label has since been added (to the Application) that does not exist in the `ApplicationSet` resource:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  # This annotation and label exists only on this Application, and not in 
  # the parent ApplicationSet template:
  annotations: 
    my-custom-annotation: some-value
  labels:
    my-custom-label: some-value
spec:
  # (...)
```

To preserve this annotation and label we can use the `preservedFields` property of the `ApplicationSet` like so:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
spec:
  # (...)
  preservedFields:
    annotations: ["my-custom-annotation"]
    labels: ["my-custom-label"]
```

The ApplicationSet controller will leave this annotation and label as-is when reconciling, even though it is not defined in the metadata of the ApplicationSet itself.

By default, the Argo CD notifications and the Argo CD refresh type annotations are also preserved.

!!!note
  One can also set global preserved fields for the controller by passing a comma separated list of annotations and labels to 
  `ARGOCD_APPLICATIONSET_CONTROLLER_GLOBAL_PRESERVED_ANNOTATIONS` and `ARGOCD_APPLICATIONSET_CONTROLLER_GLOBAL_PRESERVED_LABELS` respectively.

## Debugging unexpected changes to Applications

When the ApplicationSet controller makes a change to an application, it logs the patch at the debug level. To see these
logs, set the log level to debug in the `argocd-cmd-params-cm` ConfigMap in the `argocd` namespace:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
  namespace: argocd
data:
  applicationsetcontroller.log.level: debug
```

## Previewing changes

To preview changes that the ApplicationSet controller would make to Applications, you can create the AppSet in dry-run 
mode. This works whether the AppSet already exists or not.

```shell
argocd appset create --dry-run ./appset.yaml -o json | jq -r '.status.resources[].name'
```

The dry-run will populate the returned ApplicationSet's status with the Applications which would be managed with the 
given config. You can compare to the existing Applications to see what would change.
