# GCP Pub/Sub

## Parameters

This notification service is capable of sending messages to Google Cloud Pub/Sub topics.

* `project` - GCP project ID where the Pub/Sub topic is located.
* `topic` - name of the Pub/Sub topic you are intending to send messages to. Can be overridden with target destination annotation.
* `keyFile` - optional, path to GCP service account key file. If not provided, uses Application Default Credentials

## Example

### Using Service Account Key File

Resource Annotation:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  annotations:
    notifications.argoproj.io/subscribe.on-deployment-ready.gcppubsub: "override-topic"
```

ConfigMap:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.gcppubsub: |
    project: "my-gcp-project"
    topic: "my-topic"
    keyFile: "/var/secrets/gcp/key.json"

  template.deployment-ready: |
    message: |
      Deployment {{.obj.metadata.name}} is ready!
    gcppubsub:
      attributes:
        app: "{{.obj.metadata.name}}"
        environment: "production"

  trigger.on-deployment-ready: |
    - when: any(obj.status.conditions, {.type == 'Available' && .status == 'True'})
      send: [deployment-ready]
    - oncePer: obj.metadata.annotations["generation"]
```

### Using Workload Identity (GKE)

When running on GKE with Workload Identity enabled, you can omit the `keyFile` parameter and the service will use the default credentials from the service account bound to the Kubernetes service account.

Resource Annotation:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  annotations:
    notifications.argoproj.io/subscribe.on-deployment-ready.gcppubsub: ""
```

ConfigMap:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.gcppubsub: |
    project: "my-gcp-project"
    topic: "my-topic"

  template.deployment-ready: |
    message: |
      Deployment {{.obj.metadata.name}} is ready!
    gcppubsub:
      attributes:
        app: "{{.obj.metadata.name}}"
        status: "{{.obj.status.conditions[0].type}}"

  trigger.on-deployment-ready: |
    - when: any(obj.status.conditions, {.type == 'Available' && .status == 'True'})
      send: [deployment-ready]
    - oncePer: obj.metadata.annotations["generation"]
```

## Message Attributes

Pub/Sub messages support custom attributes (key-value pairs) that can be used for filtering or routing. You can include attributes in your template:

```yaml
template.deployment-ready: |
  message: |
    Deployment {{.obj.metadata.name}} is ready!
  gcppubsub:
    attributes:
      severity: "info"
      app: "{{.obj.metadata.name}}"
      namespace: "{{.obj.metadata.namespace}}"
      timestamp: "{{(call .time.Now).Unix}}"
```

Attributes support template variables just like the message body, allowing you to dynamically populate metadata from the object being monitored.
