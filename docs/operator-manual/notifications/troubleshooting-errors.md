## Failed to parse new settings

### error converting YAML to JSON

YAML syntax is incorrect.

**incorrect:**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.slack: |
    token: $slack-token
    icon: :rocket:
```

**correct:**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.slack: |
    token: $slack-token
    icon: ":rocket:"
```

### service type 'xxxx' is not supported

You need to check your argocd-notifications controller version. For instance, the teams integration is to support `v1.1.0` and more.

## Failed to notify recipient

### notification service 'xxxx' is not supported"

You have not defined `xxxx` in `argocd-notifications-cm` or to fail to parse settings.
