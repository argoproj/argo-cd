# Observability

Argo CD components expose Prometheus metrics and can export OpenTelemetry traces.

* [Metrics](metrics.md): the Prometheus metrics each component serves, and example dashboards.
* [Notifications monitoring](notifications/monitoring.md): metrics of the notifications controller.
* [CPU/Memory profiling](high_availability.md#cpumemory-profiling): the `pprof` endpoint on the
  metrics port.
* [OpenTelemetry tracing](#opentelemetry-tracing): described on this page.

## OpenTelemetry tracing

The following components can export traces to an OpenTelemetry collector over OTLP/gRPC:

| Component                    | `service.name`       |
| ---------------------------- | -------------------- |
| API server                   | `argocd-server`      |
| Application controller       | `argocd-controller`  |
| Repo server                  | `argocd-repo-server` |
| Config Management Plugin sidecar | `argocd-cmp-server`  |

gRPC calls between components carry W3C trace context, so a reconciliation that starts in the
application controller and generates manifests in the repo server is recorded as one trace.

### Enabling tracing

Set the collector address in `argocd-cmd-params-cm`. The API server, application controller and
repo server read these keys:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
  namespace: argocd
data:
  # Collector address. Tracing is disabled when empty.
  otlp.address: "otel-collector.observability:4317"
  # Set to "false" to connect over TLS. Defaults to "true" (plaintext).
  otlp.insecure: "false"
  # Extra headers sent with each export, as comma-separated key=value pairs.
  otlp.headers: ""
  # Extra resource attributes, as comma-separated key:value pairs.
  otlp.attrs: "deployment.environment:production"
  # Fraction of new traces to sample, between 0.0 and 1.0. Defaults to 1.0.
  otlp.sample.ratio: "0.1"
```

Restart the components after changing these keys. Each key can also be set per component with the
`--otlp-*` flags or the `ARGOCD_<COMPONENT>_OTLP_*` environment variables, where `<COMPONENT>` is
`SERVER`, `APPLICATION_CONTROLLER`, `REPO_SERVER` or `CMP_SERVER`. For example,
`ARGOCD_REPO_SERVER_OTLP_ADDRESS`.

Config Management Plugin sidecars do not read `argocd-cmd-params-cm`. Set the
`ARGOCD_CMP_SERVER_OTLP_*` environment variables on the sidecar container instead.

### Sampling

`otlp.sample.ratio` sets the fraction of new traces that are recorded. The sampler is parent-based:
when a request already carries a sampling decision in its trace context, that decision is used. A
trace started by the application controller is therefore recorded in full or not at all across the
repo server and plugins. Set the same ratio on every component to get a consistent rate.

### Collector TLS and authentication

With `otlp.insecure: "false"`, the collector certificate is verified against the first of the
following that is configured:

1. The standard `OTEL_EXPORTER_OTLP_CERTIFICATE`, `OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE` and
   `OTEL_EXPORTER_OTLP_CLIENT_KEY` environment variables. Use these for mTLS.
2. The CA configured for the collector host in `argocd-tls-certs-cm`. The port is ignored when
   matching the host.
3. The system root CAs.

To trust a private CA for the collector, add it to `argocd-tls-certs-cm` the same way as for a
repository:

```bash
argocd cert add-tls otel-collector.observability --from ca.crt
```

The CA is read at startup, so restart the components after adding it.

For collectors that authenticate with a static header, such as a bearer token or a vendor API key,
use `otlp.headers`, or `OTEL_EXPORTER_OTLP_HEADERS` when `otlp.headers` is empty. Because
`argocd-cmd-params-cm` is not a Secret, set the header from a Secret instead by overriding the
component's environment variable:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: argocd-otlp-auth
  namespace: argocd
stringData:
  headers: "authorization=Bearer <token>"
---
# Patch for the argocd-server Deployment. Repeat for the other components.
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-server
spec:
  template:
    spec:
      containers:
        - name: argocd-server
          env:
            - name: ARGOCD_SERVER_OTLP_HEADERS
              valueFrom:
                secretKeyRef:
                  name: argocd-otlp-auth
                  key: headers
```

Header pairs are split on the first `=`, so values may contain `=`, but not `,`.

For mTLS, mount the client certificate from a Secret and point the standard environment variables
at it:

```yaml
# Patch for the argocd-server Deployment. Repeat for the other components.
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-server
spec:
  template:
    spec:
      containers:
        - name: argocd-server
          env:
            - name: OTEL_EXPORTER_OTLP_CERTIFICATE
              value: /app/config/otlp/ca.crt
            - name: OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE
              value: /app/config/otlp/tls.crt
            - name: OTEL_EXPORTER_OTLP_CLIENT_KEY
              value: /app/config/otlp/tls.key
          volumeMounts:
            - name: otlp-client-tls
              mountPath: /app/config/otlp
              readOnly: true
      volumes:
        - name: otlp-client-tls
          secret:
            secretName: argocd-otlp-client-tls
```

Config Management Plugin sidecars do not mount `argocd-tls-certs-cm`. To trust a private CA from a
sidecar, mount `argocd-tls-certs-cm` at `/app/config/tls` in the sidecar container, or use the
environment variables above.

For credentials that must be signed or refreshed, such as AWS SigV4 or OAuth, send traces to an
OpenTelemetry Collector running in the cluster and configure authentication to the backend there.
