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