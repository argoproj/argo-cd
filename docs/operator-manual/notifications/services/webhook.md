## Configuration

The webhook notification service allows sending a generic HTTP request using the templatized request body and URL.
Using Webhook you might trigger a Jenkins job, update Github commit status.

Use the following steps to configure webhook:

1 Register webhook in `argocd-notifications-cm` ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.webhook.<webhook-name>: |
    url: https://<hostname>/<optional-path>
    headers: #optional headers
    - name: <header-name>
      value: <header-value>
    basicAuth: #optional username password
      username: <username>
      password: <api-key>
```

2 Define template that customizes webhook request method, path and body:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  template.github-commit-status: |
    webhook:
      <webhook-name>:
        method: POST # one of: GET, POST, PUT, PATCH. Default value: GET 
        path: <optional-path-template>
        body: |
          <optional-body-template>
  trigger.<trigger-name>: |
    - when: app.status.operationState.phase in ['Succeeded']
      send: [github-commit-status]
```

3 Create subscription for webhook integration:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.<trigger-name>.<webhook-name>: ""
```

## Examples

### Set Github commit status

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.webhook.github: |
    url: https://api.github.com
    headers: #optional headers
    - name: Authorization
      value: token $github-token
```

2 Define template that customizes webhook request method, path and body:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.webhook.github: |
    url: https://api.github.com
    headers: #optional headers
    - name: Authorization
      value: token $github-token

  template.github-commit-status: |
    webhook:
      github:
        method: POST
        path: /repos/{{call .repo.FullNameByRepoURL .app.spec.source.repoURL}}/statuses/{{.app.status.operationState.operation.sync.revision}}
        body: |
          {
            {{if eq .app.status.operationState.phase "Running"}} "state": "pending"{{end}}
            {{if eq .app.status.operationState.phase "Succeeded"}} "state": "success"{{end}}
            {{if eq .app.status.operationState.phase "Error"}} "state": "error"{{end}}
            {{if eq .app.status.operationState.phase "Failed"}} "state": "error"{{end}},
            "description": "ArgoCD",
            "target_url": "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
            "context": "continuous-delivery/{{.app.metadata.name}}"
          }
```

### Start Jenkins Job

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.webhook.jenkins: |
    url: http://<jenkins-host>/job/<job-name>/build?token=<job-secret>
    basicAuth:
      username: <username>
      password: <api-key>

type: Opaque
```

### Send form-data

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.webhook.form: |
    url: https://form.example.com
    headers:
    - name: Content-Type
      value: application/x-www-form-urlencoded

  template.form-data: |
    webhook:
      form:
        method: POST
        body: key1=value1&key2=value2
```

### Send Slack

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.webhook.slack_webhook: |
    url: https://hooks.slack.com/services/xxxxx
    headers:
    - name: Content-Type
      value: application/json

  template.send-slack: |
    webhook:
      slack_webhook:
        method: POST
        body: |
          {
            "attachments": [{
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
          }
```
