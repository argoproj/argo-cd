# Teams

## Parameters

The Teams notification service send message notifications using Teams bot and requires specifying the following settings:

* `recipientUrls` - the webhook url map, e.g. `channelName: https://example.com`

## Configuration

1. Open `Teams` and goto `Apps`
2. Find `Incoming Webhook` microsoft app and click on it
3. Press `Add to a team` -> select team and channel -> press `Set up a connector`
4. Enter webhook name and upload image (optional)
5. Press `Create` then copy webhook url and store it in `argocd-notifications-secret` and define it in `argocd-notifications-cm`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.teams: |
    recipientUrls:
      channelName: $channel-teams-url
```

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  channel-teams-url: https://example.com
```

6. Create subscription for your Teams integration:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-sync-succeeded.teams: channelName
```

## Templates

![](https://user-images.githubusercontent.com/18019529/114271500-9d2b8880-9a4c-11eb-85c1-f6935f0431d5.png)

Notification templates can be customized to leverage teams message sections, facts, themeColor, summary and potentialAction [feature](https://docs.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/how-to/connectors-using).

```yaml
template.app-sync-succeeded: |
  teams:
    themeColor: "#000080"
    sections: |
      [{
        "facts": [
          {
            "name": "Sync Status",
            "value": "{{.app.status.sync.status}}"
          },
          {
            "name": "Repository",
            "value": "{{.app.spec.source.repoURL}}"
          }
        ]
      }]
    potentialAction: |-
      [{
        "@type":"OpenUri",
        "name":"Operation Details",
        "targets":[{
          "os":"default",
          "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true"
        }]
      }]
    title: Application {{.app.metadata.name}} has been successfully synced
    text: Application {{.app.metadata.name}} has been successfully synced at {{.app.status.operationState.finishedAt}}.
    summary: "{{.app.metadata.name}} sync succeeded"
```

### facts field

You can use `facts` field instead of `sections` field.

```yaml
template.app-sync-succeeded: |
  teams:
    facts: |
      [{
        "name": "Sync Status",
        "value": "{{.app.status.sync.status}}"
      },
      {
        "name": "Repository",
        "value": "{{.app.spec.source.repoURL}}"
      }]
```

### theme color field

You can set theme color as hex string for the message.

![](https://user-images.githubusercontent.com/1164159/114864810-0718a900-9e24-11eb-8127-8d95da9544c1.png)

```yaml
template.app-sync-succeeded: |
  teams:
    themeColor: "#000080"
```

### summary field

You can set a summary of the message that will be shown on Notification & Activity Feed

![](https://user-images.githubusercontent.com/6957724/116587921-84c4d480-a94d-11eb-9da4-f365151a12e7.jpg)

![](https://user-images.githubusercontent.com/6957724/116588002-99a16800-a94d-11eb-807f-8626eb53b980.jpg)

```yaml
template.app-sync-succeeded: |
  teams:
    summary: "Sync Succeeded"
```