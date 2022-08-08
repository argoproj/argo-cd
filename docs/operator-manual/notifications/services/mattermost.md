# Mattermost

## Parameters

* `apiURL` - the server url, e.g. https://mattermost.example.com
* `token` - the bot token
* `insecureSkipVerify` - optional bool, true or false

## Configuration

1. Create a bot account and copy token after creating it
![1](https://user-images.githubusercontent.com/18019529/111499520-62ed0500-8786-11eb-88b0-d0aade61fed4.png)
2. Invite team
![2](https://user-images.githubusercontent.com/18019529/111500197-1229dc00-8787-11eb-98e5-587ee36c94a9.png)
3. Store token in `argocd-notifications-secret` Secret and configure Mattermost integration
in `argocd-notifications-cm` ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.mattermost: |
    apiURL: <api-url>
    token: $mattermost-token
```

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  mattermost-token: token
```

4. Copy channel id
![4](https://user-images.githubusercontent.com/18019529/111501289-333efc80-8788-11eb-9731-8353170cd73a.png)

5. Create subscription for your Mattermost integration

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.<trigger-name>.mattermost: <channel-id>
```

## Templates

![](https://user-images.githubusercontent.com/18019529/111502636-5fa74880-8789-11eb-97c5-5eac22c00a37.png)

You can reuse the template of slack.  
Mattermost is compatible with attachments of Slack. See [Mattermost Integration Guide](https://docs.mattermost.com/developer/message-attachments.html).

```yaml
template.app-deployed: |
  message: |
    Application {{.app.metadata.name}} is now running new version of deployments manifests.
  mattermost:
    attachments: |
      [{
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
```
