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

* **Promotion path from `v1alpha0`.** The CRDs are introduced under the `argoproj.io/v1alpha0` API version to make
  clear they are experimental. The criteria and mechanics for promoting to `v1alpha2`+ (including any conversion
  story) are not yet defined.
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
    - No breaking changes to CLI commands (`repo add`, etc.)
    - No changes to gRPC/REST API contracts
    - `hybrid` mode reads from both CRDs and Secrets simultaneously, with CRDs taking precedence

3. **Gradual Migration**
    - Operators can migrate at their own pace by opting into `hybrid` mode
    - Automatic migration on update: updating a secret-backed repository in hybrid mode creates the CRD and then
      deletes the legacy Secret, so stale credentials do not linger in the old store. The delete is best-effort —
      the CRD already takes precedence, so a failed delete is logged rather than failing the update, and leaves a
      Secret that components still in `secret` mode keep serving until it is removed by hand
    - Every component that resolves repositories (API server, application controller, applicationset controller,
      notifications controller) takes the same flag/env, so the fleet can be switched consistently

4. **Security Parity**
    - Credentials stored in Secret references (not in CRD spec); the referenced Secret carries *only* credential
      material, everything else lives in the spec
    - Status is a subresource: client-side or server-side applies of the resource cannot influence status, and
      writing it requires an explicit `repositories/status` RBAC grant (held only by the application controller)
    - Read (`spec.write: false`) and write/push (`spec.write: true`, used by the source hydrator) repositories and
      credentials are distinct objects, and every lookup filters on the marker so the two credential sets can never
      shadow each other
    - Same encryption-at-rest guarantees; no reduction in security posture

### Non-Goals

1. **Changing Internal API Types**
    - `pkg/apis/application/v1alpha1.Repository` remains unchanged for now
    - Internal code continues using same structs; a single shared conversion
      (`util/db/repository_crd_conversion.go`) maps CRD ↔ internal types for both the DB layer and the repository
      controller
    - This is purely an external storage change
    - **Note:** CRD schema organizes fields differently (nested by type) but maps to same internal types

2. **Immediate Secret Deprecation**
    - Secret-based storage remains supported indefinitely and remains the default
    - No forced migration in this phase
    - Future deprecation would be separate proposal

3. **API Contract Changes**
    - CLI commands (`argocd repo add`) behavior unchanged
    - gRPC/REST API remains identical
    - Client SDKs unaffected

4. **Multi-Tenancy Redesign**
    - Project scoping mechanism unchanged
    - Namespace isolation (aka "repositories-in-any-namespace") is not addressed here
    - Future work could build on this foundation

5. **Workload identity support**
    - There are already some ad-hoc workload identity solutions present, which will be unchanged for this proposal
      (`spec.useAzureWorkloadIdentity` is carried through, as with secrets today)
    - Future work could build on this foundation for a more holistic solution

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
  flag whose default comes from `ARGOCD_REPOSITORY_BACKEND`; nothing below the commands reads the environment. An
  explicit flag always wins over the env var. Invalid or unset values fall back to `secret`.
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
* The controller is the **single writer** of `Repository` status. Each periodic probe invokes the repo-server's
  `TestRepository` with the repository converted through the shared conversion (i.e. exactly the credentials
  manifest generation would use).
* Status is written in two server-side applies, both under the controlplane's field manager:
  1. apply only this controlplane's `clusterConnectionStates` entry (`listType=map` keyed by `name`, so the API
     server merges entries across controlplanes); the patch response is the authoritative post-merge object;
  2. roll the aggregate `connectionState` and the `Ready` condition up from that response and apply them.
  Computing the rollup from the apply response rather than the informer cache means any controlplane can own a
  write without regressing the aggregate to a stale view.
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
  With a capped list the two steps must assert *only* the entries being removed, not the pruner's own entry
  alongside them: asserting `stale + 1` items means a controlplane that does not yet hold a slot is rejected by the
  very apply that would have freed one, and a list whose slots all belong to dead names could then never be pruned
  by anyone — leaving that `Repository`'s status permanently unwritable. The pruner asserts its own entry only after
  the drop, so it never needs a slot it does not already own.
* **Bounded status:** `clusterConnectionStates` holds one entry per reporting controlplane, so left unbounded the
  `Repository` object grows with the fleet — and a large agent-based deployment can genuinely have hundreds or
  thousands of controlplanes, well past what one object can hold. The list therefore has a hard `maxItems` (100),
  and each entry's `message` a `maxLength` of 1024 (the controller truncates before writing, since a repo-server
  error can be arbitrarily long), which caps a worst-case status at roughly 130 KiB — inside etcd's ~1.5 MiB
  per-object limit with room for the `managedFields` entry every writing controlplane also adds. The cap is a
  *retention budget*, not a supported fleet size: above it, per-controlplane detail is a sample and the aggregate is
  the fleet-wide answer.
