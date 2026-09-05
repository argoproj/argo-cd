---
title: Allow Repositories to be stored as CRDs
authors:
  - "@blakepettersson"
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2025-12-10
last-updated: 2026-08-26
---

# Repository CRD

Allow ArgoCD repositories to be stored as CRs, while maintaining full backwards compatibility with the existing
secret-based implementation.

## Open Questions

* **API version, and the `Repository` name collision behind it.** `argoproj.io/v1alpha0` is an unconventional first
  version; it is used because `pkg/apis/application/v1alpha1` already declares a `Repository` Go type (the config
  struct serialized into Secrets and exposed over gRPC/REST), so codegen cannot add a second one there. The clash is
  purely the Go identifier — `v1alpha1.Repository` is not a `runtime.Object` and that kind is not registered in the
  scheme — and `RepositoryCredential` has no such clash, but shares `v1alpha0` so the two kinds stay on one version.
  `v1alpha0` defers rather than resolves it: promoting to
  `v1alpha1` needs the colliding name. Options, cheapest first:
  - **Skip `v1alpha1` on promotion** (`v1alpha0` → `v1alpha2`/`v1beta1` → `v1`): free, but the odd version stays.
  - **New group** `repository.argoproj.io/v1alpha1`: frees the name, keeps kind `Repository` and `kubectl get
    repositories`; costs a second group to maintain and an `apiGroups` entry in every Role.
  - **Different kind** (e.g. `RepositoryConfig`): no collision, no new group, loses the `kubectl get repositories` UX.
  - **Rename the existing structs**: trivial, but technically a breaking change.
* **Per-controlplane status above the retention cap.** Transition-only writes bound churn by how often verdicts change
  rather than by probe rate, so a controlplane in a full list no longer pays per interval. The steady state above the
  cap is still open: an evicted controlplane re-claims its slot on its next transition and evicts someone else, so a
  flapping fleet trades slots indefinitely. Whether that is acceptable — or whether success should stop claiming a slot
  above the cap, making the list scale with the number of *unhealthy* controlplanes — is unsettled.
* **Cache-backed reads.** The CRD backend currently resolves repositories with direct (uncached) LIST calls, one per
  lookup. The repository controller already maintains an informer; sharing a lister with the DB backend would remove
  the remaining per-reconcile API-server load.

## Summary

Argo CD stores repository configuration and credentials in Secrets. This proposal adds two namespaced CRDs —
`Repository` and `RepositoryCredential` (`argoproj.io/v1alpha0`) — holding all *non-secret* configuration
declaratively, with credential material remaining in a Secret referenced via `spec.secretRef`. Storage is selected per
component by `--repository-backend-mode` (env `ARGOCD_REPOSITORY_BACKEND`): `secret` (default, existing behavior),
`crd`, or `hybrid` (read both, CRDs win; writes migrate to CRDs). A repository controller — running inside the
application controller in crd/hybrid mode — periodically tests connectivity and maintains a multi-controlplane-aware
status.

## Motivation

- More GitOps-friendly (and secure) declarative management, since we can put (most of) the config in Git
- Richer validations with OpenAPI validations (and possibly webhook-based ones)
- Enhanced security for agent-based architectures
- Enhanced Kubernetes integration (`kubectl get repositories` and friends)
- Richer status checks for connection health of a given repository

### Goals

1. **Introduce CRDs**
    - `Repository` CRD for repository configurations
    - `RepositoryCredential` CRD for credential templates
    - Maintain existing API types internally

2. **Backwards Compatibility**
    - Existing secret-based repositories continue to work (`secret` remains the default mode everywhere)
    - No breaking changes to CLI commands (`repo add`, etc.) or to the gRPC/REST API contracts
    - `hybrid` mode reads from both CRDs and Secrets simultaneously, with CRDs taking precedence

3. **Gradual Migration**
    - Operators can migrate at their own pace by opting into `hybrid` mode
    - Automatic migration on update: updating a secret-backed repository in hybrid mode creates the CRD and then
      deletes the legacy Secret. The delete is best-effort — a failure is logged rather than failing the update, and
      leaves a Secret that components still in `secret` mode keep serving
    - Every component that resolves repositories (API server, application controller, applicationset controller,
      notifications controller) takes the same flag/env, so the fleet can be switched consistently

4. **Security Parity**
    - Credential material stays in the referenced Secret; everything else lives in the spec
    - Status is a subresource, writable only with an explicit `repositories/status` grant (see Security
      Considerations)
    - Read (`spec.write: false`) and write/push (`spec.write: true`, used by the source hydrator) repositories and
      credentials are distinct objects, filtered on in every lookup so the two sets cannot shadow each other
    - Same encryption-at-rest guarantees

### Non-Goals

