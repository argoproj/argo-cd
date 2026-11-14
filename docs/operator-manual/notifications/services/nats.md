# Nats

## Parameters

This notification service is capable of sending simple messages via Nats.

* Url - Nats server URL, e.g. `nats://nats:4222`
* Headers - optional, additional headers to be sent with the message
* User - optional, Nats user for authentication used in combination with password
* Password - optional, Nats password for authentication used in combination with user
* Nkey - optional, Nats key for authentication 

## Example

Resource Annotation:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  annotations:
    notifications.argoproj.io/subscribe.on-deployment-ready.nats: "mytopic"
```

* ConfigMap
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.nats: |
    url: "nats://nats:4222"
    headers:
      my-header: "my-value"

template.deployment-ready: |
    message: |
      Deployment {{.obj.metadata.name}} is ready!

  trigger.on-deployment-ready: |
    - when: any(obj.status.conditions, {.type == 'Available' && .status == 'True'})
      send: [deployment-ready]
    - oncePer: obj.metadata.annotations["generation"]

```



