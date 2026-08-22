---
title: Allow Repositories to be stored as CRDs
authors:
  - @blakepettersson
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2025-12-10
last-updated: 2026-08-22
---

# Repository CRD

Allow ArgoCD repositories to be stored as CRs, while maintaining full backwards compatibility with the existing
secret-based implementation.

## Open Questions

* **API version, and the `Repository` name collision behind it.** `argoproj.io/v1alpha0` is an unconventional first
  version; it is used because `pkg/apis/application/v1alpha1` already declares a `Repository` Go type (the config
  struct serialized into Secrets and exposed over gRPC/REST), so codegen cannot add a second one there. The clash is
  purely the Go identifier — `v1alpha1.Repository` is not a `runtime.Object` and that kind is not registered in the
  scheme — and `RepositoryCredential` is unaffected. `v1alpha0` defers rather than resolves it: promoting to
  `v1alpha1` needs the colliding name. Options, cheapest first:
  - **Skip `v1alpha1` on promotion** (`v1alpha0` → `v1alpha2`/`v1beta1` → `v1`): free, but the odd version stays.
  - **New group** `repository.argoproj.io/v1alpha1`: frees the name, keeps kind `Repository` and `kubectl get
    repositories`; small codegen change, but a second group to maintain and an `apiGroups` entry in every Role.
  - **Different kind** (e.g. `RepositoryConfig` in `argoproj.io/v1alpha1`): no collision, no new group, loses the
    `kubectl get repositories` UX.
  - **Rename the existing `Repository` struct**: easy-peasy but is technically a breaking change
* **Per-controlplane status above the retention cap.** Eviction keeps `clusterConnectionStates` bounded, but every
  controlplane still probes and still wants a slot, so once the fleet exceeds the cap the retained set churns on
  each interval: an evicted controlplane re-claims a slot on its next probe and evicts someone else, costing writes
  on every repository forever. The alternative is that success stops claiming a slot at all — a controlplane writes
  an entry only while it has something to report — which makes the list scale with the number of *unhealthy*
  controlplanes rather than with the fleet, and fits large agent-based deployments far better. It gives up two
  properties universal entries provide, though: the `attemptedAt` heartbeat that drives stale-entry GC, and the
  `Ready` condition's `observedGeneration` minimum, which can only speak for controlplanes that have an entry. What
  replaces those two if entries become failure-only?
* **Cache-backed reads.** The CRD backend currently resolves repositories with direct (uncached) LIST calls, one per
  lookup. The repository controller already maintains an informer; sharing a lister with the DB backend would remove
  the remaining per-reconcile API-server load.

## Summary

Argo CD stores repository configuration and credentials in Kubernetes Secrets. This proposal introduces two
namespaced CRDs — `Repository` and `RepositoryCredential` (API version `argoproj.io/v1alpha0`) — that hold all
*non-secret* repository configuration declaratively, while credential material remains in a Secret referenced via
`spec.secretRef`. Storage is selected per component by a `--repository-backend-mode` flag (env
`ARGOCD_REPOSITORY_BACKEND`) with three modes: `secret` (default, existing behavior), `crd` (CRDs only) and `hybrid`
(read from both with CRDs taking precedence; writes migrate to CRDs). A new repository controller — running inside
the application controller when the crd/hybrid mode is enabled — periodically tests repository connectivity and
maintains a rich, multi-controlplane-aware status on the `Repository` resource.

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
   repository controller. The CRD schema groups fields by type but maps onto the same structs.
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
  flag whose default comes from `ARGOCD_REPOSITORY_BACKEND`.
* `db.NewDB` validates the mode/clientset pairing once: crd/hybrid without an application clientset downgrades to
  the secrets backend with a warning instead of failing at first use.
* Components that only read *cluster* data (sharding, several admin commands) intentionally stay on the secrets
  backend regardless of mode — cluster storage is unaffected by this proposal. `argocd admin repo generate-spec`
  also intentionally keeps emitting Secret manifests, since that is the command's purpose.

#### Storage model

* The CRD spec carries all non-secret configuration, including fields that are stored inside the secret today when
  using the secrets backend: display `name`, `project`, proxy settings, `forceHttpBasicAuth`,
  `useAzureWorkloadIdentity`, per-type blocks (`git.enableLFS`, `git.depth`, `git.githubAppID`,
  `git.githubAppInstallationID`, `git.githubAppEnterpriseBaseUrl`, `helm.name`, `helm.enableOCI`,
  `oci.insecureSkipTLS`, …) and the `write` marker.