1. **Changing internal API types.** `pkg/apis/application/v1alpha1.Repository` is unchanged; a single shared
   conversion (`util/db/repository_crd_conversion.go`) maps CRD ↔ internal types for both the DB layer and the
   repository controller. The CRD *spec* groups fields by type but maps onto the same structs. The CRD *status* is
   richer than the internal `ConnectionState` (`Status`/`Message`/`ModifiedAt`, statuses `Successful`/`Failed`/
   `Unknown`) and is projected onto it lossily: aggregate `Successful` → `Successful`; `Failed` and `Degraded` →
   `Failed` with the aggregate message (e.g. "1 of 2 clusters connected successfully"); `attemptedAt` → `ModifiedAt`.
   Per-entry detail, `reason`, `lastTransitionTime` and `observedGeneration` are CRD-only — `kubectl get` is where the
   multi-controlplane view lives. In crd/hybrid modes the API server serves this projection from the CR status instead
   of running its own `TestRepository` on a connection-state cache miss (`server/repository.List` today), so there is
   one prober per controlplane and one verdict, and the forge load counted below is the whole of it.
2. **Immediate Secret deprecation.** Secret-based storage stays supported and remains the default; deprecating it
   would be a separate proposal.
3. **API contract changes.** CLI behavior, the gRPC/REST API and client SDKs are all unaffected.
4. **Multi-tenancy redesign.** Project scoping is unchanged, and namespace isolation
   ("repositories-in-any-namespace") is not addressed — though the namespaced CRD shape leaves room for it.
5. **Workload identity.** Existing ad-hoc support is carried through as-is (`spec.useAzureWorkloadIdentity`, as
   with secrets today); a holistic solution is future work.

## Proposal

### Use cases

#### Use case 1:
As a user, I would like to manage my repositories using GitOps.

#### Use case 2:
As a user, when running Argo CD in separate controlplanes, I want to ensure that my secrets stay on the workload
clusters.

### Implementation Details/Notes/Constraints

#### Backend selection

* The storage backend is resolved at the command edges only: each binary registers a `--repository-backend-mode`
  flag whose default comes from `ARGOCD_REPOSITORY_BACKEND`. An explicit flag wins over the env var; an invalid or
  unset value resolves to `secret` with a warning.
* `db.NewDB` validates the mode/clientset pairing once: crd/hybrid without an application clientset downgrades to
  the secrets backend with a warning instead of failing at first use. The notifications controller's `argoCDService`
  currently builds `db.NewDB` with only a Kubernetes clientset and so would always take this downgrade; it gains the
  application clientset as part of this work.
* Components that only read *cluster* data (sharding, several admin commands) intentionally stay on the secrets
  backend regardless of mode — cluster storage is unaffected by this proposal. `argocd admin repo generate-spec`
  keeps emitting Secret manifests from CLI arguments, since that is the command's purpose; a new `argocd admin repo
  export` emits the equivalent Secret manifests from existing `Repository`/`RepositoryCredential` CRs, which is what
  downgrade needs (see Upgrade / Downgrade Strategy).

#### Storage model

* The spec carries all non-secret configuration, including fields stored inside the Secret today: `project`, proxy
  settings, `forceHttpBasicAuth`, `useAzureWorkloadIdentity`, per-type blocks (`git.enableLFS`, `git.depth`,
  `git.githubAppID`, `git.githubAppInstallationID`, `git.githubAppEnterpriseBaseUrl`, `helm.name`, `helm.enableOCI`,
  `oci.insecureSkipTLS`, …) and the `write` marker. There is no top-level `name`: the internal `Repository.Name` is
  the Helm repo alias (`helm repo add <name>`, and `@alias` resolution for Chart.yaml dependencies), unused for
  git/OCI, so it lives in the `helm` block.
* The Secret carries *only* credential material: `username`, `password`, `bearerToken`, `sshPrivateKey`,
  `tlsClientCertData`/`tlsClientCertKey`, `githubAppPrivateKey`, `gcpServiceAccountKey`, and the Azure
  service-principal fields. A `secretRef` is written only when credential material exists, so e.g. a
  workload-identity-only repository has no dangling reference.
* **`secretRef` resolves only to opted-in Secrets** — those labeled `argocd.argoproj.io/secret-type` with the new
  value `repository-creds` (credential material only, no `url`), or with the existing `repository`/`repo-creds`
  values, which already publish the Secret to Argo CD and so need no second opt-in; this is what lets migration
  reference a user-provided legacy Secret without relabelling it. `repository-creds` is a distinct value because the
  label is single-valued and `repo-creds` Secrets are listed as URL-prefix templates, where a url-less Secret would
  match every URL. Otherwise anyone able to create a `Repository` could name an arbitrary Secret in the control-plane
  namespace and have Argo CD send it to a URL of their choosing (see Security Considerations). An existing but
  unlabeled target reports `CredentialsMissing` rather than being silently ignored.

#### Repository resolution

Lookup is by URL + project, not resource name, so declaratively-created resources may use any name.

* **URL matching is normalized identically in both backends.** Repositories match with `git.SameURL` (case, whitespace
  and a `.git` suffix do not matter; a trailing slash does — `NormalizeGitURL` does not strip it, so `.../repo/` and
  `.../repo` are different repositories today and stay so); credential templates normalize with `git.NormalizeGitURL`
  and take the longest URL prefix. The CRD backend reuses the secrets backend's helpers, so hybrid mode cannot resolve
  one spelling of a URL to two objects.
* **Longest prefix wins across backends, not just within one.** In `hybrid` mode candidate templates from both stores
  are gathered and the longest prefix wins; CRD precedence is only the tiebreak at equal length. Backend precedence
  alone would let a CRD template for `https://github.com` beat a Secret template for `https://github.com/org` and
  silently change which credentials a repository gets mid-migration. Repository objects match on the whole URL, so
  there CRD precedence applies directly.
