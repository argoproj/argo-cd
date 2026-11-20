# Observability

Argo CD components expose Prometheus metrics and can export OpenTelemetry traces.

* [Metrics](metrics.md): the Prometheus metrics each component serves, and example dashboards.
* [Notifications monitoring](notifications/monitoring.md): metrics of the notifications controller.
* [CPU/Memory profiling](high_availability.md#cpumemory-profiling): the `pprof` endpoint on the
  metrics port.
* [OpenTelemetry tracing](#opentelemetry-tracing) and
  [pushing metrics via OTLP](#pushing-metrics-via-otlp): described on this page.

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
  # Set to "false" to connect over TLS. Defaults to "true" (plaintext), which ignores the
  # OTEL_EXPORTER_OTLP_* TLS environment variables.
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

### Application controller span attributes

`controller.Refresh` and `controller.Operation` each describe one workqueue processing attempt.
An operation that waits, resumes, or retries can produce multiple traces. Comparison and sync spans
record the work performed within an attempt. These attributes add metadata to the existing spans;
they do not change queue behavior or trace parentage.

| Attribute | Spans | Meaning |
| --- | --- | --- |
| `argocd.app.sync.status` | Refresh, CompareAppState | Final sync status observed by the comparison, or an explicit status snapshot when Refresh returns before comparison. |
| `argocd.app.health.status` | Refresh, CompareAppState | Health status observed by the same result or explicit snapshot. |
| `argocd.resolved_revision` | Refresh, CompareAppState, SyncAppState | Resolved revision for a single-source Application. |
| `argocd.resolved_revisions` | Refresh, CompareAppState, SyncAppState | Ordered string array of resolved revisions for a multi-source Application. |
| `argocd.operation.phase` | Operation | Operation phase retained by Kubernetes after the attempt, including retry handling or a size-limit fallback. Omitted when persistence fails and the retained phase is unknown. |
| `argocd.operation.attempt.phase` | Operation | Phase observed after sync work, before retry handling can reset it to `Running`. Absent for a retry-backoff dequeue that performs no sync work. |
| `argocd.operation.retry_count` | Operation | Retry count at the end of the attempt. |
| `argocd.operation.initiated_by.automated` | Operation | Whether the operation was initiated automatically. |
| `argocd.sync.dry_run` | Operation, SyncAppState | Whether the sync is a dry run. |
| `argocd.sync.prune` | Operation, SyncAppState | Whether pruning is enabled for the sync. |
| `argocd.refresh.type` | Refresh | Requested refresh type, such as `normal` or `hard`. |
| `argocd.refresh.comparison_level` | Refresh | `nothing`, `recent`, `latest`, or `latest_force_resolve`; `unknown` for an unrecognized level. |
| `argocd.reconciliation.cause` | Refresh, Operation | Bounded explanation available from controller state when the attempt starts. |
| `argocd.reconciliation.outcome` | Refresh, Operation | Bounded result of the attempt, described below. |
| `argocd.reconciliation.status_changed` | Refresh | Whether the sync and health snapshots changed, including revision and comparison inputs. Reconciliation timestamps do not count as changes. False when the attempt reports an error. |

Resolved revision attributes are emitted only after every source resolves successfully. Requested refs
such as `main` or `HEAD`, and partially resolved source sets, are not labeled as resolved aggregate
revisions. If comparison fails before producing a result, Refresh does not export the previous
Application status as the failed comparison's result.

#### Causes and outcomes

Refresh causes are `explicit_request`, `source_changed`, `refresh_expired`,
`hard_refresh_expired`, `destination_changed`, `managed_namespace_metadata_changed`,
`ignore_differences_changed`, and `controller_request`. The last value is coarse: the refresh queue
does not retain the specific event that requested the work.

Operation causes are `auto_sync`, `manual`, `resume`, `retry`, `termination`, and `unknown`.
Causes describe available controller state; they do not establish the identity of a triggering event.

Refresh outcomes are `state_changed`, `no_change`, and `error`. A state change does not imply that
the Application is healthy or that an operation has finished. Operation outcomes are `succeeded`,
`progressing`, `retry_wait`, `terminating`, `error`, and `unknown`.

The persisted phase, attempt phase, outcome, and span status answer different questions:

| Scenario | Persisted phase | Attempt phase | Outcome | OTel status |
| --- | --- | --- | --- | --- |
| Failed sync scheduled for retry | `Running` | `Failed` or `Error` | `error` | `Error` |
| Waiting for retry backoff | `Running` | Absent | `retry_wait` | `Unset` |
| Running sync observes a concurrent termination | `Terminating` | `Running` | `terminating` | `Unset` |
| Successful sync requires a size-limit fallback | `Terminating` | `Succeeded` | `terminating` | `Error` |

The attempt phase, retry count, and sync flags describe the attempted work, including when a reduced
fallback is persisted. `Unset` means the span was not marked as a controller error; it does not report
Application health.

#### Error status

An observed `OutOfSync` or `Degraded` Application is not by itself a controller execution error.
Comparison, cache, and application-status or operation-state persistence failures set the relevant
span's OpenTelemetry status to `Error`. Failed sync attempts also set error status, including attempts
scheduled for retry.

An application-status write failure also marks the parent `controller.Refresh` as an error, with
`argocd.reconciliation.outcome=error` and `argocd.reconciliation.status_changed=false`. This includes
size-limit failures where a reduced error condition is persisted instead of the requested status.

Failure to initiate auto-sync marks both `controller.autoSync` and its `controller.Refresh` parent as
errors. Historical failed-sync conditions and intentional policy skips are observations of controller
state and do not, by themselves, mark the current attempt as an error.

Refresh and sync panics are recorded before the span ends, preserving the controller's existing
recovery behavior. `SyncAppState` re-panics after finalizing its span.

These are Argo CD attributes, not standardized OpenTelemetry GitOps semantic conventions. Arbitrary
Application annotations, labels, operation info, and initiator usernames are not added to these spans.

## Pushing metrics via OTLP

Every component keeps serving its metrics at `/metrics` for scraping. For environments without a
Prometheus that can reach Argo CD, the API server, application controller and repo server can
additionally push the same metrics to an OpenTelemetry collector over OTLP/gRPC.

Pushing reuses the [tracing connection settings](#enabling-tracing), including
[collector TLS and authentication](#collector-tls-and-authentication), and is off by default. Set it
in `argocd-cmd-params-cm`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
  namespace: argocd
data:
  # Collector address; required for both tracing and metrics push.
  otlp.address: "otel-collector.observability:4317"
  otlp.insecure: "true"
  otlp.metrics.enabled: "true"
  # Push interval, minimum 1s. Defaults to 30s.
  otlp.metrics.interval: "30s"
```

The equivalent flags are `--otlp-metrics-enabled` and `--otlp-metrics-interval`, alongside the
existing `--otlp-address`, `--otlp-insecure`, `--otlp-headers` and `--otlp-attrs`.

> [!NOTE]
> Setting `otlp.address` also enables trace export. To push metrics without traces, set
> `otlp.sample.ratio: "0"`.

What is pushed is exactly the set of metric families served at `/metrics`, converted by the
OpenTelemetry Prometheus bridge with names unchanged. Each push carries the resource attributes
`service.name` (`argocd-server`, `argocd-controller` or `argocd-repo-server`),
`service.instance.id` (the pod hostname) and any pairs from `otlp.attrs`. Because pushed metrics
have no scrape target, `service.instance.id` is what keeps series from different replicas apart.

> [!WARNING]
> If a collector already scrapes `/metrics`, enabling push delivers every series twice: once with
> the scrape's `instance`/`job` labels and once with the push's resource attributes. Use one or
> the other per component.

A registry that fails to gather (for example, an inconsistent label set in
`argocd_app_labels`) is omitted from that push while the other registries are still sent. The
failure is logged at error level by the component.

## Collector TLS and authentication

These settings apply to both trace export and metrics push.


With `otlp.insecure: "false"`, Argo CD always connects over TLS. The collector certificate is
verified against the first of the following that is configured:

1. The CA in `OTEL_EXPORTER_OTLP_CERTIFICATE`, or its signal-specific variant such as
   `OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE`, which takes precedence.
2. The CA configured for the collector host in `argocd-tls-certs-cm`. The port is ignored when
   matching the host.
3. The system root CAs.

For mTLS, set `OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE` and `OTEL_EXPORTER_OTLP_CLIENT_KEY`, or their
signal-specific variants. The client certificate works with any of the CAs above. The certificate
and key are read as a pair: the signal-specific pair is used if both are set, otherwise the generic
pair. Empty values are ignored.

> [!NOTE]
> A component fails to start if a configured CA file cannot be read or contains no certificates,
> or if a client certificate or key is set without a complete pair.
> `OTEL_EXPORTER_OTLP_INSECURE` and `http://` endpoints do not override `otlp.insecure: "false"`.

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
# Strategic merge patch for the argocd-server Deployment, e.g. a Kustomize `patches` entry or
# `kubectl patch deployment argocd-server -n argocd --patch-file <file>`. Repeat for the other components.
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
# Strategic merge patch for the argocd-server Deployment, e.g. a Kustomize `patches` entry or
# `kubectl patch deployment argocd-server -n argocd --patch-file <file>`. Repeat for the other components.
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
