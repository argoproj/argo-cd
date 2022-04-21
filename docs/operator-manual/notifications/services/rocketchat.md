# Rocket.Chat

## Parameters

The Rocket.Chat notification service configuration includes following settings:

* `email` - the Rocker.Chat user's email
* `password` - the Rocker.Chat user's password
* `alias` - optional alias that should be used to post message
* `icon` - optional message icon
* `avatar` - optional message avatar
* `serverUrl` - optional Rocket.Chat server url

## Configuration

1. Login to your RocketChat instance
2. Go to user management

![2](https://user-images.githubusercontent.com/15252187/115824993-7ccad900-a411-11eb-89de-6a0c4438ffdf.png)

3. Add new user with `bot` role. Also note that `Require password change` checkbox mus be not checked

![3](https://user-images.githubusercontent.com/15252187/115825174-b4d21c00-a411-11eb-8f20-cda48cea9fad.png)

4. Copy username and password that you was created for bot user
5. Create a public or private channel, or a team, for this example `my_channel`
6. Add your bot to this channel **otherwise it won't work**
7. Store email and password in argocd_notifications-secret Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  rocketchat-email: <email>
  rocketchat-password: <password>
```

8. Finally, use these credentials to configure the RocketChat integration in the `argocd-configmap` config map:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.rocketchat: |
    email: $rocketchat-email
    password: $rocketchat-password
```

9. Create a subscription for your Rocket.Chat integration:

*Note: channel, team or user must be prefixed with # or @ elsewhere we will be interpretative destination as a room ID*

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-sync-succeeded.rocketchat: #my_channel
```

## Templates

Notification templates can be customized with RocketChat [attachments](https://developer.rocket.chat/api/rest-api/methods/chat/postmessage#attachments-detail).

*Note: Attachments structure in RocketChat is same with Slack attachments [feature](https://api.slack.com/messaging/composing/layouts).*

<!-- TODO: @sergeyshevch Need to add screenshot with RocketChat attachments -->

The message attachments can be specified in `attachments` string fields under `rocketchat` field:

```yaml
template.app-sync-status: |
  message: |
    Application {{.app.metadata.name}} sync is {{.app.status.sync.status}}.
    Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.
  rocketchat:
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