* **Project scope ranks above backend.** The secrets backend returns an exact `project` match first and falls back to
  a project-less repository only when none exists. The same order holds across backends: an exact-project match in
  either store beats a project-less one in either store, and CRD precedence breaks ties only within the same scope.
  Otherwise a project-less CR would silently replace an exact-project Secret's credentials mid-migration.
* **Duplicates resolve deterministically and are logged.** Two `Repository` resources with the same URL and project
  are a misconfiguration: lookup picks the first-sorting resource name so every component agrees, and whichever
  component resolved it logs a warning naming both resources, once per component per (URL, project) until the set of
  duplicates changes — a refresh resolves the same repository more than once, so an unthrottled warning would repeat
  every few minutes per Application. A log line rather than an event or a status condition: detection happens inside
  the lookup, which runs in four components, three of them read-only on these resources. The controller could set a
  condition on the losing resource, but duplicates detected by the other three would go unreported, and a
  per-request event would re-fire on every resolution. CEL cannot express cross-object uniqueness, so this is
  detection, not prevention.

#### Secret ownership and lifecycle

One rule drives both deletion and migration: Argo CD deletes a Secret only if it created it. "Created it" is decided
the way the secrets backend already decides it: legacy Secrets Argo CD generated carry the
`managed-by: argocd.argoproj.io` annotation (`util/db/secrets.go`), and anything without it is user-provided.

* **Generated Secrets** — created when a repository is registered imperatively (`argocd repo add`, UI, API). One per
  `Repository`, named as repository Secret names are derived today, carrying an owner reference to the CR (and the
  `managed-by` annotation, for components still in `secret` mode). Deleting the `Repository` garbage-collects it; no
  finalizer.
* **User-provided Secrets** — anything else a `secretRef` points at. Never owner-referenced, never deleted; several
  repositories may share one, and its lifecycle belongs to whoever manages it, including external controllers such as
  External Secrets Operator (which would recreate it anyway).
* **Migration follows the same rule.** In `hybrid` mode an update writes the CR first — its spec populated from the
  Secret's non-credential keys — then reconciles the Secret: an Argo-CD-generated legacy Secret is replaced by a
  generated Secret owned by the new CR and the old one deleted; a user-provided one is left untouched and referenced
  via `secretRef`. It keeps its `url`/`project`/`type` keys, which crd/hybrid components ignore from then on and
  `secret`-mode components keep serving; drift between the two is the dual-representation risk below. CR-first
  ordering plus idempotent steps means a crash mid-migration leaves the repository resolvable — via the CR if written,
  via the still-present Secret if not — and the next update completes it.
* **Deleting a `Repository` ignores Applications that reference it**, matching `argocd repo rm`: no finalizer, no
  admission check, no warning. Affected Applications fail at their next refresh. Blocking deletion on references would
  be a behavior change and is out of scope.

#### Repository controller

Runs inside the application controller when the mode is `crd` or `hybrid`. Flags: `--repo-controller-workers`,
`--repo-controller-test-interval` (default 3m, `0` disables probing), `--repo-controller-test-timeout` (30s),
`--repo-controller-status-refresh-period` (15m), `--repo-controller-status-entry-ttl` (1h, `0` disables pruning) and
`--controlplane-name` (env `ARGOCD_CONTROLPLANE_NAME`).

* **`--controlplane-name` identifies a *logical* controlplane, not a process.** All replicas of one installation share
  it — hence the default constant `argocd-application-controller` rather than anything host-derived — and contribute
  one `clusterConnectionStates` entry between them. It must be unique only across *separate* controlplanes writing to
  the same objects, i.e. agent-based deployments. Two application-controller shards are one controlplane and report one
  entry, not "2 clusters".
