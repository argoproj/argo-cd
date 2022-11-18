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

# Deletion Using `kubectl`

To perform a non-cascade delete:

```bash
kubectl delete app APPNAME
```

To perform a cascade delete set the finalizer, e.g. using `kubectl patch`:

```bash
kubectl patch app APPNAME  -p '{"metadata": {"finalizers": ["resources-finalizer.argocd.argoproj.io"]}}' --type merge
kubectl delete app APPNAME
```

# About The Deletion Finalizer

For the technical amongst you, the Argo CD application controller watches for this finalizer:

```yaml
metadata:
  finalizers:
    - resources-finalizer.argocd.argoproj.io
```

Argo CD's app controller watches for this and will then delete both the app and its resources.

When you invoke `argocd app delete` with `--cascade`, the finalizer is added automatically.