* **Retention policy: evict the least-recently-*changed* entry.** A steady-state `Successful` entry carries no
  information the aggregate does not already give, so when a controlplane needs a slot in a full list it evicts the
  entry whose state has been unchanged the longest. Three things this requires:
  - **A recency key that is not `attemptedAt`.** The heartbeat refreshes `attemptedAt` on every probe, so it is
    always fresh and useless for ranking. Each entry gains a `lastTransitionTime` — when that entry's `status` last
    *changed* — which is what eviction orders on.
  - **Failures are pinned.** A `Failed` entry is never evicted to make room for a `Successful` one, however stale
    its transition: the failing controlplanes are the reason the per-entry detail exists. Eviction therefore ranks
    `Successful` entries first (oldest `lastTransitionTime` first, entry name as tiebreak so concurrent writers pick
    the same victim), and only considers `Failed` entries when every retained entry is already failing.
  - **The aggregate must not silently undercount.** `totalClusters`/`successfulClusters`/`failedClusters` are
    derived from the entries, so once entries are evicted they describe the retained sample, not the fleet. The
    aggregate carries an explicit marker (`truncated: true`) so a consumer can tell "2 of 2 clusters" from "2 of 2
    *retained* clusters", rather than reading an evicted fleet as a healthy one.
  Eviction reuses the two-step apply from stale-entry GC above — a victim's entry is owned by another field
  manager, so it must be force-adopted before it can be dropped. Eviction bounds the object, but it does not by
  itself make a fleet larger than the cap behave well — see the open question below.
* Spec changes (generation bumps) trigger prompt re-tests via the informer; the controller's own status writes do
  not bump the generation and are filtered out, so there is no self-triggering write loop.
* **Credentials the probe actually uses:** a repository without its own credentials inherits them from the
  `RepositoryCredential` whose URL is the longest prefix of its own (the same longest-prefix rule the CRD backend
  applies), so the probe cannot report `Ready=True` for a repository that manifest generation would fail to read.
* **Credential-change detection:** the controller also watches `RepositoryCredential` and the credential Secrets,
  and maps both back to the repositories they feed:
  - a `RepositoryCredential` spec change enqueues every repository its URL covers — both before and after the
    change, so a moved `spec.url` re-tests the repositories it stopped covering as well;
  - a Secret whose *contents* change enqueues every repository that reads it, directly via `spec.secretRef` or
    through a covering credential. Rotating a Secret touches neither CR, so without this the new material would
    only be verified at the next periodic probe. Only `data` changes count: comparing it discards the informer
    resync's synthetic updates and metadata-only writes by other controllers. Secret creation and deletion are
    handled too (a created Secret can clear `CredentialsMissing`; a deleted one should report it promptly), with
    Adds that predate controller startup ignored so the informer's initial LIST is not read as a fleet-wide
    rotation.
  The Secret watch is the one the settings manager already runs in every component — every non-cluster Secret in
  the control-plane namespace — so this adds no watch surface, and no RBAC beyond the `secrets` get/list/watch the
  application controller already holds. Rotation deliberately emits **no** event: every controlplane watching the
  Secret would emit its own duplicate, and the re-test's own transition events (`CredentialsInvalid`,
  `ConnectionRecovered`) already report the outcome operators act on.

#### Events

##### Lifecycle events (emitted by the component performing the write — the API server in crd/hybrid modes)
  - `RepositoryCreated`: new `Repository` registered (including a hybrid-mode migration creating the CRD)
  - `RepositoryUpdated`: `Repository` specification changed
  - `CredentialsCreated`: new `RepositoryCredential` registered
  - `CredentialsUpdated`: `RepositoryCredential` changed

Lifecycle events are deliberately emitted by the write path rather than the repository controller: an
informer-driven emitter would re-announce every existing resource on each controller restart, and in
multi-controlplane setups every controlplane would emit duplicates for a single write.

##### Connection health events (emitted by the repository controller on state *transitions* of its own entry)
  - `ConnectionSuccessful`: repository became accessible (first result, or from unknown state)
  - `ConnectionFailed`: cannot connect to repository (unclassified failure)
  - `ConnectionRecovered`: connection restored after failure

Note that `CredentialsUpdated` covers changes to the `RepositoryCredential` resource only. A rotation of the
referenced Secret's contents is detected and re-tested (see the repository controller above) but not announced as a
lifecycle event, because the detection is informer-driven and would duplicate per controlplane.

##### Credential events (emitted by the repository controller, classified from the test failure)
  - `CredentialsInvalid`: authentication failed
  - `CredentialsMissing`: referenced secret not found

There is deliberately no `CredentialsValid` event: a successful connection test cannot distinguish "credentials
verified" from "repository allows anonymous access", and recovery from a credentials problem is already covered by
`ConnectionRecovered`.

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
  in the normal case the overlap window is a single update. Because that delete is best-effort and is not retried,
  a repository can be left with both representations; components in `hybrid`/`crd` mode are unaffected (the CRD
  wins), but any component still in `secret` mode keeps serving the stale copy.
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
* **There is no bulk migration.** Existing secret-backed repositories become CRDs one at a time, when something
  updates them. `hybrid` mode is therefore the only safe way in: it reads both stores, so nothing is lost while the
  fleet converges. Switching a fleet straight from `secret` to `crd` makes every not-yet-migrated repository
  invisible — `crd` mode has no Secret fallback — and the applications using them fail to resolve their sources. A
  migration command that converts every secret-backed repository in one pass is left to a follow-up.
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
