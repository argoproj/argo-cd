# Webhook

The webhook notification service allows sending a generic HTTP request using the templatized request body and URL.
Using Webhook you might trigger a Jenkins job, update GitHub commit status.

## Parameters

The Webhook notification service configuration includes following settings:

- `url` - the url to send the webhook to
- `headers` - optional, the headers to pass along with the webhook
- `basicAuth` - optional, the basic authentication to pass along with the webhook
- `insecureSkipVerify` - optional bool, true or false
- `retryWaitMin` - Optional, the minimum wait time between retries. Default value: 1s.
- `retryWaitMax` - Optional, the maximum wait time between retries. Default value: 5s.
- `retryMax` - Optional, the maximum number of retries. Default value: 3.

## Retry Behavior

The webhook service will automatically retry the request if it fails due to network errors or if the server returns a 5xx status code. The number of retries and the wait time between retries can be configured using the `retryMax`, `retryWaitMin`, and `retryWaitMax` parameters.

The wait time between retries is between `retryWaitMin` and `retryWaitMax`. If all retries fail, the `Send` method will return an error.

## Configuration

Use the following steps to configure webhook:

1 Register webhook in `argocd-notifications-cm` ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.webhook.<webhook-name>: |
    url: https://<hostname>/<optional-path>
    headers: #optional headers
    - name: <header-name>
      value: <header-value>
    basicAuth: #optional username password
      username: <username>
      password: <api-key>
    insecureSkipVerify: true #optional bool
```

2 Define template that customizes webhook request method, path and body:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
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

### Set GitHub commit status

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
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
  name: argocd-notifications-cm
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
  name: argocd-notifications-cm
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
  name: argocd-notifications-cm
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
  name: argocd-notifications-cm
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
