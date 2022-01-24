# Slack

If you want to send message using incoming webhook, you can use [webhook](./webhook.md#send-slack).

## Parameters

The Slack notification service configuration includes following settings:

* `token` - the app token
* `apiURL` - optional, the server url, e.g. https://example.com/api
* `username` - optional, the app username
* `icon` - optional, the app icon, e.g. :robot_face: or https://example.com/image.png
* `insecureSkipVerify` - optional bool, true or false

## Configuration

1. Create Slack Application using https://api.slack.com/apps?new_app=1
![1](https://user-images.githubusercontent.com/426437/73604308-4cb0c500-4543-11ea-9092-6ca6bae21cbb.png)
1. Once application is created navigate to `Enter OAuth & Permissions`
![2](https://user-images.githubusercontent.com/426437/73604309-4d495b80-4543-11ea-9908-4dea403d3399.png)
1. Click `Permissions` under `Add features and functionality` section and add `chat:write` scope. To use the optional username and icon overrides in the Slack notification service also add the `chat:write.customize` scope.
![3](https://user-images.githubusercontent.com/426437/73604310-4d495b80-4543-11ea-8576-09cd91aea0e5.png)
1. Scroll back to the top, click 'Install App to Workspace' button and confirm the installation.
![4](https://user-images.githubusercontent.com/426437/73604311-4d495b80-4543-11ea-9155-9d216b20ec86.png)
1. Once installation is completed copy the OAuth token.
![5](https://user-images.githubusercontent.com/426437/73604312-4d495b80-4543-11ea-832b-a9d9d5e4bc29.png)

1. Create a public or private channel, for this example `my_channel`
1. Invite your slack bot to this channel **otherwise slack bot won't be able to deliver notifications to this channel**
1. Store Oauth access token in `argocd-notifications-secret` secret

    ```yaml
    apiVersion: v1
    kind: Secret
    metadata:
      name: <secret-name>
    stringData:
      slack-token: <Oauth-access-token>
    ```

1. Define service type slack in data section of `argocd-notifications-cm` configmap:
service
    ```yaml
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: <config-map-name>
    data:
      service.slack: |
        token: $slack-token
    ```

1. Add annotation in application yaml file to enable notifications for specific argocd app

    ```yaml
    apiVersion: argoproj.io/v1alpha1
    kind: Application
    metadata:
      annotations:
        notifications.argoproj.io/subscribe.on-sync-succeeded.slack: my_channel
    ```

## Templates

Notification templates can be customized to leverage slack message blocks and attachments
[feature](https://api.slack.com/messaging/composing/layouts).

![](https://user-images.githubusercontent.com/426437/72776856-6dcef880-3bc8-11ea-8e3b-c72df16ee8e6.png)

The message blocks and attachments can be specified in `blocks` and `attachments` string fields under `slack` field:

```yaml
template.app-sync-status: |
  message: |
    Application {{.app.metadata.name}} sync is {{.app.status.sync.status}}.
    Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.
  slack:
    attachments: |
      [{
        "title": "{{.app.metadata.name}}",
        "title_link": "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
        "color": "#18be52",
        "fields": [{
          "title": "Sync Status",
          "value": "{{.app.status.sync.status}}",
          "short": true
        }, {
          "title": "Repository",
          "value": "{{.app.spec.source.repoURL}}",
          "short": true
        }]
      }]
```

The messages can be aggregated to the slack threads by grouping key which can be specified in a `groupingKey` string field under `slack` field.
`groupingKey` is used across each template and works independently on each slack channel.
When multiple applications will be updated at the same time or frequently, the messages in slack channel can be easily read by aggregating with git commit hash, application name, etc.
Furthermore, the messages can be broadcast to the channel at the specific template by `notifyBroadcast` field.

```yaml
template.app-sync-status: |
  message: |
    Application {{.app.metadata.name}} sync is {{.app.status.sync.status}}.
    Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.
  slack:
    attachments: |
      [{
        "title": "{{.app.metadata.name}}",
        "title_link": "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
        "color": "#18be52",
        "fields": [{
          "title": "Sync Status",
          "value": "{{.app.status.sync.status}}",
          "short": true
        }, {
          "title": "Repository",
          "value": "{{.app.spec.source.repoURL}}",
          "short": true
        }]
      }]
    # Aggregate the messages to the thread by git commit hash
    groupingKey: "{{.app.status.sync.revision}}"
    notifyBroadcast: false
template.app-sync-failed: |
  message: |
    Application {{.app.metadata.name}} sync is {{.app.status.sync.status}}.
    Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.
  slack:
    attachments: |
      [{
        "title": "{{.app.metadata.name}}",
        "title_link": "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
        "color": "#ff0000",
        "fields": [{
          "title": "Sync Status",
          "value": "{{.app.status.sync.status}}",
          "short": true
        }, {
          "title": "Repository",
          "value": "{{.app.spec.source.repoURL}}",
          "short": true
        }]
      }]
    # Aggregate the messages to the thread by git commit hash
    groupingKey: "{{.app.status.sync.revision}}"
    notifyBroadcast: true
```

The message is sent according to the `deliveryPolicy` string field under the `slack` field. The available modes are `Post` (default), `PostAndUpdate`, and `Update`. The `PostAndUpdate` and `Update` settings require `groupingKey` to be set.