* **Work is divided across replicas by repository, not duplicated.** Repositories are not shardable by cluster and the
  application controller has no leader election, so otherwise every replica would probe every repository and race to
  write the same entry — N× the probes and writes for no extra information. Each replica probes the repositories
  hashing to its shard index, resolved exactly as cluster sharding resolves it (`controller/sharding`):
  `ARGOCD_CONTROLLER_SHARD` if set, else the dynamic-distribution heartbeat ConfigMap when `ARGOCD_CONTROLLER_REPLICAS`
  > 1, else the StatefulSet hostname ordinal. The stock manifests set only the replica count, so reusing the resolver
  is what makes the default HA install work; a non-StatefulSet install with several replicas already has to set the
  shard or use dynamic distribution for clusters, and repositories inherit that requirement. Repositories of a dead
  replica go unprobed until it returns — the same gap clusters have; a StatefulSet replacement pod resumes the same
  ordinal, and dynamic distribution reassigns on its heartbeat. Replicas share a field manager and entry name, so a
  brief overlap during a rollout is harmless: identical content merges to a no-op.
* The controller is the **only** writer of `Repository` status; everything else reads it or feeds its work queue. In
  agent-based architectures there is one per controlplane, each writing its own entry. Each probe invokes the
  repo-server's `TestRepository` with the repository converted through the shared conversion, i.e. exactly the
  credentials manifest generation would use.
* **Status writes use server-side apply, one field manager per controlplane** — the manager being
  `--controlplane-name`, which is also the entry key. SSA rather than read-modify-write: GET/mutate/UPDATE makes every
  controlplane contend for the whole object and a lost race silently drops a peer's entry, whereas an apply carries
  only this controlplane's fields. Every apply targets the `status` subresource with `force: true`; the only competing
  writers run this same code. A probe that changes something writes twice:
  1. apply this controlplane's `clusterConnectionStates` entry (`listType=map` keyed by `name`, so the server merges
     entries across controlplanes); the patch response is the authoritative post-merge object;
  2. roll the aggregate `connectionState` and `Ready` condition up from that response and apply them **together with
     the entry again** — step 2 is a superset of step 1, never a disjoint set of fields.

  `conditions` is `[]metav1.Condition` with `listType=map` keyed by `type`, as `clusterConnectionStates` is keyed by
  `name`. Without it the list defaults to atomic, so each controlplane's apply would replace and sole-own the whole
  list and the shared-ownership argument below would not hold for `Ready`. One deliberate divergence from
  `metav1.Condition` convention: `Ready.observedGeneration` is the minimum across reporting entries, not the
  generation this writer observed (see below).

  Two applies rather than one because the aggregate is only computable from the merged entry list, which is not known
  until this controlplane's entry has landed. Rolling up from the response rather than the informer cache lets any
  controlplane own a write without regressing the aggregate to a stale view.

  The superset is required, not tidiness: omission deletes, so a step 2 asserting only the aggregate would drop the
  entry step 1 just wrote. The entry is sole-owned by design; the aggregate is co-asserted by peers and so survives a
  single controlplane omitting it — except in the single-controlplane default, where it is sole-owned too and step 1
  would delete the previous aggregate. Disjoint steps would therefore leave the object flip-flopping between
  entry-without-aggregate and aggregate-without-entry, and the client-side skip check would never match the cache, so
  every probe would write twice — exactly the amplification transition-only writing exists to avoid.
* Two SSA properties the design leans on: **omission deletes** — fields a manager stops asserting are removed, provided
  no other manager still owns them, which is why every apply must assert the full set of fields this controlplane still
  owns (and why prune and eviction are *not* applies; see below); and **the aggregate and `Ready` are shared,
  last-writer-wins** — deliberately not partitioned per manager, since both derive
  from the merged entry list and so converge regardless of write order. Controlplane-specific detail stays in that
  controlplane's entry. Each writer also adds a `metadata.managedFields` entry, part of why the list needs a cap.
* **Status is written on transition, not on probe.** Probing stays at the test interval; writing does not. This falls
  out of SSA rather than needing a diffing layer: an apply that merges to something equal to the live object is dropped
  by the server — no timestamp bump, no etcd write, no resourceVersion bump, no watch event. The payload therefore has
  to be stable when nothing changed:
  - **No unconditional timestamps.** Neither the entry's nor the aggregate's `attemptedAt` may be restamped every
    probe, or every apply is a real write. The aggregate's records when the aggregate verdict last *changed*.
  - **Writes trigger on `status`, `reason` or `observedGeneration` changing** — not on `message`. Failure messages wrap
    raw gRPC errors with variable detail (resolved addresses, DNS servers, temp paths), so keying on them means a write
    per probe per failing repository and a fleet-wide storm during a repo-server outage. Messages refresh on the next
    status refresh. This needs a per-entry `reason`, so `CredentialsInvalid` → `ConnectionFailed` is a visible
    transition rather than two entries that both just say `Failed`.
  - **The apply is skipped client-side** when the intended entry already matches the informer cache, so an unchanged
    repository costs no request at all. The periodic refresh bounds the resulting reliance on a possibly stale cache.
