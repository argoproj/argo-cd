# Teams Workflows

## Overview

The Teams Workflows notification service sends message notifications using Microsoft Teams Workflows (Power Automate). This is the recommended replacement for the legacy Office 365 Connectors service, which will be retired on March 31, 2026.

## Parameters

The Teams Workflows notification service requires specifying the following settings:

* `recipientUrls` - the webhook url map, e.g. `channelName: https://api.powerautomate.com/webhook/...`

## Supported Webhook URL Formats

The service supports the following Microsoft Teams Workflows webhook URL patterns:

- `https://api.powerautomate.com/...`
- `https://api.powerplatform.com/...`
- `https://flow.microsoft.com/...`
- URLs containing `/powerautomate/` in the path

## Configuration

1. Open `Teams` and go to the channel you wish to set notifications for
2. Click on the 3 dots next to the channel name
3. Select`Workflows`
4. Click on `Manage`
5. Click `New flow`
6. Write `Send webhook alerts to a channel` in the search bar or select it from the template list 
7. Choose your team and channel
8. Configure the webhook name and settings
9. Copy the webhook URL (it will be from `api.powerautomate.com`, `api.powerplatform.com`, or `flow.microsoft.com`)
10. Store it in `argocd-notifications-secret` and define it in `argocd-notifications-cm`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.teams-workflows: |
    recipientUrls:
      channelName: $channel-workflows-url
```

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  channel-workflows-url: https://api.powerautomate.com/webhook/your-webhook-id
```

11. Create subscription for your Teams Workflows integration:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-sync-succeeded.teams-workflows: channelName
```

## Channel Support

- ✅ Standard Teams channels
- ✅ Shared channels (as of December 2025)
- ✅ Private channels (as of December 2025)

Teams Workflows provides enhanced channel support compared to Office 365 Connectors, allowing you to post to shared and private channels in addition to standard channels.

## Adaptive Card Format

The Teams Workflows service uses **Adaptive Cards** exclusively, which is the modern, flexible card format for Microsoft Teams. All notifications are automatically converted to Adaptive Card format and wrapped in the required message envelope.

### Option 1: Using Template Fields (Recommended)

The service automatically converts template fields to Adaptive Card format. This is the simplest and most maintainable approach:

```yaml
template.app-sync-succeeded: |
  teams-workflows:
    # ThemeColor supports Adaptive Card semantic colors: "Good", "Warning", "Attention", "Accent"
    # or hex colors like "#000080"
    themeColor: "Good"
    title: Application {{.app.metadata.name}} has been successfully synced
    text: Application {{.app.metadata.name}} has been successfully synced at {{.app.status.operationState.finishedAt}}.
    summary: "{{.app.metadata.name}} sync succeeded"
    facts: |
      [{
        "name": "Sync Status",
        "value": "{{.app.status.sync.status}}"
      }, {
        "name": "Repository",
        "value": "{{.app.spec.source.repoURL}}"
      }]
    sections: |
      [{
        "facts": [
          {
            "name": "Namespace",
            "value": "{{.app.metadata.namespace}}"
          },
          {
            "name": "Cluster",
            "value": "{{.app.spec.destination.server}}"
          }
        ]
      }]
    potentialAction: |-
      [{
        "@type": "OpenUri",
        "name": "View in Argo CD",
        "targets": [{
          "os": "default",
          "uri": "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}"
        }]
      }]
```

**How it works:**
- `title` → Converted to a large, bold TextBlock
- `text` → Converted to a regular TextBlock
- `facts` → Converted to a FactSet element
- `sections` → Facts within sections are extracted and converted to FactSet elements
- `potentialAction` → OpenUri actions are converted to Action.OpenUrl
- `themeColor` → Applied to the title TextBlock (supports semantic colors like "Good", "Warning", "Attention", "Accent" or hex colors)

### Option 2: Custom Adaptive Card JSON

For full control and advanced features, you can provide a complete Adaptive Card JSON template:

```yaml
template.app-sync-succeeded: |
  teams-workflows:
    adaptiveCard: |
      {
        "type": "AdaptiveCard",
        "version": "1.4",
        "body": [
          {
            "type": "TextBlock",
            "text": "Application {{.app.metadata.name}} synced successfully",
            "size": "Large",
            "weight": "Bolder",
            "color": "Good"
          },
          {
            "type": "TextBlock",
            "text": "Application {{.app.metadata.name}} has been successfully synced at {{.app.status.operationState.finishedAt}}.",
            "wrap": true
          },
          {
            "type": "FactSet",
            "facts": [
              {
                "title": "Sync Status",
                "value": "{{.app.status.sync.status}}"
              },
              {
                "title": "Repository",
                "value": "{{.app.spec.source.repoURL}}"
              }
            ]
          }
        ],
        "actions": [
          {
            "type": "Action.OpenUrl",
            "title": "View in Argo CD",
            "url": "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}"
          }
        ]
      }