* The Secret referenced by `spec.secretRef` carries *only* credential material: `username`, `password`,
  `bearerToken`, `sshPrivateKey`, `tlsClientCertData`/`tlsClientCertKey`, `githubAppPrivateKey`,
  `gcpServiceAccountKey`, and the Azure service-principal fields. A `secretRef` is only written when credential
  material actually exists, so e.g. an Azure-workload-identity-only repository has no dangling reference.
* Repositories are looked up by URL + project (with an empty-project wildcard fallback), not by resource name, so
  declaratively-created resources may use any name.

#### Repository controller

* Runs inside the application controller process when the backend mode is `crd` or `hybrid`. Flags:
  `--repo-controller-workers`, `--repo-controller-test-interval` (default 3m), `--repo-controller-test-timeout`
  (default 30s), `--repo-controller-status-entry-ttl` (default 1h, `0` disables pruning) and `--controlplane-name`
  (env `ARGOCD_CONTROLPLANE_NAME`; must be unique for each application controller writing repository status to the
  same control plane).
* The repository controller is the **only component** that writes `Repository` status — no other component does,
  and everything else (the API server, reconciliation) only reads it or feeds the controller's work queue. In
  agent-based architectures there is one such controller per controlplane, each writing its own entry. Each
  periodic probe invokes the repo-server's `TestRepository` with the repository converted through the shared
  conversion (i.e. exactly the credentials manifest generation would use).
* **Status writes use server-side apply, one field manager per controlplane** — the manager being the
  controlplane's `--controlplane-name`, which is also its `clusterConnectionStates` entry key. SSA rather than
  read-modify-write of the status subresource: GET/mutate/UPDATE makes every controlplane contend for the whole
  object and a lost race silently drops a peer's entry, whereas an apply carries only this controlplane's fields and
  the server merges. Every apply targets the `status` subresource with `force: true`; the only competing writers are
  peers running this same code. Each probe writes twice:
  1. apply only this controlplane's `clusterConnectionStates` entry (`listType=map` keyed by `name`, so the API
     server merges entries across controlplanes); the patch response is the authoritative post-merge object;
  2. roll the aggregate `connectionState` and the `Ready` condition up from that response and apply them.
  Rolling up from the apply response rather than the informer cache means any controlplane can own a write without
  regressing the aggregate to a stale view.
* Two SSA properties the design leans on: **omission deletes** — fields a manager stops asserting are removed by the
  server, which is what makes prune and eviction possible (force-adopt the doomed entry, then apply without it) and
  why each apply must assert every field this controlplane still owns; and the **aggregate and `Ready` are shared,
  last-writer-wins**, deliberately not partitioned per manager, since both derive from the merged entry list and so
  converge regardless of write order. Controlplane-specific detail stays in the entry that controlplane owns. Each
  writing controlplane also adds a `metadata.managedFields` entry — part of why the list needs a cap.
* Test failures are classified into condition reasons: gRPC `Unauthenticated`/`PermissionDenied` and well-known
  git/SSH/HTTP authentication failure messages map to `CredentialsInvalid`; a spec referencing a nonexistent
  credentials Secret maps to `CredentialsMissing`; everything else is `ConnectionFailed`.
* The `Ready` condition is derived from the **aggregate**, not the local test result, so all controlplanes converge
  on the same value (`Degraded` → `Ready=False`/`ConnectionFailedInSomeClusters`). Its `observedGeneration` is the
  minimum across the entries that report one: `Ready=True` at the current generation means every controlplane that
  reports an `observedGeneration` has verified the current spec. Entries that report none (written before the field
  existed) cannot be judged and are skipped, so they do not hold the minimum down.
* Each `clusterConnectionStates` entry carries its own `observedGeneration` — the generation of the spec that
  controlplane most recently evaluated — so a verdict that predates a spec change (e.g. a credential rotation) is
  distinguishable from a failure against the current spec.
* **Prompt re-tests from real traffic:** connection-type failures during manifest generation (gRPC `Unavailable`,
  `Aborted`, `Unauthenticated`, `PermissionDenied`) are reported by the application controller's reconciliation
  path to the repository controller as *observations*, which enqueue an immediate re-test (rate-limited to one per
  repository URL per 30s). The controller remains the only status writer; observations only feed its work queue.
  Recovery is detected by the next periodic probe.