* **Status refresh period.** `attemptedAt` must still move sometimes: it is the heartbeat stale-entry GC prunes on, and
  it is what tells a reader how old a `Successful` verdict is. `--repo-controller-status-refresh-period` (15m) is the
  longest an unchanged entry may go without re-assertion — one apply per 15m instead of two per 3m, an order of
  magnitude fewer writes, and the floor below which transition-only writing cannot go — while keeping "last checked"
  trustworthy. Two constraints:
  - **`--repo-controller-status-entry-ttl` must be ≥ 4× the refresh period**, so a controlplane missing a few refreshes
    (restart, backlog, API-server blip) is not pruned as decommissioned. The defaults satisfy this; the controller
    validates the ratio at startup and warns rather than letting operators discover it through vanishing entries.
  - **Refreshes must not synchronize.** Each repository gets a deterministic phase offset from its UID so probes — and
    the refreshes riding on them — smear across the interval instead of bursting when `requeueAllRepositories` enqueues
    everything; a small random component keeps replicas from re-locking after a simultaneous restart. Without it,
    transition-only writes just move the burst from every 3m to every 15m.
* Repeatedly failing repositories back off, capped at the status refresh period, so an outage does not mean probing a
  known-broken repository every 3m from every controlplane. The cap is not arbitrary: the heartbeat rides on probes, so
  a cap above the refresh period would let a failing repository's entry go silent, be pruned at the TTL as
  decommissioned, and flip the aggregate healthy. Recovery detection is correspondingly delayed, which the observations
  below shorten for repositories actually in use.
* Failures are classified into condition reasons: gRPC `Unauthenticated`/`PermissionDenied` and well-known git/SSH/HTTP
  auth failure messages → `CredentialsInvalid`; a nonexistent credentials Secret → `CredentialsMissing`; everything
  else → `ConnectionFailed`.
* The `Ready` condition derives from the **aggregate**, not the local result, so all controlplanes converge
  (`Degraded` → `Ready=False`/`ConnectionFailedInSomeClusters`). Its `observedGeneration` is the minimum across entries
  that report one, so `Ready=True` at the current generation means every reporting controlplane verified the current
  spec. Entries reporting none (written before the field existed) are skipped rather than holding the minimum down.
* Each entry carries its own `observedGeneration` — the spec generation that controlplane last evaluated — so a verdict
  predating a spec change (e.g. a credential rotation) is distinguishable from a failure against the current spec.
* **Prompt re-tests from real traffic:** connection-type failures during manifest generation (gRPC `Unavailable`,
  `Aborted`, `Unauthenticated`, `PermissionDenied`) are reported by the reconciliation path as *observations*, which
  enqueue an immediate re-test (rate-limited to one per repository URL per 30s). The controller stays the only status
  writer; observations only feed its queue. Recovery is detected by the next periodic probe.
* **Stale-entry garbage collection:** the periodic refresh doubles as a heartbeat, so every controlplane re-asserts its
  `attemptedAt` at least once per refresh period regardless of outcome (the probe backoff cap above guarantees this
  for failing repositories too). Entries silent for longer than the entry TTL are treated as decommissioned and pruned
  by whichever controlplane notices, via the prune update described below. The TTL must comfortably exceed the
  refresh period — not the test interval, which no longer governs the heartbeat.

  A controlplane that outlives its own pruning (down longer than the TTL, then back) needs no special handling: its
  next probe finds no entry in the cache, rewrites it, and the aggregate re-counts it. Three consequences are accepted
  rather than mitigated: while pruned it is absent from the aggregate counts with no `truncated` marker — pruning
  asserts the controlplane is gone, and the counts describe what is believed alive; on return it re-emits its
  "first result" connection event, which for a controlplane silent for over an hour is arguably wanted; and a
  controlplane wedged rather than gone is indistinguishable from a decommissioned one, which is what the TTL ≥ 4×
  refresh margin is for.

  **The prune update.** SSA cannot cheaply remove an entry another manager owns: applying it with identical content
  merely *shares* ownership (and a shared field survives omission by one owner), while `force` transfers only the
  *conflicting* fields, so deleting the item would mean conflicting on every field of it. Prune is therefore a plain
  status **update**, not an apply: the object returned by the entry apply, minus the doomed entries, is written back
  with `UpdateStatus`. An update may remove fields any manager owns and the server drops them from `managedFields` in
  the same write, so there is no takeover step, no window in which this controlplane's own entry is missing, and no
  `maxItems` problem for a controlplane that holds no slot. The update's `resourceVersion` precondition is the guard
  against clobbering a peer's concurrent entry apply: a conflict is logged and the prune simply retried on the next
  probe, since the stale entries are still there. The aggregate is then rolled up from the pruned object as usual.
