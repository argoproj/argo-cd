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
last-updated: 2026-10-19
---

# Repository CRD

Allow ArgoCD repositories to be stored as CRs, while maintaining full backwards compatibility with the existing
secret-based implementation.

## Open Questions

* **Should SCM-specific fields get their own block?** GitHub App fields currently sit under `git`
  (`git.githubAppID`, …). Grouping them per SCM instead (`github.appID`, `github.appInstallationID`, …) reads better
  and leaves room for other providers, at the cost of a second axis (type × SCM) in the spec.

## Summary

Argo CD stores repository configuration and credentials in Secrets. This proposal adds two namespaced CRDs:
`Repository` and `RepositoryCredential` (`argoproj.io/v1alpha1`) which holds all *non-secret* configuration
declaratively, with credential material remaining in a Secret referenced via `spec.secretRef`. Storage is selected
fleet-wide by the `repository.backend` key in `argocd-cm`: `secret` (default, existing behavior), `crd`, or `hybrid`
(read both, CRDs win; writes migrate to CRDs). The CRDs are spec-only: connection health keeps
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
    - Automatic migration on update: updating a secret-backed repository in hybrid mode creates the CR, then handles
      the Secret by ownership (see Secret ownership and lifecycle): an Argo-CD-generated legacy Secret is replaced by
      one owned by the CR and deleted; a user-provided Secret is kept and referenced via `secretRef`, since it is the
      credential the CR points at. The delete is best-effort — a failure is logged rather than failing the update, and
      leaves a Secret that components still in `secret` mode keep serving
    - Every component that resolves repositories (API server, application controller, applicationset controller,
      notifications controller) reads the same `argocd-cm` key, so the fleet switches consistently

4. **Security Parity**
    - Credential material stays in the referenced Secret; everything else lives in the spec
    - Read (`spec.write: false`) and write/push (`spec.write: true`, used by the source hydrator) repositories and
      credentials are distinct objects, filtered on in every lookup so the two sets cannot shadow each other
    - Same encryption-at-rest guarantees

### Non-Goals

1. **Changing internal API types.** The internal repository struct keeps its shape and wire format; it is only
   renamed to free the `Repository` identifier (see API version and naming). A single shared conversion
   (`util/db/repository_crd_conversion.go`) maps CRD ↔ internal types. The CRD *spec* groups fields by type but maps
   onto the same structs. The internal `ConnectionState` is untouched and keeps being produced by the existing code
   path (see non-goal 7).
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
   every mode. A real status is worth having, but it is a design of its own (write amplification, per-controlplane
   field ownership, retention limits etc.) and is deferred to a follow-up proposal. It can be added in a backwards 
   compatible way: a `status` subresource and its RBAC grant are additive to an existing CRD version, readers finding 
   no status fall back to the probe-on-read path above, and nothing in the spec described here changes.

## Proposal

### Use cases

#### Use case 1:
As a user, I would like to manage my repositories using GitOps.

### Implementation Details/Notes/Constraints

#### API version and naming

