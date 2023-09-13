# Grafana

To be able to create Grafana annotation with argocd-notifications you have to create an [API Key](https://grafana.com/docs/grafana/latest/http_api/auth/#create-api-key) inside your [Grafana](https://grafana.com).

![sample](https://user-images.githubusercontent.com/18019529/112024976-0f106080-8b78-11eb-9658-7663305899be.png)

1. Login to your Grafana instance as `admin`
2. On the left menu, go to Configuration / API Keys
3. Click "Add API Key" 
4. Fill the Key with name `ArgoCD Notification`, role `Editor` and Time to Live `10y` (for example)
5. Click on Add button
6. Store apiKey in `argocd-notifications-secret` Secret and Copy your API Key and define it in `argocd-notifications-cm` ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
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

7. Create subscription for your Grafana integration

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.<trigger-name>.grafana: tag1|tag2 # list of tags separated with |
```

8. Change the annotations settings
![8](https://user-images.githubusercontent.com/18019529/112022083-47fb0600-8b75-11eb-849b-d25d41925909.png)
