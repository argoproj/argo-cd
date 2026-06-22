# Resources View

The Resources view (available from the **Resources** item in the left navigation, served at `/resources`) lists all
Kubernetes resources managed by your Argo CD applications in a single, filterable table. It aggregates the
resources of every application you can access, so you can search and inspect managed resources across
applications without opening each application individually.

## Disabling the Resources view

Operators can disable the Resources view for an entire Argo CD instance. This is useful when the aggregated view is
not desired, for example on instances managing a very large number of resources.

To disable it, set `ui.view.resources.disabled` to `"true"` in the `argocd-cm` ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
  labels:
    app.kubernetes.io/name: argocd-cm
    app.kubernetes.io/part-of: argocd
data:
  ui.view.resources.disabled: 'true'
```

When disabled:

- The **Resources** item is hidden from the left navigation.
- Navigating directly to `/resources` shows a message indicating the feature is not enabled on the instance.
