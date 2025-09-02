# Build Environment

[Custom tools](../operator-manual/config-management-plugins.md), [Helm](helm.md), [Jsonnet](jsonnet.md), and [Kustomize](kustomize.md) support the following build env vars:

| Variable                            | Description                                                               |
| ----------------------------------- | ------------------------------------------------------------------------- |
| `ARGOCD_APP_NAME`                   | The name of the application.                                              |
| `ARGOCD_APP_NAMESPACE`              | The destination namespace of the application.                             |
| `ARGOCD_APP_PROJECT_NAME`           | The name of the project the application belongs to.                       |
| `ARGOCD_APP_REVISION`               | The resolved revision, e.g. `f913b6cbf58aa5ae5ca1f8a2b149477aebcbd9d8`.   |
| `ARGOCD_APP_REVISION_SHORT`         | The resolved short revision, e.g. `f913b6c`.                              |
| `ARGOCD_APP_REVISION_SHORT_8`       | The resolved short revision with length 8, e.g. `f913b6cb`.               |
| `ARGOCD_APP_SOURCE_PATH`            | The path of the app within the source repo.                               |
| `ARGOCD_APP_SOURCE_REPO_URL`        | The source repo URL.                                                      |
| `ARGOCD_APP_SOURCE_TARGET_REVISION` | The target revision from the spec, e.g. `master`.                         |
| `KUBE_VERSION`                      | The semantic version of Kubernetes without trailing metadata.             |
| `KUBE_API_VERSIONS`                 | The version of the Kubernetes API.                                        |

## Revision Resolution Metadata

The following build environment variables provide metadata about how the target revision was resolved. These are available when revision resolution metadata is present:

| Variable                     | Description                                                                                                                      |
| ---------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `ARGOCD_ORIGINAL_REVISION`   | The original revision string provided in the application spec, e.g. `v1.0.*`, `HEAD`, `master`, `^1.0.0`                       |
| `ARGOCD_RESOLUTION_TYPE`     | How the revision was resolved. One of: `direct`, `range`, `symbolic_reference`, `truncated_commit_sha`, `version`               |
| `ARGOCD_RESOLVED_TAG`        | The actual resolved revision/tag. For ranges, this is the matched version. For direct resolutions, this may be the commit SHA   |
| `ARGOCD_RESOLVED_TO`         | The target of symbolic reference resolution (Git only), e.g. `refs/heads/main` when resolving `HEAD`                           |

### Resolution Types

- **`direct`**: Exact match (e.g., specific commit SHA, exact version, branch name)
- **`range`**: Resolved from a semantic version constraint (e.g., `^1.0.0` → `1.2.3`)  
- **`symbolic_reference`**: Resolved from symbolic reference like `HEAD` (Git only)
- **`truncated_commit_sha`**: Assumed to be a truncated commit SHA (Git only)
- **`version`**: Resolved as a specific version (OCI/Helm)

In case you don't want a variable to be interpolated, `$` can be escaped via `$$`.

```
command:
  - sh
  - -c
  - |
    echo $$FOO
```
