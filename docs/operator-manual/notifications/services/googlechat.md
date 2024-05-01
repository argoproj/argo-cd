# Google Chat

## Parameters

The Google Chat notification service send message notifications to a google chat webhook. This service uses the following settings:

* `webhooks` - a map of the form `webhookName: webhookUrl`

## Configuration

1. Open `Google chat` and go to the space to which you want to send messages
2. From the menu at the top of the page, select **Configure Webhooks**
3. Under **Incoming Webhooks**, click **Add Webhook**
4. Give a name to the webhook, optionally add an image and click **Save**
5. Copy the URL next to your webhook
6. Store the URL in `argocd-notification-secret` and declare it in `argocd-notifications-cm`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.googlechat: |
    webhooks:
      spaceName: $space-webhook-url
```

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  space-webhook-url: https://chat.googleapis.com/v1/spaces/<space_id>/messages?key=<key>&token=<token>  
```

6. Create a subscription for your space

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-sync-succeeded.googlechat: spaceName
```

## Templates

You can send [simple text](https://developers.google.com/chat/reference/message-formats/basic) or [card messages](https://developers.google.com/chat/reference/message-formats/cards) to a Google Chat space. A simple text message template can be defined as follows:

```yaml
template.app-sync-succeeded: |
  message: The app {{ .app.metadata.name }} has successfully synced!
```

A card message can be defined as follows:

```yaml
template.app-sync-succeeded: |
  googlechat:
    cardsV2: |
      - header:
          title: ArgoCD Bot Notification
        sections:
          - widgets:
              - decoratedText:
                  text: The app {{ .app.metadata.name }} has successfully synced!
          - widgets:
              - decoratedText:
                  topLabel: Repository
                  text: {{ call .repo.RepoURLToHTTPS .app.spec.source.repoURL }}
              - decoratedText:
                  topLabel: Revision
                  text: {{ .app.spec.source.targetRevision }}
              - decoratedText:
                  topLabel: Author
                  text: {{ (call .repo.GetCommitMetadata .app.status.sync.revision).Author }}
```
All [Card fields](https://developers.google.com/chat/api/reference/rest/v1/cards#Card_1) are supported and can be used
in notifications. It is also possible to use the previous (now deprecated) `cards` key to use the legacy card fields,
but this is not recommended as Google has deprecated this field and recommends using the newer `cardsV2`.

The card message can be written in JSON too.

## Chat Threads

It is possible send both simple text and card messages in a chat thread by specifying a unique key for the thread. The thread key can be defined as follows:

```yaml
template.app-sync-succeeded: |
  message: The app {{ .app.metadata.name }} has successfully synced!
  googlechat:
    threadKey: {{ .app.metadata.name }}
```