* **Stale-entry garbage collection:** the periodic probe doubles as a heartbeat — every controlplane refreshes its
  entry's `attemptedAt` each interval regardless of test outcome. Entries from controlplanes that have stopped
  reporting for longer than the status-entry TTL (decommissioned clusters) are pruned by whichever controlplane
  notices, using a two-step apply (force-adopt the abandoned entries, then apply without them). The TTL must
  comfortably exceed every controlplane's test interval.
  Under a capped list both steps must assert *only* the entries being removed: asserting `stale + 1` items means a
  controlplane holding no slot is rejected by the very apply that would have freed one, so a list full of dead names
  could never be pruned by anyone. The pruner asserts its own entry after the drop.
* **Bounded status:** one entry per reporting controlplane means an unbounded `clusterConnectionStates` grows with
  the fleet, and a large agent-based deployment can have hundreds or thousands of controlplanes. The list therefore
  has a hard `maxItems` (100) and each entry's `message` a `maxLength` of 1024 (the controller truncates; a
  repo-server error is otherwise unbounded), capping a worst-case status near 130 KiB — inside etcd's ~1.5 MiB
  per-object limit with room for the `managedFields` entries. The cap is a *retention budget*, not a supported fleet
  size: above it the per-entry detail is a sample and the aggregate is the fleet-wide answer.
* **Retention policy: evict the least-recently-*changed* entry.** A steady-state `Successful` entry says nothing the
  aggregate does not, so a controlplane needing a slot in a full list evicts the entry unchanged the longest. This
  requires:
  - **A recency key that is not `attemptedAt`** — the heartbeat keeps that fresh on every probe. Each entry gains a
    `lastTransitionTime`, moving only when its `status` changes, and eviction orders on that.
  - **Failures pinned.** A `Failed` entry is never evicted for a `Successful` one, however stale — failing
    controlplanes are why per-entry detail exists. Ranking is `Successful` first, oldest `lastTransitionTime` first,
    entry name as tiebreak so concurrent writers pick the same victim.
  - **An aggregate that does not silently undercount.** `totalClusters`/`successfulClusters`/`failedClusters` derive
    from the entries, so once entries are evicted they describe the retained sample; a `truncated: true` marker
    keeps "2 of 2 clusters" distinguishable from "2 of 2 *retained* clusters".
  Eviction reuses the two-step apply from stale-entry GC — the victim's entry belongs to another field manager, so
  it must be force-adopted before it can be dropped. Eviction bounds the object but does not by itself make a fleet
  larger than the cap behave well; see the open question above.
* Spec changes (generation bumps) trigger prompt re-tests via the informer; the controller's own status writes do
  not bump the generation and are filtered out, so there is no self-triggering write loop.
* **Credentials the probe actually uses:** a repository without its own credentials inherits them from the
  `RepositoryCredential` whose URL is the longest prefix of its own (the same longest-prefix rule the CRD backend
  applies), so the probe cannot report `Ready=True` for a repository that manifest generation would fail to read.
* **Credential-change detection:** the controller also watches credential Secrets, mapping them back to the repositories
they feed:
  - a Secret whose `data` changes enqueues every repository that reads it, directly or through a covering
    credential — rotation touches neither CR, so otherwise the new material waits for the next periodic probe.
    Comparing `data` discards resync-synthetic updates and metadata-only writes. Creation and deletion are handled
    too (a created Secret can clear `CredentialsMissing`), with Adds predating startup ignored so the informer's
    initial LIST is not read as a fleet-wide rotation.
  This reuses the Secret watch the settings manager already runs in every component — every non-cluster Secret in
  the control-plane namespace — so it adds no watch surface and no RBAC. Rotation emits **no** event: every
  controlplane would emit its own duplicate, and the re-test's transition events already report the outcome.

#### Events

##### Lifecycle events (emitted by the component performing the write — the API server in crd/hybrid modes)
  - `RepositoryCreated`: new `Repository` registered (including a hybrid-mode migration creating the CRD)
  - `RepositoryUpdated`: `Repository` specification changed
  - `CredentialsCreated`: new `RepositoryCredential` registered
  - `CredentialsUpdated`: `RepositoryCredential` changed — the resource only; a rotation of the referenced Secret's
    contents is detected and re-tested (see above) but not announced

Emitted by the write path rather than the controller: an informer-driven emitter would re-announce every existing
resource on each controller restart, and every controlplane would duplicate events for a single write.

