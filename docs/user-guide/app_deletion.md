# App Deletion

## Cascade App Deletion

Apps can be deleted with or without a cascade option. Cascade deletion deletes the both app's and its resources, rather than only the app. 


```bash
argocd app delete APPNAME --cascade
```

For the technical amongst you, this adds the following Kubernetes finalizer to the app's manifest: 


```yaml
metadata:
  finalizers:
    - resources-finalizer.argocd.argoproj.io
```

Argo CD's app controller notices this and will then delete both the app and its resources.

!!! warning

    Don't delete apps using `kubetctl delete app APPNAME`. You cannot perform a cascade deletion using `kubectl`.
    