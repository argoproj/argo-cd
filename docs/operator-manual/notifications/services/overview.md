The notification services represent integration with services such as slack, email or custom webhook. Services are configured in `argocd-notifications-cm` ConfigMap
using `service.<type>.(<custom-name>)` keys and might reference sensitive data from `argocd-notifications-secret` Secret. Following example demonstrates slack
service configuration:

```yaml
  service.slack: |
    token: $slack-token
```


The `slack` indicates that service sends slack notification; name is missing and defaults to `slack`.

## Sensitive Data

Sensitive data like authentication tokens should be stored in `<secret-name>` Secret and can be referenced in
service configuration using `$<secret-key>` format. For example `$slack-token` referencing value of key `slack-token` in
`<secret-name>` Secret.

## Custom Names

Service custom names allow configuring two instances of the same service type.

```yaml
  service.slack.workspace1: |
    token: $slack-token-workspace1
  service.slack.workspace2: |
    token: $slack-token-workspace2
```

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-sync-succeeded.workspace1: my-channel
    notifications.argoproj.io/subscribe.on-sync-succeeded.workspace2: my-channel
```

## Service Types

* [AwsSqs](./awssqs.md)
* [Email](./email.md)
* [GitHub](./github.md)
* [Slack](./slack.md)
* [Mattermost](./mattermost.md)
* [Opsgenie](./opsgenie.md)
* [Grafana](./grafana.md)
* [Webhook](./webhook.md)
* [Telegram](./telegram.md)
* [Teams](./teams.md)
* [Google Chat](./googlechat.md)
* [Rocket.Chat](./rocketchat.md)
* [Pushover](./pushover.md)
* [Alertmanager](./alertmanager.md)