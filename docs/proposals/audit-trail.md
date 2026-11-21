---
title: Audit trail of user actions
authors:
  - "@ChathushkaRodrigo"
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2026-09-27
last-updated: 2026-09-29
---

# Audit Trail of User Actions

Record who did what in Argo CD, when, from where, and with what result, as a structured, append-only stream
that operators can retain and forward to their log pipeline or SIEM.

Related issues: [#29877](https://github.com/argoproj/argo-cd/issues/29877),
[#9616](https://github.com/argoproj/argo-cd/issues/9616), [#10210](https://github.com/argoproj/argo-cd/issues/10210),
[#7707](https://github.com/argoproj/argo-cd/issues/7707).

## Open Questions

1. **Where records are written.** Phase 1 below writes audit records from `argocd-server` to a dedicated log stream.
   Phase 2 adds a separate `argocd-audit-controller`. Is Phase 1 enough for the project, with Phase 2 left to a
   later proposal?
2. **Query API.** Should Argo CD serve recent audit records at all (and gate them with Argo CD RBAC), or should the
   log stream be the only interface?
3. **Schema.** Should the record format follow an existing schema, for example the Kubernetes `audit.k8s.io/v1`
   `Event` or the OpenTelemetry log data model, rather than an Argo CD specific one?
4. **Internal transport (Phase 2).** Shared token or mTLS between the Argo CD components and the audit controller,
   following what the repo server and commit server do?
5. **Webhooks.** Should `/api/webhook` deliveries be recorded for every delivery, or only when they cause a refresh?

## Summary

Argo CD has no single place that answers "who did what". Today an operator has to piece it together from
`argocd-server` request logs, Kubernetes Events and controller logs. These are unstructured for this purpose, events
expire after an hour by default, not every mutating call emits one, and denied requests are not reliably recorded.

This proposal adds an opt-in audit trail. Every mutating API call (including through the UI and CLI), every login,
and every web terminal session produces one structured audit record. The record names the Argo CD identity that made
the call, the client's address and user agent, the target object, a small allow-list of parameters and the result.
Calls rejected by authentication or RBAC are recorded too. Operations the controllers perform by themselves, such as
automated syncs, are recorded and attributed to the controller, or to the user who started them.

## Motivation

Teams running Argo CD in regulated or multi-tenant environments need to answer questions such as:

- Who synced, deleted or rolled back this application, and when?
- Who turned automated sync or pruning on or off? (#10210)
- Who changed a cluster, repository or project, or created a project token?
- Which requests were denied by RBAC or rejected for bad credentials, and from which address? (#9616)
- Did this sync happen because a person clicked Sync, or because the controller synced automatically? (#7707)

Kubernetes API server audit logs do not answer these questions. Argo CD acts on clusters with its own service
accounts, so the Kubernetes audit log names Argo CD, not the Argo CD user behind the request, and it never sees
requests that Argo CD itself rejects.

### Goals

- One structured record per mutating API call, login and web terminal session, including failed and denied ones.
- Records carry the Argo CD identity RBAC uses: local account, SSO user with issuer and groups, or project role token.
- Automated operations are distinguishable from user-initiated ones, and a completed operation is attributed to the
  user who started it.
- No secrets in records: credentials, tokens, manifests and patches are never recorded.
- Auditing never blocks or slows down API calls, and records are not silently dropped.
- Opt-in, with no behaviour change for users who do not enable it.

### Non-Goals

- Recording read-only calls (get, list, watch, logs, resource tree...).
- Recording changes made directly to Argo CD resources with `kubectl`. These bypass the Argo CD API; the Kubernetes
  audit log covers them.
- Tamper-proof storage, retention policies or a UI for browsing the trail. Operators keep using their log pipeline.
- Recording the full diff of what changed in an object. The record says what was changed and by whom, not the
  before/after content.

## Proposal

### Use cases

#### Use case 1: Incident review
As an operator, I want to find out who deleted an application and from which address, so that I can review an
incident.

#### Use case 2: Compliance
As a compliance officer, I want every change to production applications, clusters and repositories recorded with
the responsible identity, so that I can show the change history to an auditor.

#### Use case 3: Security monitoring
As a security engineer, I want failed logins and RBAC denials recorded in a structured form, so that my SIEM can alert
on brute-force attempts or privilege probing.

#### Use case 4: Automation versus people
As a platform engineer, I want to tell automated syncs apart from syncs a person started, so that I can explain why an
application changed.

### Record format

Each record is a single JSON object:

```json
{
  "kind": "AuditRecord",
  "id": "5b7e3a0c-2f0e-4a53-9b0c-2a8d1f6f3e7a",
  "timestamp": "2026-09-27T08:08:39Z",
  "source": "argocd-server",
  "actor": {
    "username": "alice@example.com",
    "subject": "CgVhbGljZRIEbGRhcA",
    "issuer": "https://argocd.example.com/api/dex",
    "groups": ["platform-team"],
    "sourceIP": "203.0.113.9",
    "userAgent": "argocd-client/v3.6.0"
  },
  "action": "application.delete",
  "verb": "delete",
  "method": "/application.ApplicationService/Delete",
  "resource": {"type": "application", "name": "guestbook", "namespace": "argocd", "project": "default"},
  "details": {"cascade": "true"},
  "result": {"status": "failure", "code": "PermissionDenied", "message": "permission denied"},
  "durationMillis": 12
}
```

- `actor.username` is the Argo CD user: the local account name, the SSO email claim, `proj:<project>:<role>` for
  project tokens, `unauthenticated` for missing or invalid credentials, or `system:<component>` for controllers.
- `action` is `<resource type>.<rpc name>` (for example `application.sync`, `cluster.rotate-auth`), so new RPCs are
  covered without a mapping table.
- `result.code` is the gRPC status code for API calls, the HTTP status for HTTP-only endpoints (SSO login and
  logout, webhooks), or the operation phase for controller records.

### Phase 1: audit records from `argocd-server`

A gRPC unary interceptor in `argocd-server` builds a record for every audited call. The REST API is covered too,
because grpc-gateway turns REST calls into gRPC calls.

- **Which calls.** RPCs whose names start with `Create`, `Update`, `Patch`, `Delete`, `Sync`, `Rollback`,
  `Terminate`, `RunResourceAction`, `UpdatePassword`, `RotateAuth` or `InvalidateCache`, plus local-account login
  (`SessionService/Create`) and `SessionService/Delete`. Everything else is read-only and not recorded.
- **Endpoints outside gRPC.** Some actions don't go through gRPC, so the interceptor never sees them. Each one gets
  an explicit audit call:
  - SSO login: the OIDC callback handler (`/auth/callback`) records a successful login with the claims it
    established, and a failed one with the reason (invalid state, token exchange or verification failure).
  - Logout: the `/auth/logout` handler records who logged out.
  - Webhooks: the `/api/webhook` handler records each delivery with the provider, the event, the repository
    (credentials redacted), whether the signature was valid, and which applications were refreshed. The actor is
    `system:webhook` with the sender's source address, because webhooks carry no Argo CD identity.
- **Identity.** The interceptor is installed before the authentication interceptor, so calls rejected by
  authentication are recorded too. A second interceptor after authentication captures the claims. When anonymous
  access is enabled and an invalid token is downgraded to anonymous, the record says so instead of showing a plain
  anonymous call.
- **Source address.** The client address is resolved exactly like source IP logging does, honouring
  `server.trusted.proxies` and `server.client.ip.header`, so clients cannot spoof it with `X-Forwarded-For`.
- **Web terminal.** A record is written when a terminal session is refused (for any reason, not only RBAC), opened,
  and closed (with its duration). The resource namespace is the effective application namespace.
- **Output.** Records are written as JSON lines to a dedicated logger, to standard output with `"kind":"AuditRecord"`,
  or optionally to a file. They are kept apart from the server's operational log format and level, so a log level
  change cannot turn auditing off.
- **Controller operations.** The application and ApplicationSet controllers already report what they do through
  `argo.AuditLogger` (automated sync started, operation completed, resource action ran...), which today only turns
  those calls into Kubernetes Events. `AuditLogger` gains an audit sink: every such call also produces an audit
  record, whether or not Kubernetes Events are enabled with `--enable-k8s-event`, and even if creating the Event
  fails. The record is attributed to `system:<controller>`, or to the user who initiated the operation (taken from
  the operation's `initiatedBy`). Kubernetes Events are not an input to the audit trail at all, so there is nothing
  to watch, checkpoint or replay.

Phase 1 needs no new component and could ship on its own. Each component writes its own records to its own log.

### Phase 2: `argocd-audit-controller`

A new, optional component becomes the single place the audit trail is written. It is only a sink: it receives
records and writes them out, and does not read anything from the cluster.

- `argocd-server` and the controllers ship the records Phase 1 produces to the audit controller asynchronously, in
  batches, over an authenticated internal endpoint, instead of logging them themselves. The queue is bounded. If the
  audit controller cannot be reached after retries, or the queue is full, the component writes the records to its own
  log with a distinct message, so they are not lost and API calls and reconciliation are never blocked.
- It writes each record as one JSON line to standard output, and optionally to a file on a persistent volume.
  Standard output is the source of truth: if an optional output fails, the record is still accepted and the failure is
  counted in a metric, so the sender does not retry and duplicate it.
- Delivery is at least once. Every record carries a unique ID assigned by the component that produced it, so a batch
  retried after a lost response can be de-duplicated by consumers, or by the audit controller itself within a
  bounded window.
- It exposes Prometheus metrics: records written by source, action and result; write errors; and rejected ingest
  requests.
- Optionally (Open Question 2), it serves recent records on an API gated by Argo CD RBAC.

```text
 users, SSO, webhooks ──► argocd-server ──────────────┐
                                                      ├─► argocd-audit-controller ──► stdout (pod log)
 argocd-application-controller ───────────────────────┤                           ├─► optional file
 argocd-applicationset-controller ────────────────────┘                           └─► /metrics
```

### Implementation Details/Notes/Constraints

- **Configuration.** Phase 1: `server.audit.enabled` (and optionally `server.audit.log.file`) in
  `argocd-cmd-params-cm`. Phase 2: `server.audit.controller.address`, plus `auditcontroller.*` keys for the controller.
- **Manifests.** Phase 2 adds `manifests/base/audit-controller` and an `install-with-audit.yaml` variant, like
  `install-with-hydrator.yaml`. The audit controller needs no Kubernetes API permissions, because it does not read
  from the cluster.
- **Existing bug.** `AuditLogger.LogAppProjEvent` prepares the `user` annotation but passes `nil` when creating the
  Event, so AppProject events never say who made the change. This is fixed independently of this proposal, and the
  audit record for project changes comes from the API call anyway.
- **Binary.** The audit controller is another entry point of the existing `argocd` binary, like the other components.
- **Prototype.** A prototype of both phases exists on
  [ChathushkaRodrigo/argo-cd@feature/audit-trail-controller](https://github.com/ChathushkaRodrigo/argo-cd/tree/feature/audit-trail-controller).
  It is for discussion only, and it still watches Kubernetes Events, which this revision drops. Its automated review
  found issues this proposal addresses: redacting credentials in repository URLs, not duplicating records when an
  optional output fails, recording every refused terminal session, and distinguishing a downgraded invalid token from
  anonymous access.

### Detailed examples

Enable Phase 1:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
data:
  server.audit.enabled: "true"
```

Find who deleted what in the last day:

```shell
kubectl -n argocd logs deploy/argocd-server --since=24h \
  | jq -r 'select(.kind == "AuditRecord" and .verb == "delete") | [.timestamp, .actor.username, .resource.type, .resource.name, .result.status] | @tsv'
```

An automated sync, recorded by the application controller:

```json
{"kind": "AuditRecord", "source": "argocd-application-controller",
 "actor": {"username": "system:argocd-application-controller"},
 "action": "application.operation-started", "resource": {"type": "application", "name": "guestbook", "namespace": "argocd"},
 "details": {"automated": "true"}, "result": {"status": "success", "code": "Running"}}
```

### Security Considerations

- **No secrets in records.** Only an explicit allow-list of request fields is copied into a record. Request bodies,
  manifests, patches, passwords, bearer tokens, TLS keys and SSH keys are never read. URLs (repositories, clusters,
  Helm and OCI registries) have any embedded user information removed before they are recorded. Unit tests assert
  that secrets in requests do not reach records.
- **Personal data.** Records contain user names, email addresses, groups and IP addresses, which may be personal data.
  The feature is opt-in and the documentation says so.
- **Spoofing.** The client address honours only configured trusted proxies. Records are JSON-encoded, so user input
  such as application names cannot inject fake log lines.
- **Webhooks.** Webhook deliveries are recorded even when their signature is invalid, which makes forged or
  misconfigured webhooks visible. Payloads are never recorded.
- **Phase 2 endpoint.** The ingest endpoint only accepts records from Argo CD components (shared token or mTLS, and a
  NetworkPolicy). Rejected attempts are counted in a metric so they can be alerted on. Request size and records per
  request are bounded.
- **Availability.** The in-memory queue is bounded and batches are capped, so a slow or unavailable audit controller
  cannot exhaust `argocd-server` memory or delay API calls.

### Risks and Mitigations

- **Log volume.** Only mutating calls are recorded, so the volume is small compared to request logs. Busy CI pipelines
  that sync often produce one record per sync.
- **Lost records.** Phase 1 writes synchronously to a local stream. In Phase 2, records a component cannot deliver
  are written to its own log. Controller records don't depend on Kubernetes Events, so turning Events off or a failed
  Event write loses nothing.
- **Duplicates.** Phase 2 delivery is at least once. Record IDs let consumers drop duplicates.
- **New RPCs.** Classification by RPC name prefix covers new mutating RPCs that follow the existing naming
  conventions. A unit test lists every registered RPC and fails when a new one is neither audited nor explicitly
  marked read-only.

### Upgrade / Downgrade Strategy

No CRD or API changes. The feature is off unless configured, so upgrading changes nothing. Downgrading removes the
audit records; records already written stay wherever the operator's log pipeline stored them. Removing
`argocd-audit-controller` while `server.audit.controller.address` is still set makes `argocd-server` write undelivered
records to its own log, with a warning.

## Drawbacks

- Phase 2 adds a component to deploy, secure and monitor.
- Some information overlaps with existing request logs and Kubernetes Events.
- The record format becomes an interface users build tooling on, so it has to be kept stable.

## Alternatives

1. **Improve the existing request logs.** Add the user and result to every gRPC log line. This is cheaper, but the
   audit trail stays mixed with operational logs, depends on the log level, and still lacks denied and rejected calls
   in a structured form. Phase 1 is close to this alternative, with a dedicated stream instead.
2. **Rely on Kubernetes Events.** Events expire (one hour by default), are not emitted for every mutating call, can
   be turned off with `--enable-k8s-event`, and a failed Event write is only logged. Reading controller actions back
   from Events (as the first draft of this proposal did) also needs a checkpoint to survive restarts and still
   cannot recover an Event that was never written.
3. **Rely on the Kubernetes API server audit log.** It names Argo CD's service account instead of the Argo CD user
   and does not see requests Argo CD rejects.
4. **Export audit records through OpenTelemetry logs.** Argo CD already exports traces via OTLP. Audit records could
   be sent as OTLP logs, which would suit users with an OpenTelemetry collector but add a dependency for everyone else.
   This could be an additional output of Phase 1 or Phase 2.
