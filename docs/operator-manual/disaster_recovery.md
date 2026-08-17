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

> [!NOTE]
> If you are running Argo CD on a namespace different than default remember to pass the namespace parameter (-n <namespace>). 'argocd admin export' will not fail if you run it in the wrong namespace.

## Exporting a clean, re-creatable manifest set

By default, `argocd admin export` includes the `status` field of exported resources (e.g. `Application` sync history and health). This is useful for warm-standby/disaster-recovery setups where you want the restored resources to retain their last known status.

If instead you want a clean backup that can be applied directly with `kubectl apply` or re-created from scratch (for example, to seed a new cluster or store manifests in git), pass `--strip-status` to omit the `status` field from the export:

```bash
argocd admin export --strip-status > backup.yaml
kubectl apply -f backup.yaml
```

> [!NOTE]
> `--strip-status` only affects the export. `argocd admin import` already leaves `status` untouched on create, and only overwrites it on update when the backup manifest actually has a `status` field, so importing a `--strip-status` backup will not clear the `status` of existing live resources.
