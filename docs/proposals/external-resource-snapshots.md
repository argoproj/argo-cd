---
title: Opt-in snapshots for externally managed resources
authors:
  - "@bjzhangshuai"
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD
creation-date: 2026-10-16
last-updated: 2026-10-16
---

# Opt-in Snapshots for Externally Managed Resources

## Status and Open Questions

This is an unapproved design proposal associated with
[issue #30222](https://github.com/argoproj/argo-cd/issues/30222), not an implementation or a compatibility commitment.
AI tooling assisted with analysis and drafting. No sponsor, reviewer, or approver has been assigned.

Before implementation, the community needs to decide:

1. Does restoring non-rendered state belong in Argo CD, or in an external controller or hook integration?
2. Where should administrators define selection, field-level permissions, retention, and quotas?
3. Should snapshots be a separate namespaced resource, and what authorization governs reading and restoring them?
4. Which destination identity survives credential rotation but prevents reuse after cluster replacement?
5. How should cooperating operators explicitly allow restoration without immediately overwriting it?
6. What availability and retention guarantees apply to snapshots referenced by deployment history?

## Summary

An Application can depend on Kubernetes resources outside its rendered manifests. This proposal explores an opt-in
mechanism to capture explicitly permitted live fields and associate them with a deployment history entry. An explicitly
requested rollback could restore that snapshot in addition to the ordinary revision rollback.

The initial scope is bounded, namespaced resources and ApplyOnly restoration, disabled by default. This PR adds no
CRD, API, CLI flag, controller behavior, or resource permissions.

## Motivation

Rolling back a Git or Helm revision does not necessarily restore external configuration produced by an operator or
another platform component. A history-linked snapshot could make that dependency visible and, where the owning
system permits it, recover approved configuration associated with a deployment.

Live state is not automatically a source of truth. Restoring fields without cooperation from their controller can
create a reconciliation loop. Selection cannot be based only on the absence of an Argo CD tracking label.

### Goals

* Require administrator opt-in and Application-level selection within that authorization boundary.
* Separate management-cluster storage from destination-cluster discovery, reads, and writes.
* Bind each snapshot to an exact Application incarnation, destination, operation, and history entry.
* Make capture availability and rollback outcomes observable and restart-safe.
* Reject unauthorized, incomplete, incompatible, or stale restores before mutation where possible.
* Preserve existing behavior when the feature is disabled or external restoration is not requested.

### Non-Goals

* Full-cluster backup, transactional rollback, or persistent-volume recovery.
* Making all live resources Git-managed or adding them to ordinary pruning.
* Secret payload capture, encryption, cluster-scoped resources, or creating missing external resources initially.
* Prune, DiffOnly, automatic force-apply, or restoration of status and runtime metadata.
* Compatibility with a downstream prototype's CRD or operation annotations.

## Proposal

### Use Cases

#### Recover explicitly delegated configuration

A platform operator delegates a bounded set of fields on a namespaced custom resource to an Application's deployment
workflow. An administrator permits capture and restoration of those fields. A deployment records the permitted values;
an explicitly requested rollback can reapply them to the same resource incarnation.

#### Explain an unavailable or failed external rollback

A snapshot is incomplete, its source resource was recreated, or current policy no longer permits access. Argo CD
rejects or fails the requested restoration with a concrete reason instead of reporting successful full rollback
while hiding an external-resource error in controller logs.

### Authorization and Selection

Authorization is the intersection of administrator policy, AppProject destination/resource restrictions, Argo CD user
permissions, destination-cluster permissions, and the Application's selection. Capture permission does not imply restore
permission. Updating an Application must not let a tenant widen the administrator's policy.

Policy should constrain namespace, API group/kind, resource identity or bounded label selection, and captured/restorable
fields. It must bound resource count and payload size. Default limits and policy syntax need design review before
implementation; this document does not introduce configuration fields.

Resources in rendered manifests, resources managed by another Application, and resources with unresolved ownership are
ineligible. Checks must respect the configured tracking method and installation identity, not one hard-coded label.
An owner reference or another controller's field ownership requires explicit delegation by that system; selecting an
object is not proof of delegation.

Secrets are excluded initially. Other kinds can also contain credentials or personal data, so kind exclusion alone
is insufficient. Administrators must approve field allowlists and avoid capturing embedded credentials.

### Destination and Snapshot Identity

Snapshot storage belongs in the management cluster. Live-resource operations must resolve the Application's destination
through normal cluster configuration and authorization, never silently fall back to the management-cluster client.

Immutable snapshot provenance needs at least:

* Application namespace, name, and UID, distinguishing recreated Applications.
* Destination identity and namespace, separate from rotating authentication material.
* Exact deployment history ID, including zero, and the associated operation identity.
* Resolved source/revision metadata for single-source and multi-source Applications.
* Policy identity/version, capture time, schema version, and resource-level outcomes.
* Resource identities including UID, and the permitted normalized field payloads.

The destination identity and API representation remain open questions. UIDs are provenance for replacement detection,
not fields to apply. Payload digests can detect accidental corruption but do not prove authenticity: immutable finalized
storage and restricted write permissions are still required.

### Capture Lifecycle

Capture requires an eligible successful deployment with a persisted history entry. A durable capture request binds
to that operation/history identity when created; a worker must not infer it from a later refreshed `operationState`.

Discovery and reads use the resolved destination. Capture takes only approved fields and removes status and
server-managed metadata. It observes state after synchronization, not an atomic cluster-wide point-in-time image.
Capture timestamps and resource versions must make the observation window explicit.

The proposed lifecycle distinguishes Pending, Capturing, Ready, and Failed snapshots. Discovery, list, normalization,
authorization, and storage failures are persisted with resource details. Partial results are not Ready and cannot be
restored initially. Capture failure does not retrospectively fail an ordinary successful sync; it exposes external
rollback as unavailable through a separate condition.

Capture work must have deadlines and quotas, deduplicate immutable operation/history identities, and recover after
restart. Overlapping deployments must not overwrite one another's snapshots.

### Restore Lifecycle

External restoration is explicitly requested with an ordinary history rollback; snapshot existence does not enable it.
Existing restrictions on ordinary rollback continue to apply.
Dry-run requests may validate and describe the selected restore plan but must not persist mutations to live resources.

Before starting, resolve the exact requested history entry and snapshot. Validate readiness, Application UID,
destination, schema compatibility, current policy, and user permissions. Missing, incomplete, tampered, or incompatible
snapshots fail preflight. Never fall back to the newest snapshot.

The proposed execution order is ordinary revision synchronization followed by external restoration within the same
operation. Completion is gated on both phases. Initial-scope restoration is strict: an external failure makes the
combined operation unsuccessful and identifies which phase changed resources. Multi-resource writes are not atomic,
and the feature does not promise automatic compensation for writes already completed.

Before each external write, revalidate destination, resource UID, current authorization, and ownership. Reject missing
or recreated resources initially instead of silently recreating or overwriting them. Apply only approved fields with
a dedicated field manager. SSA conflicts fail the resource, without force-apply even when errors also mention metadata.
Schema and field-type changes require an explicit compatibility decision.

Persist the selected snapshot and per-resource progress, support idempotent retry, honor termination, and survive
restart. Capture cannot replace a snapshot selected for restoration. No detached worker may report errors only after
the Application has recorded successful rollback.

### Storage, Retention, and Observability

A dedicated namespaced snapshot resource is one option, subject to API review. It needs separate access control,
immutable finalized payloads, bounded count/total bytes, and garbage collection. Oversized payloads fail explicitly;
they are not silently truncated. Application recreation cannot inherit snapshots from an earlier UID.

Protect an actively selected snapshot from retention cleanup until its operation completes or terminates. Define
behavior when history retention is disabled, entries are removed, policy is removed, or an Application is deleted
before enabling the feature, rather than allowing unbounded storage growth.

Expose capture eligibility and failures alongside history and restore progress in operation results. CLI and UI must
distinguish ordinary rollback from rollback with external restoration. Logs/events contain identities and reasons,
not payloads. Any public API requires generated schemas, clients, documentation, and both Argo CD and Kubernetes RBAC review.

### Detailed Example

An Application in a management cluster deploys into namespace `payments` in a different destination cluster. An
administrator permits a delegated subset of fields on one external custom resource there.

History ID `0` records the Application UID, destination identity, revision, and a Ready snapshot. A later deployment
records a separate snapshot. An explicit rollback to ID `0` with external restoration selects the first snapshot.
After revision synchronization, the operation reapplies approved fields to the original resource UID in the destination.

If another controller now owns a selected field, the external phase reports the conflict and fails. It does not force
ownership, touch a similarly named management-cluster resource, or report complete success. This is a behavioral
example, not installable configuration syntax.

### Security Considerations

Snapshot access can expose data that a user could not previously read from the destination. Authorize snapshot reading
independently of Application visibility. Consider encryption at rest, backups, logs, and exports even without Secret capture.

Restore must not bypass project restrictions, destination isolation, impersonation, resource exclusions, or current
policy. Revalidate permissions after capture and before every write. Negative tests must cover cross-project,
cross-Application, cross-namespace, and cross-cluster substitution.

### Risks and Mitigations

* Operator reconciliation may undo restored values. Require delegation and field restrictions; prefer restoring the
  operator's declarative input where possible.
* Captures are not transactionally consistent. Document the observation window and do not claim database recovery.
* Partial writes are possible. Persist resource outcomes, bound retries, and expose operation failure.
* Snapshot storage and discovery add load. Enforce quotas, retention, deadlines, and bounded concurrency.
* Recreation and schema changes invalidate payloads. Fail closed on identity or compatibility mismatch.
* The controller's security surface grows. Keep the feature disabled by default and require security/API review.

### Upgrade / Downgrade Strategy

When disabled, upgrades preserve existing behavior and require no additional permissions. Opt-in requires supported
policy/storage schemas and appropriate RBAC only after those APIs are approved.

Version snapshot formats; unknown versions cannot be restored. Downgrade guidance must disable new capture/restore
requests and resolve in-flight work before deploying a controller that cannot interpret that state. Retention/export
is explicit; downgrade does not imply old snapshots are safe to apply.

### Validation Plan

Unit tests cover selection, field filtering, history ID zero, multiple sources, partial capture failure, normalization,
size limits, immutable identity, malformed payloads, strict operation results, and retention.

Integration/e2e tests require separate management/destination clusters. Cover disabled-feature compatibility, permitted
capture/restore, other Applications' resources, credential rotation, cluster replacement, Application recreation,
resource UID changes, revoked permissions, SSA conflicts, termination, retry, restart, concurrent deployments,
and retention during restore. A test must fail if any live write reaches the management cluster.

API changes require generated CRD/client/CLI artifacts and a UI plan. Implementation must not advertise unsupported
encryption, pruning, diff, or recovery modes. These are acceptance criteria for future code, not tests implemented here.

## Drawbacks

This extends Argo CD beyond declarative revision management and adds durable state, authorization, retention, and
recovery semantics. Even a correct snapshot cannot make arbitrary external resources safe to restore. Maintenance
cost may outweigh the value of integration into the controller.

## Alternatives

* Keep configuration in Git or Helm inputs and let its operator reconcile it.
* Use a dedicated backup/restore controller with explicit hooks or workflow integration.
* Record external-state references in deployment metadata without granting restoration capability.
* Initiate a cooperating operator's snapshot/restore API rather than applying live manifests.

If the community prefers another boundary, redirect the proposal rather than treating the downstream prototype as
an obligation to add a new Argo CD API.