##### Connection health events (emitted by the repository controller on state *transitions* of its own entry)
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
  name: example-charts
  helm:
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
    attemptedAt: "2026-07-12T10:00:05Z"  # stamped at rollup, so never older than the entry that triggered it
    totalClusters: 2
    successfulClusters: 1
    failedClusters: 1

  # Per-controlplane connection states. Each controlplane server-side-applies only its own entry, using its
  # --controlplane-name as both the entry key and the field manager. observedGeneration records which spec
  # generation the verdict applies to, so stale verdicts are distinguishable after a spec change. attemptedAt is
  # refreshed by every probe (it is the heartbeat); lastTransitionTime moves only when status changes, and is what
  # eviction ranks on once the list is at its maxItems cap.
  clusterConnectionStates:
    - name: "argocd-application-controller"  # the only controlplane by default (or the hub in hub 'n' spoke)
      connectionState:
        status: Successful
        message: "Repository connection test successful"
        attemptedAt: "2026-07-12T10:00:00Z"
        lastTransitionTime: "2026-07-01T08:14:22Z"  # unchanged for days: the first candidate for eviction
        observedGeneration: 3
    - name: "ap-southeast-1-workload"
      connectionState:
        status: Failed
        message: "Repository connection test failed: authentication failed"
        attemptedAt: "2026-07-12T10:00:05Z"
        lastTransitionTime: "2026-07-12T09:42:11Z"  # failing entries are pinned regardless of age
        observedGeneration: 2  # has not yet re-tested the latest spec

  conditions:
    - type: Ready
      status: "False"  # derived from the aggregate, so all controlplanes converge on the same value
      lastTransitionTime: "2026-07-12T10:00:05Z"
      reason: ConnectionFailedInSomeClusters
      message: "1 of 2 clusters connected successfully"
      observedGeneration: 2  # the minimum across reporting entries
```

### Security Considerations

* Credential material never enters the CRD: the spec is safe to store in Git, and the referenced Secret carries only
  the sensitive keys.
* `status` is a subresource. Applying a `Repository` manifest — even one containing a `status` block — cannot
  influence status; the API server strips it. Writing status requires an explicit `repositories/status` grant,
  which only the application controller holds.
* Read and write (push) credentials are distinct objects distinguished by `spec.write` and are filtered on in every
  lookup, so registering hydrator push credentials can neither shadow nor be served in place of read credentials.
* In agent-based architectures, each controlplane writes status under its own field manager; a compromised or
  misconfigured controlplane can overwrite the shared aggregate/condition (any status writer can), but per-entry
  ownership means it cannot silently *masquerade* as another controlplane without taking SSA ownership of its entry.

### Risks and Mitigations

* **Two writers of repository configuration (CRD + legacy Secret) during migration.** Mitigated by strict
  precedence (CRDs win in hybrid mode) and by the migration deleting the legacy Secret once the CRD is created, so
  the overlap is normally a single update. That delete is best-effort and not retried, so a repository can keep both
  representations; `hybrid`/`crd` components are unaffected, but anything still in `secret` mode serves the stale
  copy.
* **Status write amplification.** Every controlplane probes every repository each interval (default 3m) and writes
  two status patches. This is also the heartbeat that makes stale-entry GC possible; the interval and worker count
  are tunable.
* **Probe storms from failing fleets.** Reconciliation-driven re-test observations are deduplicated by the work
  queue and rate-limited per repository URL.
* **Controlplane name collisions.** Two controllers sharing a `--controlplane-name` would fight over one status
  entry. Documented as a hard uniqueness requirement on the flag.

### Upgrade / Downgrade Strategy

* **Upgrading** requires applying the new CRDs (they are part of the install manifests) but changes no behavior:
  every component defaults to the `secret` backend, and the repository controller only starts in crd/hybrid modes.
* **There is no bulk migration.** Repositories become CRDs one at a time, when something updates them, so `hybrid`
  is the only safe way in: it reads both stores. Going straight from `secret` to `crd` makes every unmigrated
  repository invisible (no Secret fallback) and their applications fail to resolve sources. A one-pass migration
  command is left to a follow-up.
* **Downgrading** while still in `secret` mode is safe. After running in `hybrid` mode, note that updating a
  repository migrates it to a CRD **and deletes the legacy Secret** — so before downgrading to a version without
  CRD support, migrated repositories must be exported back to secrets (or re-created). Downgrading from `crd` mode
  has the same constraint for all repositories.

## Drawbacks

* It will be more difficult to reason about how a specific repository credential gets selected. There could be
  scenarios where a repository is defined both in a CR and in a secret. While the CR in this proposal takes
  precedence, it might still pose challenging for an admin or a user to understand why and how a credential is
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