* **Bounded status:** one entry per controlplane means an unbounded list grows with the fleet, and large agent-based
  deployments have hundreds or thousands of controlplanes. The list has a hard `maxItems` (100) and each `message` a
  `maxLength` of 1024 (the controller truncates; repo-server errors are otherwise unbounded), capping worst-case status
  near 130 KiB — inside etcd's ~1.5 MiB limit with room for `managedFields`. The cap is a *retention budget*, not a
  supported fleet size: above it per-entry detail is a sample and the aggregate is the fleet-wide answer.
* **Retention policy: evict the least-recently-*changed* entry.** A steady-state `Successful` entry says nothing the
  aggregate does not, so a controlplane needing a slot in a full list evicts the entry unchanged the longest. This
  requires:
  - **A recency key that is not `attemptedAt`** — the heartbeat keeps that fresh. Each entry gains a
    `lastTransitionTime`, moving only when its `status` changes, and eviction orders on that.
  - **Failures pinned.** A `Failed` entry is never evicted for a `Successful` one, however stale — failing
    controlplanes are why per-entry detail exists. Ranking: `Successful` first, oldest `lastTransitionTime` first,
    entry name as tiebreak so concurrent writers pick the same victim.
  - **An aggregate that does not silently undercount.** `totalClusters`/`successfulClusters`/`failedClusters` derive
    from the entries, so after eviction they describe the retained sample; a `truncated: true` marker keeps "2 of 2
    clusters" distinguishable from "2 of 2 *retained* clusters".

  Eviction reuses the prune update: the victim's entry is removed with `UpdateStatus` (resourceVersion-guarded), after
  which the evictor's entry apply fits. It bounds the object but does not by itself make an over-cap fleet behave well;
  see the open question above.
* Spec changes (generation bumps) trigger prompt re-tests via the informer; the controller's own status writes do not
  bump the generation and are filtered out, so there is no self-triggering loop. A `RepositoryCredential` generation
  bump enqueues every repository it covers *and* every repository it covered before the change (the URL prefix or
  `secretRef` may have moved), since either set's effective credentials changed without the repository's own
  generation moving.
* **Credentials the probe actually uses:** a repository without its own credentials inherits them from the
  `RepositoryCredential` whose URL is the longest prefix of its own — the same rule the CRD backend applies — so the
  probe cannot report `Ready=True` for a repository manifest generation would fail to read.
* **Credential-change detection:** the controller watches credential Secrets and maps them back to the repositories
  they feed. A `data` change enqueues every repository reading that Secret, directly or through a covering credential,
  since rotation touches neither CR and would otherwise wait for the next periodic probe. The enqueue is smeared with
  the same per-UID phase offset as periodic probes, spread over one test interval: an org-wide covering credential
  rotated on every controlplane at once would otherwise re-test every repository under it everywhere within seconds —
  the per-URL rate limit does not help, since these are distinct URLs. Comparing `data` discards resync-synthetic and
  metadata-only updates. Creation and deletion are handled too (a created Secret can clear
  `CredentialsMissing`), with Adds predating startup ignored so the initial LIST is not read as a fleet-wide rotation.
  This reuses the Secret watch the settings manager already runs in every component, so it adds no watch surface and no
  RBAC. Rotation emits **no** event: every controlplane would emit a duplicate, and the re-test's transition events
  already report the outcome.

#### Events

##### Lifecycle events (emitted by the component performing the write — the API server in crd/hybrid modes)

These go through the existing audit logger with its existing reasons — `ResourceCreated`, `ResourceUpdated`,
`ResourceDeleted` — exactly as Applications and AppProjects do today, so they need no new `--enable-k8s-event`
allowlist entries and `argocd repo rm` in crd/hybrid mode is audited like any other delete. The event is on the
`Repository` or `RepositoryCredential` object; a hybrid-mode migration creating the CR emits `ResourceCreated`. A
rotation of a referenced Secret's contents is detected and re-tested (see above) but not announced.

Emitted by the write path rather than the controller: an informer-driven emitter would re-announce every existing
resource on each controller restart, and every controlplane would duplicate events for a single write.

##### Connection health events (emitted by the repository controller on state *transitions* of its own entry)

New reasons. The default `--enable-k8s-event` allowlist is `all`, so they are emitted unless an operator has narrowed
the list, in which case they must be added explicitly.

  - `ConnectionSuccessful`: repository became accessible (first result, or from unknown state)
  - `ConnectionFailed`: cannot connect to repository (unclassified failure)
  - `ConnectionRecovered`: connection restored after failure

##### Credential events (emitted by the repository controller, classified from the test failure)
  - `CredentialsInvalid`: authentication failed
  - `CredentialsMissing`: referenced secret not found

There is deliberately no `CredentialsValid` event: a successful test cannot distinguish "credentials verified" from
"anonymous access allowed", and recovery is already covered by `ConnectionRecovered`.

#### RBAC

* application controller: `repositories`/`repositorycredentials` get/list/watch/update/patch plus
  `repositories/status` update/patch (sole status writer)
