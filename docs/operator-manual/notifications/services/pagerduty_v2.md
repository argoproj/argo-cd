# PagerDuty V2

## Parameters

The PagerDuty notification service is used to trigger PagerDuty events and requires specifying the following settings:

* `serviceKeys` - a dictionary with the following structure:
  * `service-name: $pagerduty-key-service-name` where `service-name` is the name you want to use for the service to make events for, and `$pagerduty-key-service-name` is a reference to the secret that contains the actual PagerDuty integration key (Events API v2 integration)

If you want multiple Argo apps to trigger events to their respective PagerDuty services, create an integration key in each service you want to setup alerts for.

To create a PagerDuty integration key, [follow these instructions](https://support.pagerduty.com/docs/services-and-integrations#create-a-generic-events-api-integration) to add an Events API v2 integration to the service of your choice.

## Configuration

The following snippet contains sample PagerDuty service configuration. It assumes the service you want to alert on is called `my-service`.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  pagerduty-key-my-service: <pd-integration-key>
```

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.pagerdutyv2: |
    serviceKeys:
      my-service: $pagerduty-key-my-service
```

## Template

[Notification templates](../templates.md) support specifying subject for PagerDuty notifications:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  template.rollout-aborted: |
    message: Rollout {{.rollout.metadata.name}} is aborted.
    pagerdutyv2:
      summary: "Rollout {{.rollout.metadata.name}} is aborted."
      severity: "critical"
      source: "{{.rollout.metadata.name}}"
```

The parameters for the PagerDuty configuration in the template generally match with the payload for the Events API v2 endpoint. All parameters are strings.

* `summary` - (required) A brief text summary of the event, used to generate the summaries/titles of any associated alerts.
* `severity` - (required) The perceived severity of the status the event is describing with respect to the affected system. Allowed values: `critical`, `warning`, `error`, `info`
* `source` - (required) The unique location of the affected system, preferably a hostname or FQDN.
* `component` - Component of the source machine that is responsible for the event.
* `group` - Logical grouping of components of a service.
* `class` - The class/type of the event.
* `url` - The URL that should be used for the link "View in ArgoCD" in PagerDuty.

The `timestamp` and `custom_details` parameters are not currently supported.

## Annotation

Annotation sample for PagerDuty notifications:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-rollout-aborted.pagerdutyv2: "<serviceID for PagerDuty>"
```
