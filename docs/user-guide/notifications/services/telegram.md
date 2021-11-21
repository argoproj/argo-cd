# Telegram

1. Get an API token using [@Botfather](https://t.me/Botfather).
2. Store token in `<secret-name>` Secret and configure telegram integration
in `<config-map-name>` ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.telegram: |
    token: $telegram-token
```

3. Create new Telegram [channel](https://telegram.org/blog/channels).
4. Add your bot as an administrator.
5. Use this channel `username` (public channel) or `chatID` (private channel) in the subscription for your Telegram integration:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-sync-succeeded.telegram: username
```

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-sync-succeeded.telegram: -1000000000000
```
