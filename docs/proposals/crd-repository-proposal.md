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
last-updated: 2026-09-11
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
* **Cache-backed reads.** The CRD backend currently resolves repositories with direct (uncached) LIST calls, one per
  lookup. Backing the lookup with a shared informer/lister would remove the remaining per-reconcile API-server load.

## Summary

Argo CD stores repository configuration and credentials in Secrets. This proposal adds two namespaced CRDs —
`Repository` and `RepositoryCredential` (`argoproj.io/v1alpha0`) — holding all *non-secret* configuration
declaratively, with credential material remaining in a Secret referenced via `spec.secretRef`. Storage is selected per
component by `--repository-backend-mode` (env `ARGOCD_REPOSITORY_BACKEND`): `secret` (default, existing behavior),
`crd`, or `hybrid` (read both, CRDs win; writes migrate to CRDs). The CRDs are spec-only: connection health keeps
being reported the way it is today, and a `status` subresource is deferred to a follow-up (see Non-Goals).

## Motivation

- More GitOps-friendly (and secure) declarative management, since we can put (most of) the config in Git
- Richer validations with OpenAPI validations (and possibly webhook-based ones)
- Enhanced security for agent-based architectures
- Enhanced Kubernetes integration (`kubectl get repositories` and friends)

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
    - Read (`spec.write: false`) and write/push (`spec.write: true`, used by the source hydrator) repositories and
      credentials are distinct objects, filtered on in every lookup so the two sets cannot shadow each other
    - Same encryption-at-rest guarantees

### Non-Goals

1. **Changing internal API types.** `pkg/apis/application/v1alpha1.Repository` is unchanged; a single shared
   conversion (`util/db/repository_crd_conversion.go`) maps CRD ↔ internal types. The CRD *spec* groups fields by type
   but maps onto the same structs. The internal `ConnectionState` is untouched and keeps being produced by the
   existing code path (see non-goal 6).
2. **Immediate Secret deprecation.** Secret-based storage stays supported and remains the default; deprecating it
   would be a separate proposal.
3. **API contract changes.** CLI behavior, the gRPC/REST API and client SDKs are all unaffected.
4. **Multi-tenancy redesign.** Project scoping is unchanged, and namespace isolation
   ("repositories-in-any-namespace") is not addressed — though the namespaced CRD shape leaves room for it.
5. **Workload identity.** Existing ad-hoc support is carried through as-is (`spec.useAzureWorkloadIdentity`, as
   with secrets today); a holistic solution is future work.
6. **Repository status.** The CRDs ship spec-only: no `status` subresource and no connection-health controller.
   Connection state keeps being produced exactly as it is today — the API server runs `TestRepository` on a
   connection-state cache miss (`server/repository.List`) — so `argocd repo list` and the UI behave identically in
   every mode. A real status is worth having, especially the multi-controlplane view an agent-based deployment cannot
   get from a per-controlplane cache, but it is a design of its own (write amplification, per-controlplane field
   ownership, retention limits) and is deferred to a follow-up proposal. It can be added backwards compatibly:
   a `status` subresource and its RBAC grant are additive to an existing CRD version, readers finding no status fall
   back to the probe-on-read path above, and nothing in the spec described here changes.

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
  every few minutes per Application. A log line rather than an event: detection happens inside the lookup, which runs
  in four components and re-fires on every resolution, so an event would re-announce the same misconfiguration
  indefinitely. CEL cannot express cross-object uniqueness, so this is detection, not prevention.

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

#### Events

Lifecycle events are emitted by the component performing the write — the API server in crd/hybrid modes — through the
existing audit logger with its existing reasons (`ResourceCreated`, `ResourceUpdated`, `ResourceDeleted`), exactly as
Applications and AppProjects do today. They therefore need no new `--enable-k8s-event` allowlist entries, and `argocd
repo rm` in crd/hybrid mode is audited like any other delete. The event is on the `Repository` or
`RepositoryCredential` object; a hybrid-mode migration creating the CR emits `ResourceCreated`.

Emitting from the write path rather than from an informer avoids re-announcing every existing resource on each
component restart.

#### RBAC

* application controller: `repositories`/`repositorycredentials` get/list/watch
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
* Read and write (push) credentials are distinct objects distinguished by `spec.write` and filtered on in every lookup,
  so hydrator push credentials can neither shadow nor be served in place of read credentials.

### Risks and Mitigations

* **Two writers of repository configuration during migration.** Mitigated by strict precedence (CRDs win in hybrid) and,
  for Argo-CD-generated Secrets, by migration removing the legacy Secret. A user-provided Secret is kept by design, so a
  repository can legitimately have both representations; `hybrid`/`crd` components are unaffected, but anything still in
  `secret` mode serves whichever copy the Secret holds.
* **A misconfigured `Repository` is silent until something uses it.** With no controller probing connectivity, a CR
  applied from Git with bad credentials or an unreachable URL surfaces only when an Application referencing it fails
  to refresh. This is the status quo for declaratively-created repository Secrets, and imperative creation still runs
  its connection test (`argocd repo add`), but it is the main thing deferring status costs.

### Upgrade / Downgrade Strategy

* **Upgrading** requires applying the new CRDs (part of the install manifests) but changes no behavior: every component
  defaults to `secret`.
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

## Alternatives

* It can be argued that secret-based Repositories has worked well up until now and doesn't need to be changed.
* Alternatively, we could have a single `Repository` CRD instead of having the `Repository`/`RepositoryCredential`
  split. In that case we would need to define how a repository credential template is defined in the context of a
  `Repository` (perhaps with a field denoting it to be a credential template)