```

**Note:** When using `adaptiveCard`, you only need to provide the AdaptiveCard JSON structure (not the full message envelope). The service automatically wraps it in the required `message` + `attachments` format for Teams Workflows.

**Important:** If you provide `adaptiveCard`, it takes precedence over all other template fields (`title`, `text`, `facts`, etc.).

## Template Fields

The Teams Workflows service supports the following template fields, which are automatically converted to Adaptive Card format:

### Standard Fields

- `title` - Message title (converted to large, bold TextBlock)
- `text` - Message text content (converted to TextBlock)
- `summary` - Summary text (currently not used in Adaptive Cards, but preserved for compatibility)
- `themeColor` - Color for the title. Supports:
  - Semantic colors: `"Good"` (green), `"Warning"` (yellow), `"Attention"` (red), `"Accent"` (blue)
  - Hex colors: `"#000080"`, `"#FF0000"`, etc.
- `facts` - JSON array of fact key-value pairs (converted to FactSet)
  ```yaml
  facts: |
    [{
      "name": "Status",
      "value": "{{.app.status.sync.status}}"
    }]
  ```
- `sections` - JSON array of sections containing facts (facts are extracted and converted to FactSet)
  ```yaml
  sections: |
    [{
      "facts": [{
        "name": "Namespace",
        "value": "{{.app.metadata.namespace}}"
      }]
    }]
  ```
- `potentialAction` - JSON array of action buttons (OpenUri actions converted to Action.OpenUrl)
  ```yaml
  potentialAction: |-
    [{
      "@type": "OpenUri",
      "name": "View Details",
      "targets": [{
        "os": "default",
        "uri": "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}"
      }]
    }]
  ```

### Advanced Fields

- `adaptiveCard` - Complete Adaptive Card JSON template (takes precedence over all other fields)
  - Only provide the AdaptiveCard structure, not the message envelope
  - Supports full Adaptive Card 1.4 specification
  - Allows access to all Adaptive Card features (containers, columns, images, etc.)

- `template` - Raw JSON template (legacy, use `adaptiveCard` instead)

### Field Conversion Details

| Template Field | Adaptive Card Element | Notes |
|---------------|----------------------|-------|
| `title` | `TextBlock` with `size: "Large"`, `weight: "Bolder"` | ThemeColor applied to this element |
| `text` | `TextBlock` with `wrap: true` | Uses `n.Message` if `text` is empty |
| `facts` | `FactSet` | Each fact becomes a `title`/`value` pair |
| `sections[].facts` | `FactSet` | Facts extracted from sections |
| `potentialAction[OpenUri]` | `Action.OpenUrl` | Only OpenUri actions are converted |
| `themeColor` | Applied to title `TextBlock.color` | Supports semantic and hex colors |

## Migration from Office 365 Connectors

If you're currently using the `teams` service with Office 365 Connectors, follow these steps to migrate:

1. **Create a new Workflows webhook** using the configuration steps above

2. **Update your service configuration:**
   - Change from `service.teams` to `service.teams-workflows`
   - Update the webhook URL to your new Workflows webhook URL

3. **Update your templates:**
   - Change `teams:` to `teams-workflows:` in your templates
   - Your existing template fields (`title`, `text`, `facts`, `sections`, `potentialAction`) will automatically be converted to Adaptive Card format
   - No changes needed to your template structure - the conversion is automatic

4. **Update your subscriptions:**
   ```yaml
   # Old
   notifications.argoproj.io/subscribe.on-sync-succeeded.teams: channelName
   
   # New
   notifications.argoproj.io/subscribe.on-sync-succeeded.teams-workflows: channelName
   ```

5. **Test and verify:**
   - Send a test notification to verify it works correctly
   - Once verified, you can remove the old Office 365 Connector configuration

**Note:** Your existing templates will work without modification. The service automatically converts your template fields to Adaptive Card format, so you get the benefits of modern cards without changing your templates.

## Differences from Office 365 Connectors

| Feature | Office 365 Connectors | Teams Workflows |
|---------|----------------------|-----------------|
| Service Name | `teams` | `teams-workflows` |
| Standard Channels | ✅ | ✅ |
| Shared Channels | ❌ | ✅ (Dec 2025+) |
| Private Channels | ❌ | ✅ (Dec 2025+) |
| Card Format | messageCard (legacy) | Adaptive Cards (modern) |
| Template Conversion | N/A | Automatic conversion from template fields |
| Retirement Date | March 31, 2026 | Active |

## Adaptive Card Features

The Teams Workflows service leverages Adaptive Cards, which provide:

- **Rich Content**: Support for text, images, fact sets, and more
- **Flexible Layout**: Containers, columns, and adaptive layouts
- **Interactive Elements**: Action buttons, input fields, and more
- **Semantic Colors**: Built-in color schemes (Good, Warning, Attention, Accent)
- **Cross-Platform**: Works across Teams, Outlook, and other Microsoft 365 apps

### Example: Advanced Adaptive Card Template

For complex notifications, you can use the full Adaptive Card specification:

```yaml
template.app-sync-succeeded-advanced: |
  teams-workflows:
    adaptiveCard: |
      {
        "type": "AdaptiveCard",
        "version": "1.4",
        "body": [
          {
            "type": "Container",
            "items": [
              {
                "type": "ColumnSet",
                "columns": [
                  {
                    "type": "Column",
                    "width": "auto",
                    "items": [
                      {
                        "type": "Image",
                        "url": "https://example.com/success-icon.png",
                        "size": "Small"
                      }
                    ]
                  },
                  {
                    "type": "Column",
                    "width": "stretch",
                    "items": [
                      {
                        "type": "TextBlock",
                        "text": "Application {{.app.metadata.name}}",
                        "weight": "Bolder",
                        "size": "Large"
                      },
                      {
                        "type": "TextBlock",
                        "text": "Successfully synced",
                        "spacing": "None",
                        "isSubtle": true
                      }
                    ]
                  }
                ]
              },
              {
                "type": "FactSet",
                "facts": [
                  {
                    "title": "Status",
                    "value": "{{.app.status.sync.status}}"
                  },
                  {
                    "title": "Repository",
                    "value": "{{.app.spec.source.repoURL}}"
                  }
                ]
              }
            ]
          }
        ],
        "actions": [
          {
            "type": "Action.OpenUrl",
            "title": "View in Argo CD",
            "url": "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}"
          }
        ]
      }
```