* API server: full CRUD on both resources (backs `argocd repo add`/`repo rm`/etc. in crd/hybrid modes)
* notifications controller and applicationset controller: get/list/watch (they only resolve repositories and
  credentials)

### Detailed examples

**Example Repository Manifests:**

**Git:**

```yaml
apiVersion: argoproj.io/v1alpha0
kind: Repository
metadata:
  name: my-git-repo
  namespace: argocd
spec:
  url: https://github.com/example/repo
  type: git
  project: default
  secretRef:
    name: github-creds
  git:
    enableLFS: true
    depth: 1  # shallow clone
```

**Helm:**

```yaml
apiVersion: argoproj.io/v1alpha0
kind: Repository
metadata:
  name: my-helm-repo
  namespace: argocd
spec:
  url: https://charts.example.com
  type: helm
  helm:
    name: example-charts  # Helm repo alias, resolvable from Chart.yaml dependencies
    enableOCI: false
  secretRef:
    name: helm-creds
```

**OCI:**

```yaml
apiVersion: argoproj.io/v1alpha0
kind: Repository
metadata:
  name: my-oci-repo
  namespace: argocd
spec:
  url: oci://registry.example.com/charts
  type: oci
  oci:
    insecureSkipTLS: false
  secretRef:
    name: oci-creds
```

**Credential template (matches repositories by URL prefix):**

```yaml
apiVersion: argoproj.io/v1alpha0
kind: RepositoryCredential
metadata:
  name: github-org-creds
  namespace: argocd
spec:
  url: https://github.com/example
  type: git
  git:
    githubAppID: 123
    githubAppInstallationID: 456
  secretRef:
    name: github-app-key
```

**Example status:**

```yaml
apiVersion: argoproj.io/v1alpha0
kind: Repository
metadata:
  name: prod-app-repo
  namespace: argocd
  generation: 3
spec:
  url: https://github.com/company/production-apps
  type: git
  project: production
  secretRef:
    name: git-creds
status:
  # Aggregated connection state across all controlplanes. By default there is only a single controlplane;
  # in agent-based architectures each controlplane contributes an entry and the aggregate rolls them up.
  connectionState:
    status: Degraded  # Successful if all succeed, Failed if all fail, Degraded if mixed
    message: "1 of 2 clusters connected successfully"
    attemptedAt: "2026-07-12T09:42:11Z"  # when the aggregate last CHANGED (ap-southeast-1 failing), not last recomputed
    totalClusters: 2
    successfulClusters: 1
    failedClusters: 1

  # One entry per controlplane, server-side-applied under its --controlplane-name (also the field manager).
  # Written on transition, so attemptedAt moves on a change or at most one refresh period after the last write.
  clusterConnectionStates:
    - name: "argocd-application-controller"  # the only controlplane by default (or the hub in hub 'n' spoke)
      connectionState:
        status: Successful
        message: "Repository connection test successful"
        attemptedAt: "2026-07-12T09:58:41Z"  # within one refresh period, so this verdict is current
        lastTransitionTime: "2026-07-01T08:14:22Z"  # unchanged for days: first candidate for eviction
        observedGeneration: 3
    - name: "ap-southeast-1-workload"
      connectionState:
        status: Failed
        reason: CredentialsInvalid  # per-entry, so a reclassification is a transition, not a message-only change
        message: "Repository connection test failed: authentication failed"
        attemptedAt: "2026-07-12T10:00:05Z"
        lastTransitionTime: "2026-07-12T09:42:11Z"  # failing entries are pinned regardless of age
        observedGeneration: 2  # has not yet re-tested the latest spec

  conditions:  # []metav1.Condition, listType=map keyed by type
    - type: Ready
      status: "False"  # derived from the aggregate, so all controlplanes converge on the same value
      lastTransitionTime: "2026-07-12T09:42:11Z"  # moves with the aggregate
      reason: ConnectionFailedInSomeClusters
      message: "1 of 2 clusters connected successfully"
      observedGeneration: 2  # the minimum across reporting entries
```

### Security Considerations

* Credential material never enters the CRD: the spec is safe to store in Git, and the referenced Secret carries only
  the sensitive keys.
* **`spec.secretRef` is a privilege boundary, gated by a label.** Today `argocd repo add` without credentials already
  inherits the longest-prefix credential template (`CreateRepository` copies them before the connection test, and the
  prefix match has no path boundary, so a template for `https://github.com/example` also covers
  `https://github.com/example-evil`), so registering a URL under a template already exercises credentials the caller
  does not hold. That exposure is unchanged. `secretRef` adds a *new* one: naming any Secret in the control-plane
  namespace. Unconstrained, anyone permitted to create a `Repository` — but not to read Secrets — could make Argo CD
  read a Secret they cannot see and send it to a URL they control. A reference is therefore honoured only when the
  target carries an Argo CD `secret-type` label (`repository-creds`, or the pre-existing `repository`/`repo-creds`),
  making exposure an explicit act by the Secret's owner rather than a consequence of granting `repositories` create.
  Labelling a Secret is equivalent to publishing it to every principal who can create a `Repository`, and is documented
  as such. The gate closes only the `secretRef` path; template inheritance keeps its current semantics.
