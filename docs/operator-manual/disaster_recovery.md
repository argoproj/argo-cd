# Disaster Recovery

You can use `argocd admin` to import and export all Argo CD data.

Make sure you have `~/.kube/config` pointing to your Argo CD cluster.

Figure out what version of Argo CD you're running:

```bash
argocd version | grep server
# ...
export VERSION=v1.0.1
```

Export to a backup:

```bash
docker run -v ~/.kube:/home/argocd/.kube --rm quay.io/argoproj/argocd:$VERSION argocd admin export > backup.yaml
```

Import from a backup:

```bash
docker run -i -v ~/.kube:/home/argocd/.kube --rm quay.io/argoproj/argocd:$VERSION argocd admin import - < backup.yaml
```

!!! note
    If you are running Argo CD on a namespace different than default, remember to add the namespace parameter (-n <namespace>) so that the 'argocd admin export' command can pass, as it looks for the resources in the default namespace (`argocd`).
