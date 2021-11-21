The trigger defines the condition when the notification should be sent. The definition includes name, condition
and notification templates reference. The condition is a predicate expression that returns true if the notification
should be sent. The trigger condition evaluation is powered by [antonmedv/expr](https://github.com/antonmedv/expr).
The condition language syntax is described at [Language-Definition.md](https://github.com/antonmedv/expr/blob/master/docs/Language-Definition.md).

The trigger is configured in `argocd-notifications-cm` ConfigMap. For example the following trigger sends a notification
when application sync status changes to `Unknown` using the `app-sync-status` template:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  trigger.on-sync-status-unknown: |
    - when: app.status.sync.status == 'Unknown'     # trigger condition
      send: [app-sync-status, github-commit-status] # template names
```

Each condition might use several templates. Typically each template is responsible for generating a service-specific notification part.
In the example above `app-sync-status` template "knows" how to create email and slack notification and `github-commit-status` knows how to
generate payload for Github webhook.

## Conditions Bundles

Triggers are typically managed by administrators and encapsulate information about when and which notification should be sent.
The end users just need to subscribe to the trigger and specify the notification destination. In order to improve user experience
triggers might include multiple conditions with a different set of templates for each condition. For example, the following trigger
covers all stages of sync status operation and use a different template for different cases:


```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  trigger.sync-operation-change: |
    - when: app.status.operationState.phase in ['Succeeded']
      send: [github-commit-status]
    - when: app.status.operationState.phase in ['Running']
      send: [github-commit-status]
    - when: app.status.operationState.phase in ['Error', 'Failed']
      send: [app-sync-failed, github-commit-status]
```

## Avoid Sending Same Notification Too Often

In some cases, the trigger condition might be "flapping". The example below illustrates the problem.
The trigger is supposed to generate a notification once when Argo CD application is successfully synchronized and healthy.
However, the application health status might intermittently switch to `Progressing` and then back to `Healthy` so the trigger might unnecessarily generate
multiple notifications. The `oncePer` field configures triggers to generate the notification only when the corresponding application field changes.
The `on-deployed` trigger from the example below sends the notification only once per observed Git revision of the deployment repository.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  # Optional 'oncePer' property ensure that notification is sent only once per specified field value
  # E.g. following is triggered once per sync revision
  trigger.on-deployed: |
    when: app.status.operationState.phase in ['Succeeded'] and app.status.health.status == 'Healthy'
    oncePer: app.status.sync.revision
    send: [app-sync-succeeded]
```

### oncePer

The `oncePer` filed is supported like as follows.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    example.com/version: v0.1
```

```yaml
oncePer: app.metadata.annotations["example.com/version"]
```

## Default Triggers

You can use `defaultTriggers` field instead of specifying individual triggers to the annotations.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  # Holds list of triggers that are used by default if trigger is not specified explicitly in the subscription
  defaultTriggers: |
    - on-sync-status-unknown

  defaultTriggers.mattermost: |
    - on-sync-running
    - on-sync-succeeded
```

Specify the annotations as follows to use `defaultTriggers`. In this example, `slack` sends when `on-sync-status-unknown`, and `mattermost` sends when `on-sync-running` and `on-sync-succeeded`.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.slack: my-channel
    notifications.argoproj.io/subscribe.mattermost: my-mattermost-channel
```

## Functions

Triggers have access to the set of built-in functions.

Example:

```yaml
when: time.Now().Sub(time.Parse(app.status.operationState.startedAt)).Minutes() >= 5
```

{!functions.md!}
