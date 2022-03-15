The notification template is used to generate the notification content and configured in `argocd-notifications-cm` ConfigMap. The template is leveraging
[html/template](https://golang.org/pkg/html/template/) golang package and allow to customize notification message.
Templates are meant to be reusable and can be referenced by multiple triggers.

The following template is used to notify the user about application sync status.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  template.my-custom-template-slack-template: |
    message: |
      Application {{.app.metadata.name}} sync is {{.app.status.sync.status}}.
      Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.
```

Each template has access to the following fields:

- `app` holds the application object.
- `context` is user defined string map and might include any string keys and values.
- `serviceType` holds the notification service type name. The field can be used to conditionally
render service specific fields.
- `recipient` holds the recipient name.

## Defining user-defined `context`

It is possible to define some shared context between all notification templates by setting a top-level
YAML document of key-value pairs, which can then be used within templates, like so:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  context: |
    region: east
    environmentName: staging

  template.a-slack-template-with-context: |
    message: "Something happened in {{ .context.environmentName }} in the {{ .context.region }} data center!"
```

## Notification Service Specific Fields

The `message` field of the template definition allows creating a basic notification for any notification service. You can leverage notification service-specific
fields to create complex notifications. For example using service-specific you can add blocks and attachments for Slack, subject for Email or URL path, and body for Webhook.
See corresponding service [documentation](services/overview.md) for more information.

## Change the timezone

You can change the timezone to show it as follows.

1. Call time functions.

```
{{ (call .time.Parse .app.status.operationState.startedAt).Local.Format "2006-01-02T15:04:05Z07:00" }}
```

2. Set environment to container.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-notifications-controller
spec:
(snip)
    spec:
      containers:
      - name: argocd-notifications-controller
        env:
        - name: TZ
          value: Asia/Tokyo
```

## Functions

Templates have access to the set of built-in functions:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  template.my-custom-template-slack-template: |
    message: "Author: {{(call .repo.GetCommitMetadata .app.status.sync.revision).Author}}"
```

{!functions.md!}
