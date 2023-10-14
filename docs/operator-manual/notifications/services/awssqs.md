# AWS SQS 

## Parameters

This notification service is capable of sending simple messages to AWS SQS queue. 

* `queue` - name of the queue you are intending to send messages to. Can be overwriten with target destination annotation.
* `region` - region of the sqs queue can be provided via env variable AWS_DEFAULT_REGION
* `key` - optional, aws access key must be either referenced from a secret via variable or via env variable AWS_ACCESS_KEY_ID
* `secret` - optional, aws access secret must be either referenced from a secret via variableor via env variable AWS_SECRET_ACCESS_KEY
* `account` optional, external accountId of the queue
* `endpointUrl` optional, useful for development with localstack

## Example

### Using Secret for credential retrieval:

Resource Annotation:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  annotations:
    notifications.argoproj.io/subscribe.on-deployment-ready.awssqs: "overwrite-myqueue"
```

* ConfigMap
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.awssqs: |
    region: "us-east-2"
    queue: "myqueue"
    account: "1234567"
    key: "$awsaccess_key"
    secret: "$awsaccess_secret"

  template.deployment-ready: |
    message: |
      Deployment {{.obj.metadata.name}} is ready!

  trigger.on-deployment-ready: |
    - when: any(obj.status.conditions, {.type == 'Available' && .status == 'True'})
      send: [deployment-ready]
    - oncePer: obj.metadata.annotations["generation"]

```
 Secret
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  awsaccess_key: test
  awsaccess_secret: test
```


### Minimal configuration using AWS Env variables

Ensure following list of enviromental variable is injected via OIDC, or other method. And assuming SQS is local to the account.
You may skip usage of secret for sensitive data and omit other parameters. (Setting parameters via ConfigMap takes precedent.)

Variables:

```bash
export AWS_ACCESS_KEY_ID="test"
export AWS_SECRET_ACCESS_KEY="test"
export AWS_DEFAULT_REGION="us-east-1"
```

Resource Annotation:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  annotations:
    notifications.argoproj.io/subscribe.on-deployment-ready.awssqs: ""
```

* ConfigMap
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.awssqs: |
    queue: "myqueue"

  template.deployment-ready: |
    message: |
      Deployment {{.obj.metadata.name}} is ready!

  trigger.on-deployment-ready: |
    - when: any(obj.status.conditions, {.type == 'Available' && .status == 'True'})
      send: [deployment-ready]
    - oncePer: obj.metadata.annotations["generation"]

```
