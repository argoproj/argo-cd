# Overview

Argo CD Notifications continuously monitors Argo CD applications and provides a flexible way to notify
users about important changes in the application state. Using a flexible mechanism of
[triggers](./notifications/triggers.md) and [templates](./notifications/templates.md) you can configure when the notification should be sent as
well as notification content. Argo CD Notifications includes the [catalog](./notifications/catalog.md) of useful triggers and templates.
So you can just use them instead of reinventing new ones.

## Getting Started

* Install Argo CD Notifications

```
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj-labs/argocd-notifications/v1.1.0/manifests/install.yaml
```

* Install Triggers and Templates from the catalog

```
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj-labs/argocd-notifications/v1.1.0/catalog/install.yaml
```

* Add Email username and password token to `argocd-notifications-secret` secret

```bash
export EMAIL_USER=<your-username>
export PASSWORD=<your-password>
kubectl apply -n argocd -f - << EOF
apiVersion: v1
kind: Secret
metadata:
  name: argocd-notifications-secret
stringData:
  email-username: $EMAIL_USER
  email-password: $PASSWORD
type: Opaque
EOF
```

* Register Email notification service

```bash
kubectl patch cm argocd-notifications-cm -n argocd --type merge -p '{"data": {"service.email.gmail": "{ username: $email-username, password: $email-password, host: smtp.gmail.com, port: 465, from: $email-username }" }}'
```

* Subscribe to notifications by adding the `notifications.argoproj.io/subscribe.on-sync-succeeded.slack` annotation to the Argo CD application or project:

```bash
kubectl patch app <my-app> -n argocd -p '{"metadata": {"annotations": {"notifications.argoproj.io/subscribe.on-sync-succeeded.slack":"<my-channel>"}}}' --type merge
```

Try syncing and application and get the notification once sync is completed.

## Kustomize Getting Started

The argocd-notification manifests can also be installed using [Kustomize](https://kustomize.io/). To install
argocd-notifications, we recommend using the remote kustomize resource:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: argocd
resources:
- https://raw.githubusercontent.com/argoproj-labs/argocd-notifications/stable/manifests/install.yaml

patchesStrategicMerge:
- https://raw.githubusercontent.com/argoproj-labs/argocd-notifications/stable/catalog/install.yaml
```

## Helm v3 Getting Started

argocd-notifications is now on [Helm Hub](https://hub.helm.sh/charts/argo/argocd-notifications) as a Helm v3 chart, making it even easier to get started as
installing and configuring happen together:

```shell
helm repo add argo https://argoproj.github.io/argo-helm
helm install argo/argocd-notifications --generate-name -n argocd -f values.yaml
```

```yaml
argocdUrl: https://argocd.example.com

notifiers:
  service.email.gmail: |
    username: $email-username
    password: $email-password
    host: smtp.gmail.com
    port: 465
    from: $email-username

secret:
  items:
    email-username: <your-username>
    email-password: <your-password>

templates:
  template.app-deployed: |
    email:
      subject: New version of an application {{.app.metadata.name}} is up and running.
    message: |
      {{if eq .serviceType "slack"}}:white_check_mark:{{end}} Application {{.app.metadata.name}} is now running new version of deployments manifests.
triggers:
  trigger.on-deployed: |
    - description: Application is synced and healthy. Triggered once per commit.
      oncePer: app.status.operationState.syncResult.revision
      send:
      - app-deployed
      when: app.status.operationState.phase in ['Succeeded'] and app.status.health.status == 'Healthy'
```

For more information or to contribute, check out the [argoproj/argo-helm repository](https://github.com/argoproj/argo-helm/tree/master/charts/argocd-notifications).
