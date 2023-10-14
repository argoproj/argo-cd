# Alertmanager

## Parameters

The notification service is used to push events to [Alertmanager](https://github.com/prometheus/alertmanager), and the following settings need to be specified:

* `targets` - the alertmanager service address, array type
* `scheme` - optional, default is "http", e.g. http or https
* `apiPath` - optional, default is "/api/v2/alerts"
* `insecureSkipVerify` - optional, default is "false", when scheme is https whether to skip the verification of ca
* `basicAuth` - optional, server auth
* `bearerToken` - optional, server auth
* `timeout` - optional, the timeout in seconds used when sending alerts, default is "3 seconds"

`basicAuth` or `bearerToken` is used for authentication, you can choose one. If the two are set at the same time, `basicAuth` takes precedence over `bearerToken`.

## Example

### Prometheus Alertmanager config

```yaml
global:
  resolve_timeout: 5m

route:
  group_by: ['alertname']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'default'
receivers:
- name: 'default'
  webhook_configs:
  - send_resolved: false
    url: 'http://10.5.39.39:10080/api/alerts/webhook'
```

You should turn off "send_resolved" or you will receive unnecessary recovery notifications after "resolve_timeout".

### Send one alertmanager without auth

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.alertmanager: |
    targets:
    - 10.5.39.39:9093
```

### Send alertmanager cluster with custom api path

If your alertmanager has changed the default api, you can customize "apiPath".

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.alertmanager: |
    targets:
    - 10.5.39.39:443
    scheme: https
    apiPath: /api/events
    insecureSkipVerify: true
```

### Send high availability alertmanager with auth

Store auth token in `argocd-notifications-secret` Secret and use configure in `argocd-notifications-cm` ConfigMap.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  alertmanager-username: <username>
  alertmanager-password: <password>
  alertmanager-bearer-token: <token>
```

- with basicAuth

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.alertmanager: |
    targets:
    - 10.5.39.39:19093
    - 10.5.39.39:29093
    - 10.5.39.39:39093
    scheme: https
    apiPath: /api/v2/alerts
    insecureSkipVerify: true
    basicAuth:
      username: $alertmanager-username
      password: $alertmanager-password   
```

- with bearerToken

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.alertmanager: |
    targets:
    - 10.5.39.39:19093
    - 10.5.39.39:29093
    - 10.5.39.39:39093
    scheme: https
    apiPath: /api/v2/alerts
    insecureSkipVerify: true
    bearerToken: $alertmanager-bearer-token
```

## Templates

* `labels` - at least one label pair required, implement different notification strategies according to alertmanager routing
* `annotations` - optional, specifies a set of information labels, which can be used to store longer additional information, but only for display
* `generatorURL` - optional, default is '{{.app.spec.source.repoURL}}', backlink used to identify the entity that caused this alert in the client

the `label` or `annotations` or `generatorURL` values can be templated.

```yaml
context: |
  argocdUrl: https://example.com/argocd

template.app-deployed: |
  message: Application {{.app.metadata.name}} has been healthy.
  alertmanager:
    labels:
      fault_priority: "P5"
      event_bucket: "deploy"
      event_status: "succeed"
      recipient: "{{.recipient}}"
    annotations:
      application: '<a href="{{.context.argocdUrl}}/applications/{{.app.metadata.name}}">{{.app.metadata.name}}</a>'
      author: "{{(call .repo.GetCommitMetadata .app.status.sync.revision).Author}}"
      message: "{{(call .repo.GetCommitMetadata .app.status.sync.revision).Message}}"
```

You can do targeted push on [Alertmanager](https://github.com/prometheus/alertmanager) according to labels.

```yaml
template.app-deployed: |
  message: Application {{.app.metadata.name}} has been healthy.
  alertmanager:
    labels:
      alertname: app-deployed
      fault_priority: "P5"
      event_bucket: "deploy"
```

There is a special label `alertname`. If you don’t set its value, it will be equal to the template name by default.