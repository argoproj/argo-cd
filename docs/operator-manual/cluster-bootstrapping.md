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
    targetRevision: HEAD
``` 

The sync policy to automated + prune, so that child apps are automatically created, synced, and deleted when the manifest is changed, but you may wish to disable this. I've also added the finalizer, which will ensure that your apps are deleted correctly.

Fix the revision to a specific Git commit SHA to make sure that, even if the child apps repo changes, the app will only change when the parent app change that revision. Alternatively, you can set it to HEAD or a branch name.

As you probably want to override the cluster server, this is a templated values.

`values.yaml` contains the default values:

```yaml
spec:
  destination:
    server: https://kubernetes.default.svc
```

Next, you need to create and sync your parent app, e.g. via the CLI:

```bash
argocd app create apps \
    --dest-namespace argocd \
    --dest-server https://kubernetes.default.svc \
    --repo https://github.com/argoproj/argocd-example-apps.git \
    --path apps  
argocd app sync apps  
```

The parent app will appear as in-sync but the child apps will be out of sync:

![New App Of Apps](../assets/new-app-of-apps.png)

You can either sync via the UI, firstly filter by the correct label:

![Filter Apps](../assets/filter-apps.png)

Then select the "out of sync" apps and sync: 

![Sync Apps](../assets/sync-apps.png)

Or, via the CLI: 

```bash
argocd app sync -l app.kubernetes.io/instance=apps
```

View [the example on Github](https://github.com/argoproj/argocd-example-apps/tree/master/apps).
