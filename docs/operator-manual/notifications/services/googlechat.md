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
  name: <config-map-name>
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
  message: The app {{ .app.metadata.name }} has succesfully synced!
```

A card message can be defined as follows:

```yaml
template.app-sync-succeeded: |
  googlechat:
    cards: |
      - header:
          title: ArgoCD Bot Notification
        sections:
          - widgets:
              - textParagraph:
                  text: The app {{ .app.metadata.name }} has succesfully synced!
          - widgets:
              - keyValue:
                  topLabel: Repository
                  content: {{ call .repo.RepoURLToHTTPS .app.spec.source.repoURL }}
              - keyValue:
                  topLabel: Revision
                  content: {{ .app.spec.source.targetRevision }}
              - keyValue:
                  topLabel: Author
                  content: {{ (call .repo.GetCommitMetadata .app.status.sync.revision).Author }}
```

The card message can be written in JSON too.