The CRDs ship as `argoproj.io/v1alpha1`, alongside `Application`, `AppProject` and `ApplicationSet`.
`pkg/apis/application/v1alpha1` already declares `Repository` and `RepositoryList` Go types — the config struct
serialized into Secrets and the gRPC list response — so codegen cannot add CRD kinds with those names. They are
renamed to `RepositoryConfig` and `RepositoryConfigList`, following the precedent of
[#28840](https://github.com/argoproj/argo-cd/pull/28840), which renamed `SyncWindow` to `InlineSyncWindow` to free the
kind name. The rename is Go-source only: JSON field names, the protobuf wire format and REST paths are unchanged, so
the CLI, UI and gRPC/REST clients keep working. Consumers importing the Go type update an identifier; Go types carry
no compatibility guarantee, and a CRD kind is not worth bending around one. `RepoCreds` does not collide and is left
alone.

#### Backend selection

* The storage backend is a single fleet-wide setting, `repository.backend` in `argocd-cm` (`secret` | `crd` |
  `hybrid`). Every component already constructs `db.NewDB` with a `SettingsManager`, so no new plumbing is needed.
* The key is read once when `db.NewDB` runs, not hot-reloaded: switching backends under running reconciles would
  change lookup results mid-refresh, and `crd`/`hybrid` also need the CRD RBAC to be in place. Changing the key
  requires a restart of the affected components, which is documented next to the key. Until the restart completes,
  `secret` and `hybrid` components coexist and the `secret` ones cannot see repositories migrated in that window, so
  the rollout should finish before repositories are updated. An invalid or unset value resolves to `secret` with a 
  warning. `ARGOCD_REPOSITORY_BACKEND` overrides the key when set, for tests and for `argocd admin` invocations that 
  have no `argocd-cm` in reach.
* `db.NewDB` validates the mode/clientset pairing once: crd/hybrid without an application clientset downgrades to
  the secrets backend with a warning instead of failing at first use. The notifications controller's `argoCDService`
  currently builds `db.NewDB` with only a Kubernetes clientset and so would always take this downgrade; it gains the
  application clientset as part of this work.

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
  value `repository-secret` (credential material only, no `url`), or with the existing `repository`/`repo-creds`
  values, which already publish the Secret to Argo CD and so need no second opt-in; this is what lets migration
  reference a user-provided legacy Secret without relabeling it. `repository-secret` is a distinct value because the
  label is single-valued and `repo-creds` Secrets are listed as URL-prefix templates, where a url-less Secret would
  match every URL; it deliberately avoids "creds" in the name, which `repo-creds` already uses for a different thing. 
  Otherwise, anyone able to create a `Repository` could name an arbitrary Secret in the control-plane namespace and have 
  Argo CD send it to a URL of their choosing (see Security Considerations). An existing but unlabeled target reports 
 `CredentialsMissing` rather than being silently ignored.

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
* **A miss is not an error.** When no repository matches in any store, `crd` and `hybrid` return a URL-only
  repository, as the secrets backend does today: credential templates still apply, and public repositories need no
  object at all.

#### Secret ownership and lifecycle

One rule drives both deletion and migration: Argo CD deletes a Secret only if it created it. "Created it" is decided
the way the secrets backend already decides it: legacy Secrets Argo CD generated carry the
`managed-by: argocd.argoproj.io` annotation (`util/db/secrets.go`), and anything without it is user-provided.

* **Generated Secrets** — created when a repository is registered imperatively (`argocd repo add`, UI, API). One per
  `Repository`, named as repository Secret names are derived today, and carrying an owner reference to the CR.
  Deleting the `Repository` garbage-collects it; no finalizer. As a `secretRef` target it is credential-only and
  labeled `repository-secret` (see Storage model).
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
apiVersion: argoproj.io/v1alpha1
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
apiVersion: argoproj.io/v1alpha1
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
apiVersion: argoproj.io/v1alpha1
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
apiVersion: argoproj.io/v1alpha1
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
  target carries an Argo CD `secret-type` label (`repository-secret`, or the pre-existing `repository`/`repo-creds`),
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
* **Uncached lookups add API-server load.** The CRD backend resolves repositories with a direct LIST per lookup.
  Backing it with a shared informer/lister removes that load and can follow without changing the spec.

### Upgrade / Downgrade Strategy

* **Upgrading** requires applying the new CRDs (part of the install manifests) but changes no behavior:
  `repository.backend` defaults to `secret`.
* **There is no bulk migration.** Repositories become CRDs one at a time, when something updates them, so `hybrid` is
  the only safe way in. Going straight from `secret` to `crd` drops every unmigrated repository to a URL-only lookup
  (see Repository resolution): public ones keep working, private ones lose their credentials unless a credential
  template covers them.
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
* **A ConfigMap schema instead of CRDs.** The non-secret configuration could live in `argocd-cm` (or a dedicated
  ConfigMap) with credential material pulled in via the existing `$<secret>:<key>` reference syntax already used for
  `oidc.config`. In its favour: no CRD to install, no new RBAC, and the settings hot-reload comes for free. Against
  it, and why CRDs are proposed: no OpenAPI/CEL validation (a typo is found at refresh time); no per-repository RBAC,
  so anyone who can edit the ConfigMap can edit every repository; no per-object events or audit trail, only "the
  ConfigMap changed"; no `kubectl get repositories`; one document for the whole fleet, which makes GitOps ownership
  of individual repositories awkward and turns every change into a write to a shared, contended object; and no room
  for a `status` later. The `$secret:key` indirection also solves less than `secretRef` does — it inlines one value
  at a time, so a credential spanning several keys (TLS cert + key, GitHub App ID + key) is several references rather
  than one Secret.
* **Convert Secrets to CRs at startup instead of a `hybrid` mode.** A one-time pass would create a CR for every
  repository Secret, after which only CRDs are read. This removes the cross-backend resolution rules (longest prefix
  across stores, project-scope ranking), but a startup pass only sees Secrets that exist at boot: those applied later
  from Git, by External Secrets Operator or via the Helm chart's `configs.repositories` stay invisible until the next
  restart. Closing that gap means either reading Secrets at runtime (`hybrid`), watching for them (a controller), or
  dropping Secret-based repositories (non-goal 2). It also moves the source of truth unasked: for a Secret managed in
  Git, later edits to its `url`, `project` or proxy settings are silently shadowed by the converted CR. `hybrid` keeps
  that dual-representation risk opt-in and per-repository, and keeps backend branching confined to `db.NewDB`.
* Alternatively, we could have a single `Repository` CRD instead of having the `Repository`/`RepositoryCredential`
  split. In that case we would need to define how a repository credential template is defined in the context of a
  `Repository` (perhaps with a field denoting it to be a credential template)
