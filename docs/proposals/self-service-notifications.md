---
title: ArgoCD self-service notification

authors:
- "@mayzhang2000"
- "@zachaller"
- "@crenshaw-dev"
- "@leoluz"

sponsors:
- TBD

reviewers:
- TBD

approvers:
- TBD

creation-date: 2023-08-22  
last-updated: 2023-08-22
---

# Self Service Notification for ArgoCD

## Summary
This proposal is to enable application teams to have their own configurations of ArgoCD notifications aka self-service notification.
Application team will be able to receive notifications from the default configuration of notifications as well as their own configuration of notifications.

## Motivation
As of now the configuration for ArgoCD notification is centrally managed. Only ArgoCD admin can make notification configuration changes.

When application teams use PagerDutyV2 for their notification service, every application team needs to create an integration key for each service in Pager Duty.
ArgoCD amin needs to add the integration key to kubernete's secret `argocd-notifications-secret`,
also needs to modify configmap `argocd-notifications-cm` to add the reference to the integration key stored in above secret under the list of `serviceKeys`

When there are many application teams want to use PagerDutyV2 for their notification service, they all have to go to ArgoCD admin team. This does not scale.

We need to enable application team to configure their own notification configurations.

## Proposal
Deploy app-specific notification configuration resources in the same namespace where ArgoCD application is in.
ArgoCD applications are in any namespaces.

Enhance notification controller to support app in any namespace so that it sends notifications for apps in any namespaces.
Notification controller knows the set of namespaces to monitor by using `--application-namespaces` startup parameter. 
It can also be conveniently set up and kept in sync by specifying the application.namespaces settings in the argocd-cmd-params-cm ConfigMap.

Enhance notification controller to create notification-engine controller using function `NewControllerWithNamespaceSupport`. This sets flag `namespaceSupport = true`. 
When this flag is on, notification-engine controller calls apiFactory to creates apis from both the default configuration and also configuration in the application's namespace.

![img.png](images/self-service-notifications.png)

### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

#### Use case 1: application team wants to configure ArgoCD notification using pager duty V2.
I want to receive Pager Duty notification when my application is degraded, but our default ArgoCD notification is using slack.

* Create two additional resources `argocd-notifications-secret` and `argocd-notifications-cm`.
  In these resources I used PagerDutyV2 as service type.
* Deploy these two additional resources to the same namespace as my ArgoCD application.

### Detailed examples

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-sync-running.pagerdutyv2: hello-2_xxxx
    notifications.argoproj.io/subscribe.on-sync-running.slack: may-test
  name: guest-book
  namespace: app-may-test
spec:
  destination:
    namespace: app-may-test
    server: https://xxxx
  project: default
  source:
    path: guestbook
    repoURL: https://github.com/mayzhang2000/argocd-example-apps.git
    targetRevision: HEAD
```

```yaml
apiVersion: v1
data:
  service.pagerdutyv2: |
    serviceKeys:
      hello-2_xxxxx: $pagerdutyv2-key-hello-2_xxxx
  template.app-sync-running: |
    pagerdutyv2:
      summary: "App {{.app.metadata.name}} sync running "
      severity: "info"
      source: "{{.app.metadata.name}}"
  trigger.on-sync-running: |
    - description: Application is being synced
      send:
      - app-sync-running
      when: app.status.operationState.phase in ['Running']
kind: ConfigMap
metadata:
  labels:
    app.kubernetes.io/instance: mayeast
  name: argocd-notifications-cm
  namespace: app-may-test
```

```yaml
apiVersion: v1
data:
  pagerdutyv2-key-hello-2_4759196499290493255: ++++++++
kind: Secret
metadata:
  labels:
    app.kubernetes.io/instance: mayeast
  name: argocd-notifications-secret
  namespace: app-may-test
type: Opaque
```

### Security Considerations

### Risks and Mitigations

## Drawbacks

## Alternatives
