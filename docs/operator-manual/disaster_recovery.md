# Disaster Recovery

You can use `argocd-util` can be used to import and export all Argo CD data.

Make sure you have `~/.kube/config` pointing to your Argo CD cluster.

Export to a backup:

```bash
docker run -v ~/.kube:/home/argocd/.kube --rm argoproj/argocd:v1.0.1 argocd-util export > backup.yaml
```

Import from a backup:

```bash
docker run -v ~/.kube:/home/argocd/.kube --rm argoproj/argocd:v1.0.1 argocd-util import - < backup.yaml
```
