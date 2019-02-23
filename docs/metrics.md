# Prometheus Metrics

Argo CD exposes two sets of prometheus metrics

## Application Metrics
Metrics about applications. Scraped at the `argocd-metrics:8082/metrics` endpoint. 

* Gauge for application health status
* Gauge for application sync status
* Counter for application sync history

## API Server Metrics
Metrics about API Server API request and response activity (request totals, response codes, etc...).
Scraped at the `argocd-server-metrics:8083/metrics` endpoint.
