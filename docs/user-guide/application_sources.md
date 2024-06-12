# Tools

## Production

Argo CD supports several different ways in which Kubernetes manifests can be defined:

* [Kustomize](kustomize.md) applications
* [Helm](helm.md) charts
* A directory of YAML/JSON/Jsonnet manifests, including [Jsonnet](jsonnet.md).
* Any [custom config management tool](../operator-manual/config-management-plugins.md) configured as a config management plugin

Argo CD also supports the "rendered manifest" pattern, i.e. pushing the hydrated manifests to git before syncing them to 
the cluster. See the [source hydrator](source-hydrator.md) page for more information.

## Development
Argo CD also supports uploading local manifests directly. Since this is an anti-pattern of the
GitOps paradigm, this should only be done for development purposes. A user with an `override` permission is required
to upload manifests locally (typically an admin). All of the different Kubernetes deployment tools above are supported.
To upload a local application:

```bash
$ argocd app sync APPNAME --local /path/to/dir/
```
