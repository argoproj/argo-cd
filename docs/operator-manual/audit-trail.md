# Audit Trail

Argo CD can record an audit trail of **who did what**: every change made through the API, CLI or web UI, every web
terminal session, and every operation Argo CD's controllers carry out by themselves. The trail is written by a dedicated
component, the `argocd-audit-controller`, as one JSON line per action on its standard output, so it lives in that pod's
log and can be shipped to your log pipeline like any other log.

> [!NOTE]
> The audit trail is disabled by default. Nothing changes until `server.audit.controller.address` is set.

## What is recorded

| Source | Recorded | Actor |
|--------|----------|-------|
| API server (`argocd-server`) | Every mutating API call (create, update, patch, delete, sync, rollback, terminate, run action, password updates, token creation, cache invalidation...), whether it succeeded or failed, including calls rejected by authentication or RBAC | The Argo CD user that made the call |
| API server | Logins, successful and failed | The user logging in |
| API server | Web terminal sessions: opened, refused, and ended (with duration) | The Argo CD user |
| Argo CD controllers (Kubernetes Events) | Operations started and completed (e.g. automated syncs and their results), resource actions | `system:<controller>`, or the user who initiated the operation |

Read-only calls (get, list, watch, logs...) are not recorded.

The actor is the **Argo CD user**, taken from the same token claims RBAC uses:

* local accounts, such as `admin`, are recorded by name, with the issuer `argocd`;
* SSO users are recorded by their email claim, with their subject, issuer and groups;
* project role tokens are recorded as `proj:<project>:<role>`;
* calls with missing, invalid or expired credentials are recorded as `unauthenticated`.

Each record also has the client's source IP and user agent. The source IP honours `server.trusted.proxies` and
`server.client.ip.header`, exactly as source IP logging does, so `X-Forwarded-For` headers sent by clients themselves are
ignored.

Only an allow-list of non-sensitive request fields is copied into a record (target object, revision, prune/dry-run flags,
action names, resource identifiers...). Passwords, tokens, credentials, manifests and patches never are.

### Record format

```json
{
  "kind": "AuditRecord",
  "id": "5b7e3a0c-2f0e-4a53-9b0c-2a8d1f6f3e7a",
  "timestamp": "2026-09-27T05:32:36Z",
  "receivedAt": "2026-09-27T05:32:36.412Z",
  "source": "argocd-server",
  "actor": {
    "username": "alice@example.com",
    "subject": "CgVhbGljZRIEbGRhcA",
    "issuer": "https://argocd.example.com/api/dex",
    "groups": ["platform-team"],
    "sourceIP": "203.0.113.9",
    "userAgent": "Mozilla/5.0 (Macintosh)"
  },
  "action": "application.sync",
  "verb": "sync",
  "method": "/application.ApplicationService/Sync",
  "resource": {
    "type": "application",
    "name": "guestbook",
    "namespace": "argocd"
  },
  "details": {
    "revision": "HEAD",
    "prune": "true"
  },
  "result": {
    "status": "success",
    "code": "OK"
  },
  "durationMillis": 42
}
```

`result.status` is `success` or `failure`. For API calls `result.code` is the gRPC status code (`OK`,
`PermissionDenied`, `Unauthenticated`, `InvalidArgument`...). For records derived from Kubernetes Events it is the
event reason.

## Architecture

```
            ┌───────────────┐   mutating API calls,   ┌─────────────────────────┐
 users ───► │ argocd-server │ ──── terminal sessions ─►│ argocd-audit-controller │──► stdout (pod log)
            └───────────────┘   (HTTP, shared token)  │                         │──► optional file
            ┌──────────────────────────────┐          │                         │──► /api/v1/records
            │ argocd-application-controller│ ─events─►│                         │──► /metrics
            └──────────────────────────────┘          └─────────────────────────┘
```

The API server buffers records in memory and ships them to the audit controller in batches, so auditing never slows
down or blocks API calls. If the audit controller cannot be reached, the API server retries, then writes the records
to its own log with the message `failed to deliver audit record to audit controller`, so they are not lost.

## Enabling the audit trail

1. Create the token the API server and the audit controller authenticate each other with:

    ```shell
    kubectl -n argocd create secret generic argocd-audit-token --from-literal=token=$(openssl rand -hex 32)
    ```

