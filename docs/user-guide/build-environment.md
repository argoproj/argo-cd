# Build Environment

[Custom tools](config-management-plugins.md), [Helm](helm.md), [Jsonnet](jsonnet.md), and [Kustomize](kustomize.md) support the following build env vars:

* `ARGOCD_APP_NAME` - name of application
* `ARGOCD_APP_NAMESPACE` - destination application namespace.
* `ARGOCD_APP_REVISION` - the resolved revision, e.g. `f913b6cbf58aa5ae5ca1f8a2b149477aebcbd9d8`
* `ARGOCD_APP_SOURCE_PATH` - the path of the app within the repo
* `ARGOCD_APP_SOURCE_REPO_URL` the repo's URL
* `ARGOCD_APP_SOURCE_TARGET_REVISION` - the target revision from the spec, e.g. `master`.
* `KUBE_VERSION` - the version of kubernetes
* `KUBE_API_VERSIONS` = the version of kubernetes API

In case you don't want a variable to be interpolated, `$` can be escaped via `$$`.

```
command:
  - sh
  - -c
  - |
    echo $$FOO
```
