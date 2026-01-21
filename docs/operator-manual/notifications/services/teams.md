# Teams (Office 365 Connectors)

## ⚠️ Deprecation Notice

**Office 365 Connectors are being retired by Microsoft.**

Microsoft is retiring the Office 365 Connectors service in Teams. The service will be fully retired by **March 31, 2026** (extended from the original timeline of December 2025). 

### What this means:
- **Old Office 365 Connectors** (webhook URLs from `webhook.office.com`) will stop working after the retirement date
- **New Power Automate Workflows** (webhook URLs from `api.powerautomate.com`, `api.powerplatform.com`, or `flow.microsoft.com`) are the recommended replacement

### Migration Required:
If you are currently using Office 365 Connectors (Incoming Webhook), you should migrate to Power Automate Workflows before the retirement date. The notifications-engine automatically detects the webhook type and handles both formats, but you should plan your migration.

**Migration Resources:**
- [Microsoft Deprecation Notice](https://devblogs.microsoft.com/microsoft365dev/retirement-of-office-365-connectors-within-microsoft-teams/)
- [Create incoming webhooks with Workflows for Microsoft Teams](https://support.microsoft.com/en-us/office/create-incoming-webhooks-with-workflows-for-microsoft-teams-8ae491c7-0394-4861-ba59-055e33f75498)

---

## Parameters

The Teams notification service sends message notifications using Office 365 Connectors and requires specifying the following settings:

* `recipientUrls` - the webhook url map, e.g. `channelName: https://outlook.office.com/webhook/...`

> **⚠️ Deprecation Notice:** Office 365 Connectors will be retired by Microsoft on **March 31, 2026**. We recommend migrating to the [Teams Workflows service](./teams-workflows.md) for continued support and enhanced features.

## Configuration

> **💡 For Power Automate Workflows (Recommended):** See the [Teams Workflows documentation](./teams-workflows.md) for detailed configuration instructions.

### Office 365 Connectors (Deprecated - Retiring March 31, 2026)

> **⚠️ Warning:** This method is deprecated and will stop working after March 31, 2026. Please migrate to Power Automate Workflows.

1. Open `Teams` and goto `Apps`
2. Find `Incoming Webhook` microsoft app and click on it
3. Press `Add to a team` -> select team and channel -> press `Set up a connector`
4. Enter webhook name and upload image (optional)
5. Press `Create` then copy webhook url (it will be from `webhook.office.com`)
6. Store it in `argocd-notifications-secret` and define it in `argocd-notifications-cm`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
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
  channel-teams-url: https://webhook.office.com/webhook/your-webhook-id  # Office 365 Connector (deprecated)
```

> **Note:** For Power Automate Workflows webhooks, use the [Teams Workflows service](./teams-workflows.md) instead.

### Webhook Type Detection

The `teams` service supports Office 365 Connectors (deprecated):

- **Office 365 Connectors**: URLs from `webhook.office.com` (deprecated)
  - Requires response body to be exactly `"1"` for success
  - Will stop working after March 31, 2026

7. Create subscription for your Teams integration:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-sync-succeeded.teams: channelName
```

## Channel Support

- ✅ Standard Teams channels only

> **Note:** Office 365 Connectors only support standard Teams channels. For shared channels or private channels, use the [Teams Workflows service](./teams-workflows.md).

## Templates

![](https://user-images.githubusercontent.com/18019529/114271500-9d2b8880-9a4c-11eb-85c1-f6935f0431d5.png)

[Notification templates](../templates.md) can be customized to leverage teams message sections, facts, themeColor, summary and potentialAction [feature](https://docs.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/how-to/connectors-using).

The Teams service uses the **messageCard** format (MessageCard schema) which is compatible with Office 365 Connectors.

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

## Migration to Teams Workflows

If you're currently using Office 365 Connectors, see the [Teams Workflows documentation](./teams-workflows.md) for migration instructions and enhanced features.
