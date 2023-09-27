# Overview

Argo CD Notifications continuously monitors Argo CD applications and provides a flexible way to notify
users about important changes in the application state. Using a flexible mechanism of
[triggers](triggers.md) and [templates](templates.md) you can configure when the notification should be sent as
well as notification content. Argo CD Notifications includes the [catalog](catalog.md) of useful triggers and templates.
So you can just use them instead of reinventing new ones.

## Getting Started

* Install Triggers and Templates from the catalog

```
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/notifications_catalog/install.yaml
```

* Add Email username and password token to `argocd-notifications-secret` secret

```bash
EMAIL_USER=<your-username>
 PASSWORD=<your-password>

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

Try syncing an application to get notified when the sync is completed.

## Namespace based configuration

!!! important
Available since v2.9

A common installation method for Argo CD Notifications is to install it in a dedicated namespace to manage a whole cluster. In this case, the administrator is the only
person who can configure notifications in that namespace generally. However, in some cases, it is required to allow end-users to configure notifications
for their Argo CD applications. For example, the end-user can configure notifications for their Argo CD application in the namespace where they have access to and their Argo CD application is running in.

To use this feature all you need to do is create the same configmap named `argo-rollouts-notification-configmap` and possibly
a secret `argo-rollouts-notification-secret` in the namespace where the Argo CD application lives. When it is configured this way the controller
will send notifications using both the controller level configuration (the configmap located in the same namespaces as the controller) as well as
the configmap located in the same namespaces where the rollout object is at.

To enable you need to add a flag to the controller `--self-service-notification-enabled`