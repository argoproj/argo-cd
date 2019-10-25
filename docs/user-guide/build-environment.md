# Build Environment

[Custom tools](config-management-plugins.md), [Helm](helm.md), and [Jsonnet](jsonnet.md) support the following build env vars:

* `ARGOCD_APP_NAME` - name of application
* `ARGOCD_APP_NAMESPACE` - destination application namespace.
* `ARGOCD_APP_REVISION` - the resolved revision, e.g. `f913b6cbf58aa5ae5ca1f8a2b149477aebcbd9d8`
* `ARGOCD_APP_SOURCE_PATH` - the path of the app within the repo
* `ARGOCD_APP_SOURCE_REPO_URL` the repo's URL
* `ARGOCD_APP_SOURCE_TARGET_REVISION` - the target revision from the spec, e.g. `master`.