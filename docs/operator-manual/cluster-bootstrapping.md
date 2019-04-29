# Cluster Bootstrapping

This guide for operators who have already installed Argo CD, and have a new cluster and are looking to install many applications in that cluster.

There's no one particular pattern to solve this problem, e.g. you could write a script to create your applications, or you could even manually create them. However, users of Argo CD tend to use the **application of applications pattern**.

## Application Of Applications Pattern

[Declaratively](declarative-setup.md) specify one Argo CD application that consists only of other applications.

![Application of Applications](../assets/application-of-applications.png)

### Helm Example

This example shows how to use Helm to achieve this. You can, of course, use another tool if you like.

A typical layout of your Git repository for this might be:

```
├── Chart.yaml
├── templates
│   ├── guestbook.yaml
│   ├── helm-dependency.yaml
│   ├── helm-guestbook.yaml
│   └── kustomize-guestbook.yaml
└── values.yaml
```

`Chart.yaml` is boiler-plate.

`templates` contains one file for each application, roughly:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
  finalizers:
  - resources-finalizer.argocd.argoproj.io
spec:
  destination:
    namespace: argocd
    server: {{ .Values.spec.destination.server }}
  project: default
  source:
    path: guestbook
    repoURL: https://github.com/argoproj/argocd-example-apps
    targetRevision: {{ .Values.spec.source.targetRevision }}
  syncPolicy:
    automated:
      prune: true
``` 

In this example, I've set the sync policy to automated + prune, so that applications are automatically created, synced, and deleted when the manifest is changed, but you may wish to disable this. I've also added the finalizer, which will ensure that you applications are deleted correctly.

As you probably want to override the cluster server and maybe the revision, these are templated values.

`values.yaml` contains the default values:

```yaml
spec:
  destination:
    server: https://kubernetes.default.svc
  source:
    targetRevision: HEAD
```

Finally, you need to create your application, e.g.:

```bash
argocd app create applications \
    --dest-namespace argocd \
    --dest-server https://kubernetes.default.svc \
    --repo https://github.com/argoproj/argocd-example-apps.git \
    --path applications \
    --sync-policy automated 
```

In this example, I excluded auto-prune, as this would result in all applications being deleted if some accidentally deleted the *application of applications*.

View [the example on Github](https://github.com/argoproj/argocd-example-apps/tree/master/applications).