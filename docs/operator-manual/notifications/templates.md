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

- `app` holds the full [Application](../../declarative-setup.md#applications) resource object. See [commonly used `app` fields](#commonly-used-app-fields) below for details.
- `appProject` holds the [AppProject](../../declarative-setup.md#projects) object associated with the application. This provides access to project-level details like RBAC roles, policies, source repository restrictions, and destination cluster restrictions.
- `context` is a user-defined string map and might include any string keys and values.
- `secrets` provides access to sensitive data stored in `argocd-notifications-secret`
- `serviceType` holds the notification service type name (such as "slack" or "email). The field can be used to conditionally
render service-specific fields.
- `recipient` holds the recipient name.
- `time` provides time parsing functions. See [Change the timezone](#change-the-timezone) for usage.
- `strings` provides string manipulation functions.
- `repo` provides access to the Git repository. For example, `(call .repo.GetCommitMetadata .app.status.sync.revision)` returns commit metadata.

## Commonly used `app` fields

The `app` variable contains the full Application resource as an unstructured object. You can access any field using dot notation. The most commonly used fields in notification templates are:

**Metadata:**

| Expression | Description |
|------------|-------------|
| `.app.metadata.name` | Application name |
| `.app.metadata.namespace` | Application namespace |
| `.app.metadata.annotations` | Application annotations (map) |
| `.app.metadata.labels` | Application labels (map) |

**Spec (desired state):**

| Expression | Description |
|------------|-------------|
| `.app.spec.project` | Project name |
| `.app.spec.source.repoURL` | Git repository URL |
| `.app.spec.source.path` | Path within the repository |
| `.app.spec.source.targetRevision` | Target branch, tag, or commit |
| `.app.spec.source.chart` | Helm chart name (for Helm apps) |
| `.app.spec.destination.server` | Destination cluster URL |
| `.app.spec.destination.namespace` | Destination namespace |

**Status (actual state):**

| Expression | Description |
|------------|-------------|
| `.app.status.sync.status` | Sync status (`Synced`, `OutOfSync`, `Unknown`) |
| `.app.status.sync.revision` | Currently synced Git revision |
| `.app.status.health.status` | Health status (`Healthy`, `Degraded`, `Progressing`, `Missing`, `Suspended`, `Unknown`) |
| `.app.status.health.message` | Health status message |
| `.app.status.operationState.phase` | Operation phase (`Succeeded`, `Failed`, `Error`, `Running`) |
| `.app.status.operationState.message` | Operation result message |
| `.app.status.operationState.startedAt` | Operation start time |
| `.app.status.operationState.finishedAt` | Operation finish time |
| `.app.status.operationState.syncResult.revision` | Revision of the last sync operation |
| `.app.status.summary.images` | List of container images used by the application |
| `.app.status.conditions` | List of application conditions |

For the complete Application spec, refer to the [Application CRD reference](https://argo-cd.readthedocs.io/en/stable/operator-manual/application.yaml).

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

## Using AppProject information in templates

Templates can access the AppProject associated with an Application using the `appProject` variable. This is useful for including project-level information such as RBAC policies, source repositories, and destination clusters in notifications.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  template.app-project-info: |
    message: |
      Application {{.app.metadata.name}} belongs to project {{.appProject.metadata.name}}.
      Project description: {{.appProject.spec.description}}
      Allowed source repositories: {{range .appProject.spec.sourceRepos}}{{.}} {{end}}
  
  template.app-rbac-policies: |
    message: |
      Application: {{.app.metadata.name}}
      Project: {{.appProject.metadata.name}}
      RBAC Roles:
      {{range .appProject.spec.roles}}
      - Role: {{.name}}
        Policies: {{range .policies}}{{.}} {{end}}
      {{end}}
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

To change the timezone used when formatting time values in notifications, see
[Configuring the local timezone](#configuring-the-local-timezone).

## Functions

Templates have access to the set of built-in functions such as the functions of the [Sprig](https://masterminds.github.io/sprig/) package

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
