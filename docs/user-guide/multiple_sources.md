# Multiple Sources for an Application

Argo CD has the ability to specify multiple sources to add services to the Application. Argo CD compiles all the sources
and reconciles each source individually for creating the application.

You can provide multiple sources using the `sources` field. When you specify the `sources` field, Argo CD will ignore 
the `source` (singular) field when generating manifests for the application.

See the below example for specifying multiple sources:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
  labels:
    argocd.argoproj.io/refresh: hard
spec:
  project: default
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
  destination:
    server: https://kubernetes.default.svc
    namespace: argocd
  sources:
    - chart: elasticsearch
      repoURL: https://helm.elastic.co
      targetRevision: 7.6.0
    - repoURL: https://github.com/argoproj/argocd-example-apps.git
      path: guestbook
      targetRevision: HEAD
```

The above example has two sources specified. Argo CD will generate the manifests for each source separately and combine 
the resulting manifests.

In case an application has multiple entries for the same source (repoURL), Argo CD will pick the source that is 
mentioned later in the list of sources. For example, consider the below list of sources:

```yaml
sources:
- chart: elasticsearch
  repoURL: https://helm.elastic.co
  targetRevision: 7.6.0
- repoURL: https://github.com/argoproj/argocd-example-apps.git
  path: guestbook
  targetRevision: HEAD
- chart: elasticsearch
  repoURL: https://helm.elastic.co
  targetRevision: 7.7.0
```

In the above list, the application has two sources referring to the same repoURL. In this case, Argo CD will generate 
the manifests for source with `targetRevision: 7.6.0` and then append the manifests generated for source with 
`targetRevision: 7.7.0`. 

## Helm Value files from external Git repository

Users can now provide provide value files to the helm repositories from external sources. See below example ApplicationSpec for the same,

```yaml
spec:
  project: default
  sources:
  - repoURL: 'https://prometheus-community.github.io/helm-charts'
    chart: prometheus
    targetRevision: 15.6.0
    ref: prometheus
  - repoURL: 'https://prometheus-community.github.io/helm-charts'
    chart: prometheus
    targetRevision: 15.7.1
    helm:
        valueFiles:
        - $prometheus/charts/prometheus/values.yaml
  destination:
    server: 'https://kubernetes.default.svc'
    namespace: argocd
```

In the above example, the source with `targetRevision 15.7.1` will use the value files from source with `targetRevision 15.6.0` with the help of ref `$prometheus`.

Note: The source with `ref` field needs to be mentioned before referencing it in another source. For example, source with `ref: prometheus` needs to come before we use `$prometheus` in another source.