2. Install Argo CD with the audit controller. `manifests/install-with-audit.yaml` is `install.yaml` plus the audit
   controller, with `server.audit.controller.address` already set:

    ```shell
    kubectl apply -n argocd --server-side --force-conflicts -f manifests/install-with-audit.yaml
    ```

    With Kustomize, add `manifests/base/audit-controller` to your resources and set the address in `argocd-cmd-params-cm`:

    ```yaml
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: argocd-cmd-params-cm
    data:
      server.audit.controller.address: "argocd-audit-controller:8088"
    ```

3. Restart `argocd-server` if it was already running.

## Reading the audit trail

The audit trail is the audit controller's log. Operational messages go to standard error and records to standard
output; every record has `"kind": "AuditRecord"`:

```shell
kubectl -n argocd logs deploy/argocd-audit-controller | jq 'select(.kind == "AuditRecord")'
```

Who deleted what in the last day:

```shell
kubectl -n argocd logs deploy/argocd-audit-controller --since=24h \
  | jq -r 'select(.kind == "AuditRecord" and .verb == "delete") | [.timestamp, .actor.username, .resource.type, .resource.name, .result.status] | @tsv'
```

The audit controller also keeps the most recent records (1000 by default) in memory and serves them on
`/api/v1/records`. The endpoint requires the audit token and accepts the filters `user`, `action`, `verb`,
`resourceType`, `resourceName`, `project`, `result`, `since` (a duration such as `2h`, or an RFC 3339 time) and
`limit`:

```shell
kubectl -n argocd port-forward svc/argocd-audit-controller 8088 &
TOKEN=$(kubectl -n argocd get secret argocd-audit-token -o jsonpath='{.data.token}' | base64 -d)
curl -s -H "Authorization: Bearer $TOKEN" 'localhost:8088/api/v1/records?user=alice&since=2h' | jq .
```

> [!WARNING]
> The pod log is only kept as long as your cluster keeps container logs. For compliance purposes, ship the audit
> controller's log to durable storage, or mount a persistent volume at `/app/audit` and set
> `auditcontroller.log.file: /app/audit/audit.log` in `argocd-cmd-params-cm` to keep an additional copy on disk.

## Configuration

The audit controller is configured through `argocd-cmd-params-cm`:

| Key | Default | Description |
|-----|---------|-------------|
| `server.audit.controller.address` | (none) | Address of the audit controller. Setting it enables the audit trail in `argocd-server`. |
| `auditcontroller.log.format` | `json` | Format of the audit controller's operational log (`json` or `text`). Records are always JSON. |
| `auditcontroller.log.level` | `info` | Log level of the audit controller. |
| `auditcontroller.buffer.size` | `1000` | Number of recent records served on `/api/v1/records`. |
| `auditcontroller.log.file` | (none) | Also append records to this file. |
| `auditcontroller.watch.events` | `true` | Record the Kubernetes Events emitted by Argo CD controllers. |
| `auditcontroller.event.namespaces` | the controller's namespace | Namespaces to watch events in. Add the namespaces of [applications in any namespace](app-any-namespace.md). |
| `auditcontroller.event.reasons` | `OperationStarted,OperationCompleted,ResourceCreated,ResourceDeleted,ResourceActionRan` | Event reasons turned into records. |

> [!NOTE]
> Watching events in namespaces other than the audit controller's own requires granting it `get`, `list` and `watch`
> on `events` in those namespaces.

## Metrics

The audit controller exposes Prometheus metrics on port 8089:

| Metric | Description |
|--------|-------------|
| `argocd_audit_records_total{source, action, result}` | Records written. |
| `argocd_audit_record_write_errors_total` | Records that could not be written to every output. |
| `argocd_audit_ingest_rejected_total{reason}` | Rejected ingest requests (`unauthorized`, `malformed`, `too_large`, `too_many_records`). |
| `argocd_audit_kubernetes_events_total{outcome}` | Kubernetes Events seen, by outcome (`recorded`, `ignored_reason`, `ignored_source`, `duplicate`, `before_start`). |

Alert on `argocd_audit_ingest_rejected_total{reason="unauthorized"}`: something other than the API server is trying to
write to the audit trail.

## Limitations

* The audit controller runs as a single replica. While it restarts, the API server retries delivery, then falls back
  to its own log.
* Changes made directly to Argo CD resources with `kubectl` bypass the Argo CD API and are not recorded. Use the
  [Kubernetes audit log](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/) for those.
* Logging out of the web UI and the CLI only discards the session locally and is not recorded.
