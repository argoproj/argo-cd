# Notifications Overview

Argo CD Notifications continuously monitors Argo CD applications and provides a flexible way to notify
users about important changes in the application state. Using a flexible mechanism of
[triggers](triggers.md) and [templates](templates.md) you can configure when the notification should be sent as
well as notification content. Argo CD Notifications includes the [catalog](catalog.md) of useful triggers and templates.
So you can just use them instead of reinventing new ones.

## Getting Started

* Install Triggers and Templates from the catalog

    ```bash
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

A common installation method for Argo CD Notifications is to install it in a dedicated namespace to manage a whole cluster. In this case, the administrator is the only
person who can configure notifications in that namespace generally. However, in some cases, it is required to allow end-users to configure notifications
for their Argo CD applications. For example, the end-user can configure notifications for their Argo CD application in the namespace where they have access to and their Argo CD application is running in.

This feature is based on applications in any namespace. See [applications in any namespace](../app-any-namespace.md) page for more information.

In order to enable this feature, the Argo CD administrator must reconfigure the argocd-notification-controller workloads to add  `--application-namespaces` and `--self-service-notification-enabled` parameters to the container's startup command.
`--application-namespaces` controls the list of namespaces that Argo CD applications are in. `--self-service-notification-enabled` turns on this feature.

The startup parameters for both can also be conveniently set up and kept in sync by specifying
the `application.namespaces` and `notificationscontroller.selfservice.enabled` in the argocd-cmd-params-cm ConfigMap instead of changing the manifests for the respective workloads. For example:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
data:
  application.namespaces: app-team-one, app-team-two
  notificationscontroller.selfservice.enabled: "true"
```

To use this feature, you can deploy configmap named `argocd-notifications-cm` and possibly a secret `argocd-notifications-secret` in the namespace where the Argo CD application lives.

When it is configured this way the controller will send notifications using both the controller level configuration (the configmap located in the same namespaces as the controller) as well as
the configuration located in the same namespace where the Argo CD application is at.

Example: Application team wants to receive notifications using PagerDutyV2, when the controller level configuration is only supporting Slack.

The following two resources are deployed in the namespace where the Argo CD application lives.
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.pagerdutyv2: |
    serviceKeys:
      my-service: $pagerduty-key-my-service
...
```
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: argocd-notifications-secret
type: Opaque
data:
  pagerduty-key-my-service: <pd-integration-key>
```

When an Argo CD application has the following subscriptions, user receives application sync failure message from pager duty.
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-sync-failed.pagerdutyv2: "<serviceID for Pagerduty>"
```

!!! note
    When the same notification service and trigger are defined in controller level configuration and application level configuration,
    both notifications will be sent according to its own configuration.

[Defining and using secrets within notification templates](templates.md/#defining-and-using-secrets-within-notification-templates) function is not available when flag `--self-service-notification-enable` is on.
