# Multiple Sources for an Application

By default an Argo CD application is a link between a single source and a cluster. Sometimes however, you want to combine
files from multiple locations to form a single Application.

Argo CD has the ability to specify multiple sources for a single Application. Argo CD compiles all the sources
and reconciles the combined resources.

You can provide multiple sources using the `sources` field. When you specify the `sources` field, Argo CD will ignore 
the `source` (singular) field.

See the below example for specifying multiple sources:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-billing-app
  namespace: argocd
spec:
  project: default
  destination:
    server: https://kubernetes.default.svc
    namespace: default
  sources:
    - repoURL: https://github.com/mycompany/billing-app.git
      path: manifests
      targetRevision: 8.5.1
    - repoURL: https://github.com/mycompany/common-settings.git
      path: configmaps-billing
      targetRevision: HEAD
```

The above example has two sources specified that need to be combined in order to create the "billing" application. Argo CD will generate the manifests for each source separately and combine 
the resulting manifests.

!!! warning "Do not abuse multiple sources"
    Note this feature is **NOT** destined as a generic way to group different/unrelated applications. Take a look at [applicationsets](../user-guide/application-set.md) and the [app-of-apps](../../operator-manual/cluster-bootstrapping/) pattern if you want to have a single entity for multiple applications. If you find yourself using more than 2-3 items in the `sources` array then you are almost certainly abusing this feature and you need to rethink your application grouping strategy.

If multiple sources produce the same resource (same `group`, `kind`, `name`, and `namespace`), the last source to 
produce the resource will take precedence. Argo CD will produce a `RepeatedResourceWarning` in this case, but it will 
sync the resources. This provides a convenient way to override a resource from a chart with a resource from a Git repo.

## Helm value files from external Git repository

One of the most common scenarios for using multiple sources is the following

1. Your organization wants to use an external/public Helm chart
1. You want to override the Helm values with your own local values
1. You don't want to clone the Helm chart locally as well because that would lead to duplication and you would need to monitor it manually for upstream changes.

In this scenario you can use the multiple sources features to combine the external chart with your own local values.

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

!!! note
    Even when the `ref` field is configured with the `path` field, `$value` still represents the root of sources with the `ref` field. Consequently, `valueFiles` must be specified as relative paths from the root of sources.
