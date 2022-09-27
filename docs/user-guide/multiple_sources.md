# Multiple Sources for an Application

Argo CD has the ability to specify multiple sources to add services to the Application. Argo CD compiles all the sources and reconciles each source individually for creating the application.

You can provide multiple sources using the `sources` field. When you specify the sources field, Argo CD will ignore the values under `source` field for generating the application.

See below example for specifying multiple sources:

```
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
      helm:
        valueFiles:
          - values.yaml
      repoURL: https://helm.elastic.co
      targetRevision: 7.6.0
    - repoURL: https://github.com/argoproj/argocd-example-apps.git
      path: guestbook
      targetRevision: HEAD
```

The above example has 2 sources specified. Argo CD will reconcile each source separately and combine the resources that are generated for generating the application.

In case application has multiple entries for the same source (repoURL), Argo CD would pick the source that is mentioned later in the list of sources. For example, consider the below list of sources:

```
sources:
    - chart: elasticsearch
      helm:
        valueFiles:
          - values.yaml
      repoURL: https://helm.elastic.co
      targetRevision: 7.6.0
    - repoURL: https://github.com/argoproj/argocd-example-apps.git
      path: guestbook
      targetRevision: HEAD
    - chart: elasticsearch
      helm:
        valueFiles:
          - values.yaml
      repoURL: https://helm.elastic.co
      targetRevision: 7.7.0
```

In the above list, we have 2 sources referring to the same repoURL. In this case, Argo CD will use the source with `targetRevision: 7.7.0` as it was specified later in the list of sources and ignore the source with `targetRevision: 7.6.0`.

