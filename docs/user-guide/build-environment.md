# Build Environment

[Custom tools](../operator-manual/config-management-plugins.md), [Helm](helm.md), [Jsonnet](jsonnet.md), and [Kustomize](kustomize.md) support the following build env vars:

| Variable                            | Description                                                             |
|-------------------------------------|-------------------------------------------------------------------------|
| `ARGOCD_APP_NAME`                   | The name of the application.                                            |
| `ARGOCD_APP_NAMESPACE`              | The destination namespace of the application.                           |
| `ARGOCD_APP_REVISION`               | The resolved revision, e.g. `f913b6cbf58aa5ae5ca1f8a2b149477aebcbd9d8`. |
| `ARGOCD_APP_REVISION_SHORT`         | The resolved short revision, e.g. `f913b6c`.                            |
| `ARGOCD_APP_REVISION_SHORT_8`       | The resolved short revision with length 8, e.g. `f913b6cb`.             |
| `ARGOCD_APP_SOURCE_PATH`            | The path of the app within the source repo.                             |
| `ARGOCD_APP_SOURCE_REPO_URL`        | The source repo URL.                                                    |
| `ARGOCD_APP_SOURCE_TARGET_REVISION` | The target revision from the spec, e.g. `master`.                       |
| `KUBE_VERSION`                      | The version of Kubernetes.                                              |
| `KUBE_API_VERSIONS`                 | The version of the Kubernetes API.                                      |

In case you don't want a variable to be interpolated, `$` can be escaped via `$$`.

```
command:
  - sh
  - -c
  - |
    echo $$FOO
```
