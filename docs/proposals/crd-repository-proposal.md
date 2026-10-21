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
last-updated: 2026-10-21
---

# Repository CRD

Allow ArgoCD repositories to be stored as CRs, while maintaining full backwards compatibility with the existing
secret-based implementation.

## Open Questions

* **Should SCM-specific fields get their own block?** GitHub App fields currently sit under `git`
  (`git.githubAppID`, …). Grouping them per SCM instead (`github.appID`, `github.appInstallationID`, …) reads better
  and leaves room for other providers, at the cost of a second axis (type × SCM) in the spec.

## Summary

Argo CD stores repository configuration and credentials in Secrets. This proposal adds two namespaced CRDs,
`Repository` and `RepositoryCredential` (`argoproj.io/v1alpha1`), which hold all *non-secret* configuration
declaratively; credential material stays in a Secret referenced via `spec.secretRef`. Storage is selected fleet-wide
by the `repository.backend` key in `argocd-cm`: `secret` (default, existing behavior), `crd`, or `hybrid` (reads both
stores; updates migrate repositories to CRDs). The CRDs are spec-only; a `status` subresource is deferred (see
Non-Goals).

## Motivation

- More GitOps-friendly (and secure) declarative management, since we can put (most of) the config in Git
- Richer validations with OpenAPI validations (and possibly webhook-based ones)
- Enhanced security for agent-based architectures
- Enhanced Kubernetes integration (`kubectl get repositories` and friends)

### Goals

1. **Introduce CRDs**
    - `Repository` CRD for repository configurations
    - `RepositoryCredential` CRD for credential templates

2. **Backwards Compatibility**
    - Existing Secret-based repositories keep working; `secret` stays the default
    - No breaking changes to the CLI or the gRPC/REST API

3. **Gradual Migration**
    - `hybrid` reads both stores, so operators migrate at their own pace
    - Updating a repository in `hybrid` mode migrates it to a CR (see Secret ownership and lifecycle)
    - All components read the same `argocd-cm` key, so the fleet switches consistently

4. **Security Parity**
    - Credential material stays in Secrets, with the same encryption-at-rest guarantees
    - Read and write (push) credentials stay separate (see Security Considerations)

### Non-Goals

1. **Changing internal API types.** The internal repository struct keeps its shape and wire format; it is only
   renamed to free the `Repository` identifier (see API version and naming).
2. **Immediate Secret deprecation.** Secret-based storage stays supported and remains the default; deprecating it
   would be a separate proposal.
3. **API contract changes.** CLI behavior, the gRPC/REST API and client SDKs are all unaffected.
4. **Multi-tenancy redesign.** Project scoping is unchanged, and namespace isolation
   ("repositories-in-any-namespace") is not addressed, though the namespaced CRD shape leaves room for it.
5. **Workload identity.** Existing ad-hoc support is carried through as-is (e.g `spec.useAzureWorkloadIdentity`, as
   with secrets today); a holistic solution is future work.
6. **Repository status.** The CRDs ship spec-only. Connection state is reported as it is today, so `argocd repo list`
   and the UI behave the same in every mode. A `status` subresource is a design of its own and is deferred to a
   follow-up proposal; it can be added later without changing the spec.

## Proposal

### Use cases

#### Use case 1:
As a user, I would like to manage my repositories using GitOps.

### Implementation Details/Notes/Constraints

#### API version and naming

The CRDs ship as `argoproj.io/v1alpha1`, alongside `Application`, `AppProject` and `ApplicationSet`. The existing
`Repository`/`RepositoryList` Go types in `pkg/apis/application/v1alpha1` collide with the new kinds, so they are
renamed to `RepositoryConfig`/`RepositoryConfigList`, as #28840 did for `SyncWindow`. The rename is Go-source only:
JSON field names, the protobuf wire format and REST paths are unchanged.

#### Backend selection

* The storage backend is a single fleet-wide setting, `repository.backend` in `argocd-cm` (`secret` | `crd` |
  `hybrid`). An invalid or unset value resolves to `secret` with a warning.
* The key is read at startup, not hot-reloaded, so changing it requires restarting the affected components. Until
  every component has restarted, `secret` and `hybrid` components coexist and the `secret` ones cannot see
  repositories migrated in that window, so the rollout should finish before repositories are updated.

#### Storage model

* The spec carries all non-secret configuration, including fields that live in the Secret today: `project`, proxy
  settings, per-type options such as `git.enableLFS` or `helm.enableOCI`, and the `write` marker. The Helm repo alias
  is `helm.name`, since git and OCI repositories don't use it.
* The Secret carries only credential material (`username`, `password`, `sshPrivateKey`, `githubAppPrivateKey`, …). A
  `secretRef` is written only when credentials exist.
* `secretRef` resolves only to Secrets labeled `argocd.argoproj.io/secret-type` with the new value `repository-secret`
  (credential-only, no `url`; a new value because a url-less `repo-creds` template would match every URL) or the
  existing `repository`/`repo-creds` values, so migration can reference a legacy Secret without relabeling it (see
  Security Considerations). An unlabeled target reports `CredentialsMissing`.

#### Repository resolution

Today, the secrets backend looks repositories up by URL + project: URLs are compared normalized (`git.SameURL`), an
exact-`project` match beats a project-less one, credential templates match by longest URL prefix, and a miss returns
a URL-only repository, so templates still apply and public repositories need no object.

The CRD backend keeps these rules and reuses the same helpers; since lookup is by URL + project, CRs may use any
name. New with this proposal:

* **In `hybrid`, the rules apply across both stores.** Candidates from both are ranked together: an exact-project
  match beats a project-less one, and the longest template prefix wins, whichever store each comes from. CRD
  precedence only breaks ties; otherwise a broader CR could silently replace a Secret's credentials mid-migration.
