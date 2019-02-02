# Prometheus Metrics

Argo CD exposes prometheus metrics about applications and can be scraped at the
`argocd-metrics:8082/metrics` endpoint. Currently, the following metrics are exposed:

* Gauge for application health status
* Gauge for application sync status

Future metrics:
* Counter for application sync activity
