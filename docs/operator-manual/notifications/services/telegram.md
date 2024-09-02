# Telegram

1. Get an API token using [@Botfather](https://t.me/Botfather).
2. Store token in `<secret-name>` Secret and configure telegram integration
in `argocd-notifications-cm` ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
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

If your private chat contains threads, you can optionally specify a thread id by seperating it with a `|`:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-sync-succeeded.telegram: -1000000000000|2
```
