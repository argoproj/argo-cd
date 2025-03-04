Here you can find some examples of what you can do with the notifications service in ArgoCD.

## Getting notified when a sync occurs and understanding how your resources changed

With ArgoCD you can build a notification system that tells you when a sync occurred and what it changed. 
To get notified via webhook when a sync occurs you can add the following trigger:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.webhook.on-deployed-webhook: |
    url: <your-webhook-url>
    headers:
    - name: "Content-Type"
      value: "application/json"

  template.on-deployed-template: |
    webhook:
      on-deployed-webhook:
        method: POST
        body: |
              {{toJson .app.status.operationState.syncResult}}

  trigger.on-deployed-trigger: |
    when: app.status.operationState.phase in ['Succeeded'] and app.status.health.status in ['Healthy', 'Degraded']
    oncePer: app.status.sync.revision
    send: [on-deployed-template]
```

This, as explained in the [triggers section](triggers/#avoid-sending-same-notification-too-often), will generate a notification when the app is synced and healthy. We then need to create a subscription for the webhook integration:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-deployed-trigger.on-deployed-webhook: ""
```

You can test that this works and see how the response looks by adding any webhook site and syncing our application. Here you can see that we receive a list of resources, with a message and some properties of them. For example:
```json
{
  "resources": [
    {
      "group": "apps",
      "hookPhase": "Running",
      "kind": "Deployment",
      "message": "deployment.apps/test configured",
      "name": "test",
      "namespace": "argocd",
      "status": "Synced",
      "syncPhase": "Sync",
      "version": "v1"
      images: [
        "nginx:1.27.1"
      ]
    },
    {
      "group": "autoscaling",
      "hookPhase": "Running",
      "kind": "HorizontalPodAutoscaler",
      "message": "horizontalpodautoscaler.autoscaling/test-hpa unchanged",
      "name": "test-hpa",
      "namespace": "argocd",
      "status": "Synced",
      "syncPhase": "Sync",
      "version": "v2"
    }
  ],
  "revision": "f3937462080c6946ff5ec4b5fa393e7c22388e4c",
  ...
}
```

We can leverage this information to know:

1. What resources have changed (not valid for Server Side Apply)
2. How they changed

To understand what resources changed we can check the message associated with each resource. Those that say that are unchanged were not affected in during the sync operation. With the list of changed resources, we can understand how they changed by looking into the images array.

With this information you can, for example:
1. Monitor the version of your image being deployed
2. Be alerted in case of any vulnerability present in a new deployment
3. Rollback deployments with images that are known to be faulty within your organisation

This helps you build a notification system that allows you to understand the status of your deployments in a more advanced manner.
