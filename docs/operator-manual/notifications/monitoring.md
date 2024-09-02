# Monitoring

The Argo CD Notification controller serves Prometheus metrics on port 9001.

!!! note
    The metrics port can be changed using the `--metrics-port` flag in `argocd-notifications-controller` deployment.

## Metrics 
The following metrics are available:
 
### `argocd_notifications_deliveries_total`
  
 Number of delivered notifications.
 Labels:

* `trigger` - trigger name
* `service` - notification service name
* `succeeded` - flag that indicates if notification was successfully sent or failed

### `argocd_notifications_trigger_eval_total`
  
 Number of trigger evaluations.
 Labels:

* `name` - trigger name 
* `triggered` - flag that indicates if trigger condition returned true of false

## Examples

* Grafana Dashboard: [grafana-dashboard.json](grafana-dashboard.json)
