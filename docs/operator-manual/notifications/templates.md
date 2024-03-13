The notification template is used to generate the notification content and is configured in the `argocd-notifications-cm` ConfigMap. The template is leveraging
the [html/template](https://golang.org/pkg/html/template/) golang package and allows customization of the notification message.
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
- `context` is a user-defined string map and might include any string keys and values.
- `secrets` provides access to sensitive data stored in `argocd-notifications-secret`
- `serviceType` holds the notification service type name (such as "slack" or "email). The field can be used to conditionally
render service-specific fields.
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

## Defining and using secrets within notification templates

Some notification service use cases will require the use of secrets within templates. This can be achieved with the use of
the `secrets` data variable available within the templates.

Given that we have the following `argocd-notifications-secret`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: argocd-notifications-secret
stringData:
  sampleWebhookToken: secret-token 
type: Opaque
```

We can use the defined `sampleWebhookToken` in a template as such:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  template.trigger-webhook: |
      webhook:
        sample-webhook:
          method: POST
          path: 'webhook/endpoint/with/auth'
          body: 'token={{ .secrets.sampleWebhookToken }}&variables[APP_SOURCE_PATH]={{ .app.spec.source.path }}
```

## Notification Service Specific Fields

The `message` field of the template definition allows creating a basic notification for any notification service. You can leverage notification service-specific
fields to create complex notifications. For example using service-specific you can add blocks and attachments for Slack, subject for Email or URL path, and body for Webhook.
See corresponding service [documentation](services/overview.md) for more information.

## Change the timezone

You can change the timezone to show in notifications as follows.

1. Call time functions.

    ```
    {{ (call .time.Parse .app.status.operationState.startedAt).Local.Format "2006-01-02T15:04:05Z07:00" }}
    ```

2. Set the `TZ` environment variable on the argocd-notifications-controller container.

    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: argocd-notifications-controller
    spec:
      template:
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

{!docs/operator-manual/notifications/functions.md!}
