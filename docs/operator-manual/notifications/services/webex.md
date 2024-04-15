# Webex Teams

## Parameters

The Webex Teams notification service configuration includes following settings:

* `token` - the app token

## Configuration

1. Create a Webex [Bot](https://developer.webex.com/docs/bots)
1. Copy the bot access [token](https://developer.webex.com/my-apps) and store it in the `argocd-notifications-secret` Secret and configure Webex Teams integration in `argocd-notifications-cm` ConfigMap

    ``` yaml
    apiVersion: v1
    kind: Secret
    metadata:
    name: <secret-name>
    stringData:
    webex-token: <bot access token>
    ```

    ``` yaml
    apiVersion: v1
    kind: ConfigMap
    metadata:
    name: argocd-notifications-cm
    data:
    service.webex: |
        token: $webex-token
    ```

1. Create subscription for your Webex Teams integration

    ``` yaml
    apiVersion: argoproj.io/v1alpha1
    kind: Application
    metadata:
    annotations:
        notifications.argoproj.io/subscribe.<trigger-name>.webex: <personal email or room id>
    ```
