# NewRelic

## Parameters

* `apiURL` - the api server url, e.g. https://api.newrelic.com
* `apiKey` - a [NewRelic ApiKey](https://docs.newrelic.com/docs/apis/rest-api-v2/get-started/introduction-new-relic-rest-api-v2/#api_key)

## Configuration

1. Create a NewRelic [Api Key](https://docs.newrelic.com/docs/apis/intro-apis/new-relic-api-keys/#user-api-key)
2. Store apiKey in `argocd-notifications-secret` Secret and configure NewRelic integration in `argocd-notifications-cm` ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.newrelic: |
    apiURL: <api-url>
    apiKey: $newrelic-apiKey
```

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  newrelic-apiKey: apiKey
```

3. Copy [Application ID](https://docs.newrelic.com/docs/apis/rest-api-v2/get-started/get-app-other-ids-new-relic-one/#apm)
4. Create subscription for your NewRelic integration

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.<trigger-name>.newrelic: <app-id>
```

## Templates

* `description` - __optional__, high-level description of this deployment, visible in the [Summary](https://docs.newrelic.com/docs/apm/applications-menu/monitoring/apm-overview-page) page and on the [Deployments](https://docs.newrelic.com/docs/apm/applications-menu/events/deployments-page) page when you select an individual deployment.
  * Defaults to `message`
* `changelog` - __optional__, A summary of what changed in this deployment, visible in the [Deployments](https://docs.newrelic.com/docs/apm/applications-menu/events/deployments-page) page when you select (selected deployment) > Change log.
  * Defaults to `{{(call .repo.GetCommitMetadata .app.status.sync.revision).Message}}`
* `user` - __optional__, A username to associate with the deployment, visible in the [Summary](https://docs.newrelic.com/docs/apm/applications-menu/events/deployments-page) and on the [Deployments](https://docs.newrelic.com/docs/apm/applications-menu/events/deployments-page).
  * Defaults to `{{(call .repo.GetCommitMetadata .app.status.sync.revision).Author}}`

```yaml
context: |
  argocdUrl: https://example.com/argocd

template.app-deployed: |
  message: Application {{.app.metadata.name}} has successfully deployed.
  newrelic:
    description: Application {{.app.metadata.name}} has successfully deployed
```