* **Duplicates are logged.** Two `Repository` resources with the same URL and project resolve to the first-sorting
  name, so every component agrees, and a throttled warning names both. CEL cannot enforce cross-object uniqueness.

#### Secret ownership and lifecycle

Today, Secrets that Argo CD generates carry the `managed-by: argocd.argoproj.io` annotation and `argocd repo rm`
deletes only those; a user-provided Secret just loses its `secret-type` label and is kept (see `util/db/secrets.go`).
This proposal keeps the rule that Argo CD deletes a Secret only if it created it:

* **Generated Secrets**: a Secret labeled `secret-type: repository-secret` is created when a `Repository` CR is
  _imperatively_ created with credentials (=created with `argocd repo add`, UI, API). The Secret gets owner-referenced
  to the CR, and garbage-collected when the CR is removed.
* **User-provided Secrets**: never owner-referenced or deleted, and may be shared; their lifecycle belongs to whoever
  manages them (e.g. External Secrets Operator).
* **Migration** happens only on an imperative update in `hybrid` mode (`argocd repo add --upsert`, UI, API);
  editing the Secret directly with `kubectl` or from Git migrates nothing. The update first writes the CR from the
  Secret's non-credential keys, then replaces a generated legacy Secret with a generated Secret as above, or
  references a user-provided one as-is (see "Two writers of repository configuration" under Risks and Mitigations).
  Each step is idempotent, so a potential crash leaves the repository still resolvable.
* **Deleting a `Repository` does not check Applications**, like `argocd repo rm` today; any apps referencing deleted
  repositories fail at their next refresh.

#### Events

The API server, which performs every write in `crd`/`hybrid` mode, emits `ResourceCreated`/`ResourceUpdated`/
`ResourceDeleted` on the `Repository` or `RepositoryCredential` through the existing audit logger, as it does for
Applications and AppProjects; a migration emits `ResourceCreated`.

#### RBAC

* application, applicationset and notifications controllers: get/list/watch on `repositories`/`repositorycredentials`
* API server: full CRUD on both

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

* Credential material never enters the CRD, so the spec is safe to store in Git.
* **`spec.secretRef` is gated by a label.** Otherwise anyone who can create a `Repository` but not read Secrets could
  have Argo CD send any Secret in the namespace to a URL they control. A reference is honored only when the target
  carries an Argo CD `secret-type` label (`repository-secret`, `repository` or `repo-creds`); once labeled, its
  credentials are usable by anyone who can create a `Repository`, against any URL.
* **Credential-template prefix matching is unchanged.** A repository URL with no credentials of its own uses
  those of the longest-prefix template, and the prefix match has no path boundary: a template for
  `https://github.com/example` also covers `https://github.com/example-evil`. `RepositoryCredential` templates match
  the same way; this proposal neither widens nor fixes that.
* Read and write (push) credentials are separate objects (`spec.write`), filtered on in every lookup, so neither can
  be served in place of the other.

### Risks and Mitigations

* **Two writers of repository configuration during migration.** A migrated user-provided Secret keeps its
  configuration keys alongside the CR: `hybrid`/`crd` components read the CR, anything still in `secret` mode reads
  the Secret. Migration removes the second copy for generated Secrets.
* **A misconfigured `Repository` is silent until something uses it.** Without a status, a CR with bad credentials or
  an unreachable URL surfaces only when an Application using it fails to refresh, as with declarative repository
  Secrets today.

### Upgrade / Downgrade Strategy

* **Upgrading** requires applying the new CRDs but changes no behavior: `repository.backend` defaults to `secret`.
* **There is no bulk migration.** Repositories migrate one at a time when updated, so `hybrid` is the only safe way
  in. Going straight from `secret` to `crd` leaves unmigrated repositories with a URL-only lookup (see Repository
  resolution): public ones keep working, private ones lose their credentials unless a template covers them.
* **Migration progress is observable**: a gauge of repositories resolved per backend, and a store column in
  `argocd repo list`, show how many are left before moving to `crd` is safe.
* **Downgrading** from `hybrid` or `crd` requires `argocd admin repo export` first. It writes a legacy-shaped Secret
  per `Repository`/`RepositoryCredential` (spec plus the referenced credentials), to apply before switching back to
  `secret`.

## Drawbacks

* Credential selection is harder to reason about in `hybrid`, where a repository can exist as both a CR and a Secret
  and the resolution rules decide which is used.
* Most repositories now need both a CR and a Secret, which is more to manage than a single Secret today.

## Alternatives

* **Keep Secrets as they are.** They work today.
* **A ConfigMap instead of CRDs.** Non-secret configuration in a ConfigMap, credentials pulled in with the existing
  `$<secret>:<key>` syntax. It avoids a CRD and new RBAC, but gives up schema validation, per-repository RBAC and
  events, `kubectl get repositories` and a future `status`, and puts every repository in one shared object.
* **Convert Secrets to CRs at startup instead of `hybrid`.** This removes the cross-store rules, but only sees
  Secrets that exist at boot: ones applied later (from Git, External Secrets Operator, the Helm chart) stay invisible
  until a restart. Later edits to a Git-managed Secret are also silently shadowed by the converted CR.
* **Mirror Secrets into CRs with a controller instead of `hybrid`.** This sees Secrets at any time and drops the
  cross-store rules, but mirrored CRs need an ownership rule for edits (the migration rule in another form), new
  Secrets resolve only after a reconcile, it adds a leader-elected process, and the CRDs become mandatory for every
  install, so `secret` stops being a zero-change default.
* **A single `Repository` CRD** with a field marking credential templates, instead of the
  `Repository`/`RepositoryCredential` split.
