# Sync Applications with Kubectl

You can use "kubectl" to ask Argo CD to synchronize applications the same way you can use the CLI or UI. Many configurations like "force", "prune", "apply" and even synchronize a specific list of resources are equally supported. This is done by applying or patching the Argo CD application with a document that defines an "operation".

This "operation" defines how a synchronization should be done and for what resources these synchronization is to be done.

There are many configuration options that can be added to the "operation". Next, a few of them are explained. For more details, you can have a look at the CRD [applications.argoproj.io](https://github.com/argoproj/argo-cd/blob/master/manifests/crds/application-crd.yaml). Some of them are required, whereas others are optional.

To ask Argo CD to synchronize all resources of a given application, we can do:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: <app-name>
  namespace: <namespace>
spec:
  ...
operation:
  initiatedBy:
    username: <username>
  sync:
    syncStrategy:
      hook: {}
```

```bash
$ kubectl apply -f <apply-file>
```

The most important part is the "sync" definition in the "operation" field. You can pass optional information like "info" or "initiatedBy". "info" allows you to add information about the operation in the form of a list. "initiatedBy" contains information about who initiated the operation request.

Or if you prefer, you also can patch:

```yaml
operation:
  initiatedBy:
    username: <username>
  sync:
    syncStrategy:
      hook: {}
```

```bash
$ kubectl patch -n <namespace> app <app-name> --patch-file <patch-file> --type merge
```

Be aware that patches, specially with merge strategies, may not work the way you expect especially if you change sync strategies or options.
In these cases, "kubectl apply" gives better results.

Either with a "kubectl patch" or "kubectl apply", the state of the synchronization is reported in the "operationState" field in the application object.

```bash
$ kubectl get -n <namespace> get app <app-name> -o yaml
...
status:
  operationState:
    finishedAt: "2023-08-03T11:16:17Z"
    message: successfully synced (all tasks run)
    phase: Succeeded
```

# Apply and Hook synchronization strategies

There are two types of synchronization strategies: "hook", which is the default value, and "apply".

An "apply" sync strategy tells Argo CD to "kubectl apply", whereas a "hook" sync strategy informs Argo CD to submit any resource that's referenced in the operation. This way the synchronization of these resources will take into consideration any hook the resource has been annotated with.

```yaml
operation:
  sync:
    syncStrategy:
      apply: {}
```

```yaml
operation:
  sync:
    syncStrategy:
      hook: {}
```

Both strategies support "force". However, you need to be aware that a force operation deletes the resource when patch encounters a conflict after having retried 5 times.

```yaml
operation:
  sync:
    syncStrategy:
      apply:
        force: true
```

```yaml
operation:
  sync:
    syncStrategy:
      hook:
        force: true
```

# Prune

If you want to prune your resources before applying, you can instruct Argo CD to do so:

```yaml
operation:
  sync:
    prune: true
```

# List of resources

There's always the possibility to pass a list of resources. This list can be all resources the application manages or only a subset, for example resources that remained out of sync for some reason.

Only "kind" and "name" are required fields when referencing resources, but the fields "groups" and "namespace" can also be defined:

```yaml
operation:
  sync:
    resources:
      - kind: Namespace
        name: namespace-name
      - kind: ServiceAccount
        name: service-account-name
        namespace: namespace-name
      - group: networking.k8s.io
        kind: NetworkPolicy
        name: network-policy-name
        namespace: namespace-name
```

# Sync Options

In an operation, you can also pass sync-options. Each of these options is passed as "name=value" pairs. For example:

```yaml
operations:
  sync:
    syncOptions:
      - Validate=false
      - Prune=false
```

For more information about sync options, please refer to [sync-options](https://argo-cd.readthedocs.io/en/stable/user-guide/sync-options/)
