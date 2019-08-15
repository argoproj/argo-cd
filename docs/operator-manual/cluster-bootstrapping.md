# Cluster Bootstrapping

This guide for operators who have already installed Argo CD, and have a new cluster and are looking to install many apps in that cluster.

There's no one particular pattern to solve this problem, e.g. you could write a script to create your apps, or you could even manually create them. However, users of Argo CD tend to use the **app of apps pattern**.

## App Of Apps Pattern

[Declaratively](declarative-setup.md) specify one Argo CD app that consists only of other apps.

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

`templates` contains one file for each child app, roughly:

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
    targetRevision: 08836bd970037ebcd14494831de4635bad961139
  syncPolicy:
    automated:
      prune: true
``` 

The sync policy to automated + prune, so that child app is are automatically created, synced, and deleted when the manifest is changed, but you may wish to disable this. I've also added the finalizer, which will ensure that you apps are deleted correctly.

Fix the revision to a specific Git commit SHA to make sure that, even if the child apps repo changes, the app will only change when the parent app change that revision. Alternatively, you can set it to HEAD or a branch name.

As you probably want to override the cluster server, this is a templated values.

`values.yaml` contains the default values:

```yaml
spec:
  destination:
    server: https://kubernetes.default.svc
```

Finally, you need to create your parent app, e.g.:

```bash
argocd app create applications \
    --dest-namespace argocd \
    --dest-server https://kubernetes.default.svc \
    --repo https://github.com/argoproj/argocd-example-apps.git \
    --path apps \
    --sync-policy automated 
```

In this example, I excluded auto-prune, as this would result in all apps being deleted if some accidentally deleted the *app of apps*.

View [the example on Github](https://github.com/argoproj/argocd-example-apps/tree/master/apps).