* `status` is a subresource: applying a manifest containing a `status` block cannot influence it (the API server strips
  it), and writing it requires the `repositories/status` grant only the application controller holds.
* Read and write (push) credentials are distinct objects distinguished by `spec.write` and filtered on in every lookup,
  so hydrator push credentials can neither shadow nor be served in place of read credentials.
* Each controlplane writes status under its own field manager. A compromised controlplane can overwrite the shared
  aggregate/condition (any status writer can), but per-entry ownership means it cannot silently masquerade as another
  controlplane without taking SSA ownership of its entry.

### Risks and Mitigations

* **Two writers of repository configuration during migration.** Mitigated by strict precedence (CRDs win in hybrid) and,
  for Argo-CD-generated Secrets, by migration removing the legacy Secret. A user-provided Secret is kept by design, so a
  repository can legitimately have both representations; `hybrid`/`crd` components are unaffected, but anything still in
  `secret` mode serves whichever copy the Secret holds.
* **Status write amplification.** The main scaling risk, and it lands entirely on the API server holding the
  objects. Restamping every probe at 50 controlplanes × 200 repositories × 2 applies every 3m is ~110 writes/s; the
  fan-out is worse, since every write bumps `resourceVersion` and wakes every watcher (API server, applicationset
  controller, notifications controller, and each controlplane's own informer), so thousands of watch events/s.
  Transition-only writes with a 15m refresh put the steady-state floor at ~11 writes/s for the same fleet (one apply
  per refresh — the second merges to a no-op), with changes above it proportional to verdict transitions rather than
  probes. `--repo-controller-test-interval 0` disables probing entirely.
* **Probe load on upstream forges.** Independently of the API server, 200 repositories at 3m is ~4,000 connection tests
  per hour per controlplane, multiplied by the fleet where controlplanes share credentials.
* **Probe storms from failing fleets.** Observations are deduplicated by the work queue and rate-limited per repository
  URL, repeatedly failing repositories back off, and credential-rotation re-tests are smeared over a test interval.
* **Controlplane name collisions.** Two *separate* controlplanes sharing a `--controlplane-name` would fight over one
  entry and each report the other's verdict as its own. Replicas of one installation sharing it is intended, not a
  collision. Documented as a uniqueness requirement on the flag.

### Upgrade / Downgrade Strategy

* **Upgrading** requires applying the new CRDs (part of the install manifests) but changes no behavior: every component
  defaults to `secret`, and the controller only starts in crd/hybrid modes.
* **There is no bulk migration.** Repositories become CRDs one at a time, when something updates them, so `hybrid` is
  the only safe way in. Going straight from `secret` to `crd` makes every unmigrated repository invisible (no Secret
  fallback) and their applications fail to resolve sources.
* **Migration progress is observable.** Since repositories migrate only when touched, "how many are left" needs to be
  answerable by inspection, since without an answer an operator in `hybrid` cannot know whether moving to `crd` is safe.
  The API server exports a gauge of repositories resolved per backend, and `argocd repo list` gains a column reporting
  which store each came from.
* **Downgrading** in `secret` mode is safe. After running `hybrid`, updated repositories have their non-secret
  configuration in a CRD a downgraded version cannot read, and an Argo-CD-generated legacy Secret has been replaced by
  one owned by the CR, so migrated repositories must first be exported back to Secrets: `argocd admin repo export`
  emits a legacy-shaped Secret per `Repository`/`RepositoryCredential`, merging the spec with the referenced Secret's
  credential keys, to be applied before switching the mode back. Repositories with user-provided Secrets keep them, but
  not the configuration that moved into the spec, so they need exporting too. Downgrading from `crd` mode has the same
  constraint for all repositories.

## Drawbacks

* It will be more difficult to reason about how a specific repository credential gets selected. There could be
  scenarios where a repository is defined both in a CR and in a secret. While the CR in this proposal takes
  precedence, it might still be challenging for an admin or a user to understand why and how a credential is
  applied.
* In most cases we will now need _both_ a secret _and_ a CR for a repository credential to work. While we can
  provide warnings for this, it might still be an annoyance compared to the status quo.
* Each additional controlplane multiplies periodic status writes on every repository; the status subresource keeps
  this away from spec watchers (no generation bumps), but it is still audit-log and etcd churn.

## Alternatives

* It can be argued that secret-based Repositories has worked well up until now and doesn't need to be changed.
* Alternatively, we could have a single `Repository` CRD instead of having the `Repository`/`RepositoryCredential`
  split. In that case we would need to define how a repository credential template is defined in the context of a
  `Repository` (perhaps with a field denoting it to be a credential template)
