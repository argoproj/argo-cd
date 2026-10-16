# Grafana

To be able to create Grafana annotation with argocd-notifications you have to create a [service account token](https://grafana.com/docs/grafana/latest/administration/service-accounts/) inside your [Grafana](https://grafana.com).

![sample](https://user-images.githubusercontent.com/18019529/112024976-0f106080-8b78-11eb-9658-7663305899be.png)

Available parameters :

* `apiURL` - the server url, e.g. https://grafana.example.com
* `apiKey` - the API key for the serviceaccount
* `insecureSkipVerify` - optional bool, true or false
* `maxIdleConns` - optional, maximum number of idle (keep-alive) connections across all hosts.
* `maxIdleConnsPerHost` - optional, maximum number of idle (keep-alive) connections per host.
* `maxConnsPerHost` - optional, maximum total connections per host.
* `idleConnTimeout` - optional, maximum amount of time an idle (keep-alive) connection will remain open before closing.

1. Login to your Grafana instance as `admin`
2. On the left menu, go to Administration / Users and access / Service accounts
3. Click "Add service account", enter the display name `ArgoCD Notification` and click "Create"
4. Assign the `Editor` role to the service account with the role picker
5. Click "Add service account token", enter a name for the token, optionally set an expiration date, and click "Generate token"
6. Store the token in the `argocd-notifications-secret` Secret and reference it from the `apiKey` field in the `argocd-notifications-cm` ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.grafana: |
    apiUrl: https://grafana.example.com/api
    apiKey: $grafana-api-key
```

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  grafana-api-key: api-key
```

7. Create a template in `argo-notifications-cm` Configmap
This will be used to pass the (required) text of the annotation to Grafana (or re-use an existing one)
As there is no specific template for Grafana, you must use the generic `message`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  templates:
    template.app-deployed: |
      message: Application {{.app.metadata.name}} is now running new version of deployments manifests.
```

8. Create subscription for your Grafana integration

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.<trigger-name>.grafana: tag1|tag2 # list of tags separated with |
```

9. Change the annotations settings
![8](https://user-images.githubusercontent.com/18019529/112022083-47fb0600-8b75-11eb-849b-d25d41925909.png)