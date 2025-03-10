Here you can find some examples of what you can do with the notifications service in Argo CD.

## Getting notified when a sync occurs and understanding how your resources changed

With Argo CD you can build a notification system that tells you when a sync occurred and what it changed. 
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
    when: app.status.operationState.phase in ['Succeeded'] and app.status.health.status == 'Healthy'
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
      # The images array follows the same order as in the resource yaml
      "images": [
        "nginx:1.27.1"
      ],
      "kind": "Deployment",
      "message": "deployment.apps/test configured",
      "name": "test",
      "namespace": "argocd",
      "status": "Synced",
      "syncPhase": "Sync",
      "version": "v1"
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

To understand what resources changed we can check the message associated with each resource. Those that say that are unchanged were not affected during the sync operation. With the list of changed resources, we can understand how they changed by looking into the images array.

With this information you can, for example:

1. Monitor the version of your image being deployed
2. Rollback deployments with images that are known to be faulty within your organisation
3. Detect unexpected image changes: by monitoring the images array in the webhook payload, you can verify that only expected container images are being deployed

This helps you build a notification system that allows you to understand the status of your deployments in a more advanced manner.

## Send the list of images to Slack

Here we can use a similar setup as the one above, but change the receiver to be Slack:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
data:
  service.slack: |
    token: <your-slack-bot-token>

  template.on-deployed-template: |
    slack:
      message: |
        *Deployment Notification*
        *Application:* `{{.app.metadata.name}}`
        *Namespace:* `{{.app.spec.destination.namespace}}`
        *Revision:* `{{.app.status.sync.revision}}`
        *Deployed Images:*
          {{- range $resource := .app.status.operationState.syncResult.resources -}}
            {{- range $image := $resource.images -}}
              - "{{$image}}"
            {{- end }}
          {{- end }}
  trigger.on-deployed-trigger: |
    when: app.status.operationState.phase in ['Succeeded'] and app.status.health.status == 'Healthy'
    oncePer: app.status.sync.revision
    send: [on-deployed-template]
```

Now, with the setup above, a sync will send the list of images to your Slack application. For more information about integratin with Slack, see the [Slack integration guide](/operator-manual/notifications/services/slack/).

### Deduplicating images

Although the field in `syncResult.resources` contains only resources declared by the user in the GitOps repository you might end up with duplicated images depending on your setup. To avoid having duplicated images, you need to create an external webhook receiver that deduplicates the images, and then send the message to Slack. 
