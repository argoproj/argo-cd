# Metrics

Argo CD exposes two sets of Prometheus metrics

## Application Metrics
Metrics about applications. Scraped at the `argocd-metrics:8082/metrics` endpoint.

* `argocd_app_info`: Information about Applications. It contains labels such as `sync_status` and `health_status` that reflect the application state in ArgoCD.
* `argocd_app_sync_total`: Counter for application sync history
* `argocd_app_k8s_request_total`: Number of kubernetes requests executed during application reconciliation
* `argocd_kubectl_exec_total`: Number of kubectl executions
* `argocd_kubectl_exec_pending`: Number of pending kubectl executions
* `argocd_app_reconcile`: Application reconciliation performance.
* `argocd_cluster_events_total`: Number of processes k8s resource events.
* `argocd_redis_request_total`: Number of redis requests executed during application reconciliation
* `argocd_redis_request_duration`: Redis requests duration.
* `argocd_app_labels`: Argo Application labels converted to Prometheus labels. Disabled by default. See section bellow about how to enable it.

If you use ArgoCD with many application and project creation and deletion,
the metrics page will keep in cache your application and project's history.
If you are having issues because of a large number of metrics cardinality due
to deleted resources, you can schedule a metrics reset to clean the
history with an application controller flag. Example:
`--metrics-cache-expiration="24h0m0s"`.

### Exposing Application labels as Prometheus metrics

There are use-cases where ArgoCD Applications contain labels that are desired to be exposed as Prometheus metrics.
Some examples are:

* Having the team name as a label to allow routing alerts to specific receivers
* Creating dashboards broken down by business units

As the Application labels are specific to each company, this feature is disabled by default. To enable it, add the
`--metrics-application-labels` flag to the ArgoCD application controller.

The example bellow will expose the ArgoCD Application labels `team-name` and `business-unit` to Prometheus:

    containers:
    - command:
      - argocd-application-controller
      - --metrics-application-labels
      - team-name
      - --metrics-application-labels
      - business-unit

In this case, the metric would look like:

```
# TYPE argocd_app_labels gauge
argocd_app_labels{label_business_unit="bu-id-1",label_team_name="my-team",name="my-app-1",namespace="argocd",project="important-project"} 1
argocd_app_labels{label_business_unit="bu-id-1",label_team_name="my-team",name="my-app-2",namespace="argocd",project="important-project"} 1
argocd_app_labels{label_business_unit="bu-id-2",label_team_name="another-team",name="my-app-3",namespace="argocd",project="important-project"} 1
```

## API Server Metrics
Metrics about API Server API request and response activity (request totals, response codes, etc...).
Scraped at the `argocd-server-metrics:8083/metrics` endpoint.

## Prometheus Operator

If using Prometheus Operator, the following ServiceMonitor example manifests can be used.
Change `metadata.labels.release` to the name of label selected by your Prometheus.

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: argocd-metrics
  labels:
    release: prometheus-operator
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: argocd-metrics
  endpoints:
  - port: metrics
```

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: argocd-server-metrics
  labels:
    release: prometheus-operator
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: argocd-server-metrics
  endpoints:
  - port: metrics
```

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: argocd-repo-server-metrics
  labels:
    release: prometheus-operator
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: argocd-repo-server
  endpoints:
  - port: metrics
```

## Dashboards

You can find an example Grafana dashboard [here](https://github.com/argoproj/argo-cd/blob/master/examples/dashboard.json) or check demo instance
[dashboard](https://grafana.apps.argoproj.io).

![dashboard](../assets/dashboard.jpg)
