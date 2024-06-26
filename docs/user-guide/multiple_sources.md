# Multiple Sources for an Application

!!! warning "Beta Feature"
    Specifying multiple sources for an application is a beta feature. The UI and CLI still generally behave as if only
    the first source is specified. Full UI/CLI support will be added in a future release.
    This feature is subject to change in backwards incompatible ways until it is marked stable.

Argo CD has the ability to specify multiple sources for a single Application. Argo CD compiles all the sources
and reconciles the combined resources.

You can provide multiple sources using the `sources` field. When you specify the `sources` field, Argo CD will ignore 
the `source` (singular) field.

See the below example for specifying multiple sources:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
spec:
  project: default
  destination:
    server: https://kubernetes.default.svc
    namespace: default
  sources:
    - chart: elasticsearch
      repoURL: https://helm.elastic.co
      targetRevision: 8.5.1
    - repoURL: https://github.com/argoproj/argocd-example-apps.git
      path: guestbook
      targetRevision: HEAD
```

The above example has two sources specified. Argo CD will generate the manifests for each source separately and combine 
the resulting manifests.

If multiple sources produce the same resource (same `group`, `kind`, `name`, and `namespace`), the last source to 
produce the resource will take precedence. Argo CD will produce a `RepeatedResourceWarning` in this case, but it will 
sync the resources. This provides a convenient way to override a resource from a chart with a resource from a Git repo.

## Helm value files from external Git repository

Helm sources can reference value files from git sources. This allows you to use a third-party Helm chart with custom,
git-hosted values.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  sources:
  - repoURL: 'https://prometheus-community.github.io/helm-charts'
    chart: prometheus
    targetRevision: 15.7.1
    helm:
      valueFiles:
      - $values/charts/prometheus/values.yaml
  - repoURL: 'https://git.example.com/org/value-files.git'
    targetRevision: dev
    ref: values
```

In the above example, the `prometheus` chart will use the value file from `git.example.gom/org/value-files.git`. 
`$values` resolves to the root of the `value-files` repository. The `$values` variable may only be specified at the 
beginning of the value file path.

If the `path` field is set in the `$values` source, Argo CD will attempt to generate resources from the git repository
at that URL. If the `path` field is not set, Argo CD will use the repository solely as a source of value files.

!!! note
    Sources with the `ref` field set must not also specify the `chart` field. Argo CD does not currently support using  
    another Helm chart as a source for value files.
