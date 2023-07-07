# Email

## Parameters

The Email notification service sends email notifications using SMTP protocol and requires specifying the following settings:

* `host` - the SMTP server host name
* `port` - the SMTP server port
* `username` - username
* `password` - password
* `from` - from email address
* `html` - optional bool, true or false
* `insecure_skip_verify` - optional bool, true or false

## Example

The following snippet contains sample Gmail service configuration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.email.gmail: |
    username: $email-username
    password: $email-password
    host: smtp.gmail.com
    port: 465
    from: $email-username
```

Without authentication:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.email.example: |
    host: smtp.example.com
    port: 587
    from: $email-username
```

## Template

Notification templates support specifying subject for email notifications:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  template.app-sync-succeeded: |
    email:
      subject: Application {{.app.metadata.name}} has been successfully synced.
    message: |
      {{if eq .serviceType "slack"}}:white_check_mark:{{end}} Application {{.app.metadata.name}} has been successfully synced at {{.app.status.operationState.finishedAt}}.
      Sync operation details are available at: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true .
```
