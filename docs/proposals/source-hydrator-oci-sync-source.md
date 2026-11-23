---
title: OCI registries as a Source Hydrator sync source
authors:
  - "@christianh814"
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2026-11-16
last-updated: 2026-11-16
---

# OCI registries as a Source Hydrator sync source

Let the Source Hydrator publish hydrated manifests as an OCI artifact, and let the Application sync from that artifact.

> [!NOTE]
> This document is a proposal. It tracks [argoproj/argo-cd#29899](https://github.com/argoproj/argo-cd/issues/29899),
> which is labelled `proposal:required`. Nothing described here has been approved or merged.

## Open Questions

* **Annotation and artifact type names.** The manifests-digest annotation
  (`io.argoproj.argocd.hydrator.manifests-digest`) and the artifact type
  (`application/vnd.argoproj.argocd.hydrated-manifests.v1`) are new identifiers that external tooling may come to rely
  on (see [Artifact format](#artifact-format)). The names should be settled in review.
* **Commit servers with signing enabled.** When the commit server is configured to GPG-sign hydrated commits, should it
  push OCI artifacts unsigned (proposed, with a warning in the log) or refuse OCI destinations?
* **Re-pushing an already hydrated dry commit.** The hydrator skips a dry commit it has already hydrated without
  looking at where it hydrated to. For OCI destinations this proposal makes the skip conditional on the hydrator
  configuration being unchanged, so that pointing an Application at a new tag creates that tag right away. Git
  destinations keep today's behaviour. Should Git be aligned?
* **Ordering pushes to a tag.** A registry accepts whichever push to a tag arrives last, so an older hydration can
  replace a newer one when two controller shards hydrate the same tag at once (see
  [Concurrent pushes](#concurrent-pushes)). Candidate mechanisms are to have exactly one shard hydrate a hydration
  group, or to verify what the tag points to after the push and retry. Which one, and whether it is needed for a first
  iteration, should be settled in review.
* **Admission checks for ApplicationSet templates.** The CEL rule on `syncSource` (see [API changes](#api-changes)) is
  also part of the ApplicationSet CRD, where it sees the template before it is rendered. A template whose `repoURL` is
  entirely a template expression is therefore treated as Git and cannot use the root path; keeping the `oci://` prefix
  literal in the template avoids this. Relatedly, `targetBranch` is no longer required by the schema, so a Git sync
  source without it is rejected by spec validation instead of at admission. Adding CEL rules for both was tried, and
  the resulting ApplicationSet CRD did not pass the API server's CRD validation; the cause was not established, but the
  rules are repeated under every generator of that CRD. One way out is to not emit these rules into the ApplicationSet
  CRD and rely on the generated Applications being validated.

## Summary

The [Source Hydrator](../user-guide/source-hydrator.md) renders manifests from a "dry" Git source and pushes the result
to a Git branch, which the Application then syncs from. This proposal adds OCI registries as a second output backend.

When `sourceHydrator.syncSource.repoURL` uses the `oci://` scheme, the hydrator packages the rendered manifests as an
OCI artifact, pushes it to the tag named in the new `syncSource.targetRevision` field (creating the tag if it does not
exist), and records the resulting manifest digest in the Application status. The Application syncs from that artifact
using the [OCI source support](../user-guide/oci.md) Argo CD already has.

Git-backed hydration is unchanged.

## Motivation

Pushing hydrated manifests to a Git branch gives history and fits pull-request-based promotion. But Git is not always
the storage or distribution mechanism a team wants for generated content. Many organizations already distribute
deployable artifacts through an OCI registry, and Argo CD can already consume OCI artifacts as a regular Application
source.

Today, getting rendered manifests into a registry requires an external CI system to do the rendering and the push. That
is the same maintenance cost the Source Hydrator was introduced to remove for Git (see the
[Manifest Hydrator proposal](manifest-hydrator.md)). Extending the hydrator to publish to OCI lets users adopt an
artifact-oriented workflow with the rendering still done by Argo CD.

### Goals

1. An Application whose `syncSource.repoURL` is `oci://...` is hydrated and synced by Argo CD alone, with no external
   tooling.
2. Git-backed hydration behaves exactly as it does today.
3. The published artifact is readable by Argo CD's existing OCI source support with the default repo-server
   configuration.
4. No new component and no new credential mechanism are introduced. The commit server remains the only component that
   writes.
5. A dry commit that does not change the rendered manifests does not produce a new artifact.

### Non-Goals

* Using an OCI artifact as the `drySource`. The dry source remains Git.
* Supporting `hydrateTo` (a separate "staging" destination) with an OCI sync source.
* Generating tags automatically, for example by semver pattern or from the dry commit SHA.
* Signing the published artifact.
* Letting several hydration groups share one tag.
* Implementing promotion between environments, or retention of old digests in the registry.
* Replacing or deprecating Git-backed hydration.

## Proposal

A sync source is treated as OCI when the effective sync repository URL starts with `oci://`. Because the dry source
must be Git, an OCI sync source always sets `syncSource.repoURL` explicitly.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
spec:
  project: default
  destination:
    server: https://kubernetes.default.svc
    namespace: guestbook
  sourceHydrator:
    drySource:
      repoURL: https://github.com/argoproj/argocd-example-apps.git
      targetRevision: HEAD
      path: guestbook
    syncSource:
      repoURL: oci://myreg.io/myorg/argo-cd-example
      targetRevision: dev
      path: .
```

For this Application, Argo CD will:

1. Resolve and render the dry source as it does today.
2. Package the hydrated manifests as an OCI artifact.
3. Push the artifact to `myreg.io/myorg/argo-cd-example` and point the `dev` tag at it. If `dev` does not exist, it is
   created.
4. Record the manifest digest in `status.sourceHydrator`.
5. Sync the Application from `oci://myreg.io/myorg/argo-cd-example` at `dev`.

### Tag semantics

The tag is long-lived and mutable. It plays the role a branch name plays for Git, and the manifest digest plays the role
of a commit SHA. Users who need immutability reference the digest.

This is the baseline the discussion on the issue converged on. `@crenshaw-dev` drew the branch-name and commit-SHA
comparison and suggested that pushing to a long-lived tag is a good foundation to build on. `@christianh814` then
summarised the first iteration as two points: push to a long-lived tag, and create the tag when it does not exist.
Automatic tagging schemes are deferred (see [Alternatives](#alternatives)).

### API changes

`SyncSource` gains one field, `targetRevision`. The following is a sketch of the type, not final code:

```go
type SyncSource struct {
	// TargetBranch is the Git branch from which hydrated manifests are synced. Git sync sources only.
	TargetBranch string `json:"targetBranch,omitempty" protobuf:"bytes,1,opt,name=targetBranch"`
	// Path is the directory where hydrated manifests are written and synced from.
	// It may be the root (".") only for OCI sync sources.
	Path string `json:"path" protobuf:"bytes,2,name=path"`
	// RepoURL is the Git repository or OCI repository (oci://) that holds the hydrated manifests.
	// If not set, defaults to the DrySource.RepoURL.
	RepoURL string `json:"repoURL,omitempty" protobuf:"bytes,3,opt,name=repoURL"`
	// TargetRevision is the OCI tag to which hydrated manifests are pushed and from which they are synced.
	// OCI sync sources only.
	TargetRevision string `json:"targetRevision,omitempty" protobuf:"bytes,4,opt,name=targetRevision"`
}
```

Exactly one of `targetBranch` and `targetRevision` applies, depending on the repository type:

| Field            | Git sync source      | OCI sync source                                              |
|------------------|----------------------|--------------------------------------------------------------|
| `targetBranch`   | Required (as today)  | Must be empty                                                |
| `targetRevision` | Must be empty        | Required. A plain tag: not a digest, not a semver constraint |
| `path`           | Must not be the root | May be the root (`.`)                                        |
| `hydrateTo`      | Optional (as today)  | Must be unset                                                |

`targetBranch` stops being a required property in the CRD schema so that OCI sync sources can omit it; the requirement
for Git moves to spec validation, where it is already checked. These field rules are reported as an `InvalidSpecError`
condition on the Application. The hydrator applies the same rules before it pushes anything, because it does not
depend on that condition.

The root-path rule is enforced by the CRD. `syncSource.path` currently carries a pattern that rejects `.`; it is
replaced by a CEL validation rule on `syncSource` that keeps the restriction for Git and lifts it when `repoURL` starts
with `oci://`:

```text
(has(self.repoURL) && self.repoURL.startsWith('oci://')) || size(self.path) >= 2 || (self.path != '.' && self.path != '/')
```

This is the first CEL validation rule in the Argo CD CRDs, and `sourceHydrator` is embedded many times in the
ApplicationSet CRD, so the rule deliberately avoids regular expressions to stay well within the CEL cost budget.

`status.sourceHydrator.currentOperation.hydratedSHA` and `status.sourceHydrator.lastSuccessfulOperation.hydratedSHA`
keep their names. For an OCI sync source they hold the manifest digest instead of a commit SHA.

### Tag ownership

A Git branch can be shared: each hydration adds a commit that touches only its own paths. An OCI tag cannot be shared in
the same way, because pushing a tag replaces the whole artifact.

> [!WARNING]
> Each hydration rebuilds the artifact from scratch and replaces what the tag pointed to. Content that the hydrator did
> not write in that run is not carried over.

The rules are therefore:

* **One hydration group owns a tag.** A hydration group is the set of Applications that share the same dry repository
  and dry target revision and write to the same destination. All Applications in the group are rendered together and
  published as one artifact, each under its own `syncSource.path`.
* **Different dry sources cannot share a tag.** If Applications with a different dry repository or dry target revision
  target the same OCI repository and tag, hydration is rejected with an error on the Applications involved.
* **The root path is exclusive.** An Application that sets `syncSource.path: .` must be the only Application in its
  group.

These cross-application rules are checked by the controller when it hydrates the group, alongside the existing check
that two Applications do not write to the same path. Violations are reported on
`status.sourceHydrator.currentOperation`.

A side effect of replacing the artifact is that it only ever contains the paths of the Applications currently in the
group. This differs from Git, where the hydrator leaves directories of removed or moved Applications in place.

### Unchanged manifests

When a new dry commit renders manifests identical to those already published, nothing is pushed and the tag keeps its
digest. This mirrors the Git backend, which skips the commit in the same situation. Application status still records
the new dry SHA, together with the unchanged digest.

### Credentials

No new mechanism is introduced. The two existing secret types are used, with `type: oci`:

* **Push:** a `repository-write` secret for the `oci://` URL. As for Git, the controller resolves it and sends it to the
  commit server with the request.
* **Sync:** a normal `repository` secret for the `oci://` URL.

Both the dry repository and the `oci://` repository must be permitted by the AppProject, as is already required when
the sync source is a separate Git repository.

### CLI and UI

* `argocd app create` and `argocd app set` gain `--sync-source-repo` and `--sync-source-revision`. There is currently no
  flag for the sync repository at all; the existing `--sync-source-branch` continues to set `targetBranch`.
* In the UI, the create and summary panels edit the tag instead of the branch when the sync repository is `oci://`, and
  the hydration status shows the digest.

### Use cases

#### Use case 1: Publish hydrated manifests to a registry without external CI

As a platform engineer, I would like Argo CD to render my Helm or Kustomize sources and publish the result to the OCI
registry my organization already operates, so that I do not have to maintain a CI pipeline that does the rendering and
the push.

#### Use case 2: Reference hydrated manifests by digest

As an operator, I would like every hydration to produce a content-addressed artifact whose digest is recorded on the
Application, so that I can reference exactly the manifests that were rendered for a given dry commit.

#### Use case 3: Keep generated content out of Git

As a developer, I would like the rendered output stored separately from my source repository, so that generated content
does not add branches and commits to Git.

#### Use case 4: Promote without re-rendering

As a release engineer, I would like the hydrated output to be an ordinary OCI artifact, so that my own tooling can copy
or re-tag a known digest for another environment instead of rendering the manifests again. Argo CD does not perform
this promotion itself.

### Implementation Details/Notes/Constraints

#### Commit server

The commit server remains the only component that writes, and the RPC stays the same. `CommitHydratedManifests`
dispatches on the scheme of the repository URL: `oci://` goes to a new OCI handler, anything else follows the existing
Git path. The request and response messages are reused, with one optional field added to the request:

| Message field            | Git                                  | OCI                                    |
|--------------------------|--------------------------------------|----------------------------------------|
| `repo`                   | Git repository and write credentials | OCI repository and write credentials   |
| `targetBranch`           | Branch to commit to                  | Tag to push                            |
| `syncBranch`             | Branch Argo CD syncs from            | Unused                                 |
| `dryRepoURL` (new)       | Unused                               | Dry repository URL, for the metadata   |
| `hydratedSha` (response) | Commit SHA                           | Manifest digest                        |

The request has so far only carried the destination repository. `dryRepoURL` is needed because, for OCI, the
destination is never the dry repository, and the artifact metadata has to name the repository the manifests were
rendered from.

The OCI handler:

1. Validates the request and takes the existing per-key lock, keyed by repository and tag.
2. Writes the files for every path into a temporary directory, sharing the file-writing code with the Git backend.
3. Computes a digest over the rendered manifests of all paths, in sorted path order.
4. Resolves the tag. If the tag exists and its manifests-digest annotation matches, it returns the existing manifest
   digest without pushing.
5. Otherwise it packs the directory, pushes it, tags it, and returns the new manifest digest. A missing tag is created.

Pushing uses ORAS (`oras-go` v2), which `util/oci` already uses for reading. The connection setup (TLS, proxy,
credentials) is factored out of the read client so that reading and writing behave identically. The existing read
interface is not changed.

No deployment changes are expected: the commit server already mounts `argocd-tls-certs-cm`, and its network policy
restricts ingress only.

#### Artifact format

The artifact has a single layer with media type `application/vnd.oci.image.layer.v1.tar+gzip`. That media type is
already in the repo-server's default list of allowed layer media types, so no repo-server configuration is needed to
sync from it.

The layer has the same layout as a hydrated Git branch, without `.gitattributes`. For an Application with
`syncSource.path: guestbook`:

```text
.
├── hydrator.metadata
└── guestbook
    ├── manifest.yaml
    ├── hydrator.metadata
    └── README.md
```

With `syncSource.path: .`, the Application's files sit at the root of the archive. Its `hydrator.metadata` and the
top-level `hydrator.metadata` are then the same file, which holds the information of both.

The archive is built deterministically (sorted entries, fixed modification time and ownership), so identical content
yields an identical layer. The layer carries the annotations `oras push` sets on a directory, so `oras pull` unpacks
the artifact into the working directory.

The artifact type is `application/vnd.argoproj.argocd.hydrated-manifests.v1`. The manifest carries these annotations:

| Annotation                                     | Value                            |
|------------------------------------------------|----------------------------------|
| `org.opencontainers.image.revision`            | Dry commit SHA                   |
| `org.opencontainers.image.source`              | Dry repository URL               |
| `org.opencontainers.image.created`             | Date of the dry commit           |
| `org.opencontainers.image.authors`             | Author of the dry commit         |
| `io.argoproj.argocd.hydrator.manifests-digest` | Digest of the rendered manifests |

The `org.opencontainers.image.*` annotations are ones Argo CD already displays for OCI sources. `created` is taken from
the dry commit rather than the wall clock so that the same input produces the same artifact. The manifests-digest
annotation is what the commit server compares to decide whether a push can be skipped. It covers the paths and their
`manifest.yaml` files only, so a change to the set of Applications is a change, while a new dry commit on its own is
not.

#### Controller

* The hydration queue key keeps its shape. For an OCI destination, the destination "branch" is the tag.
* Group validation gains the rules in [Tag ownership](#tag-ownership) and permits the root path for OCI. An Application
  whose project does not permit the OCI repository cannot block the Applications that own a tag.
* The commit request carries the tag and no sync branch, and the returned digest is stored in `hydratedSHA`.
* A dry commit that was already hydrated is pushed again when the hydrator configuration has changed since, so a newly
  configured tag exists as soon as it is configured.
* Syncing reuses the existing OCI support: the sync source is an ordinary `oci://` `ApplicationSource` with the tag as
  its `targetRevision`. One change is needed in the API server, which resolved the revision to sync as if the sync
  source were the same kind of repository as the dry source.

#### Testing

* `util/oci`: push an artifact and read it back through the existing extraction path, which proves Argo CD can consume
  what it publishes; missing tag; annotations; identical input gives an identical digest.
* Commit server: creates a missing tag, replaces the artifact on change, skips the push when unchanged, root path,
  multiple paths, and Git behaviour unaffected.
* Controller: queue key for OCI, root-path and tag-ownership rules, request contents, digest recorded in status.
* API, validation, CLI and UI: accessors, the validation table above, the new flags, and the UI source helpers.
* End-to-end: hydrate to a registry, sync, change the dry source, and confirm that the digest moves.

### Detailed examples

#### Repository secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: hydrated-oci-push
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository-write
stringData:
  url: oci://myreg.io/myorg/argo-cd-example
  type: oci
  username: my-push-username
  password: my-push-password
---
apiVersion: v1
kind: Secret
metadata:
  name: hydrated-oci-pull
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  url: oci://myreg.io/myorg/argo-cd-example
  type: oci
  username: my-pull-username
  password: my-pull-password
```

See [Repositories](../operator-manual/declarative-setup.md#repositories) for the other fields a `type: oci` repository
secret accepts.

#### Creating the Application with the CLI

```bash
argocd app create guestbook \
  --project default \
  --dest-server https://kubernetes.default.svc \
  --dest-namespace guestbook \
  --dry-source-repo https://github.com/argoproj/argocd-example-apps.git \
  --dry-source-revision HEAD \
  --dry-source-path guestbook \
  --sync-source-repo oci://myreg.io/myorg/argo-cd-example \
  --sync-source-revision dev \
  --sync-source-path .
```

#### Status after a successful hydration

```yaml
status:
  sourceHydrator:
    currentOperation:
      phase: Hydrated
      drySHA: <dry commit SHA>
      hydratedSHA: sha256:<manifest digest>
    lastSuccessfulOperation:
      drySHA: <dry commit SHA>
      hydratedSHA: sha256:<manifest digest>
```

#### Several Applications in one artifact

Two Applications with the same dry repository and dry target revision may publish to the same tag, provided each uses
its own non-root path. Both paths end up in the one artifact:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook-dev
  namespace: argocd
spec:
  project: default
  destination:
    server: https://kubernetes.default.svc
    namespace: guestbook
  sourceHydrator:
    drySource:
      repoURL: https://github.com/argoproj/argocd-example-apps.git
      targetRevision: HEAD
      path: guestbook
    syncSource:
      repoURL: oci://myreg.io/myorg/argo-cd-example
      targetRevision: dev
      path: guestbook
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: helm-guestbook-dev
  namespace: argocd
spec:
  project: default
  destination:
    server: https://kubernetes.default.svc
    namespace: helm-guestbook
  sourceHydrator:
    drySource:
      repoURL: https://github.com/argoproj/argocd-example-apps.git
      targetRevision: HEAD
      path: helm-guestbook
    syncSource:
      repoURL: oci://myreg.io/myorg/argo-cd-example
      targetRevision: dev
      path: helm-guestbook
```

A third Application that targets `oci://myreg.io/myorg/argo-cd-example` at `dev` from a different dry repository or dry
target revision is rejected. It needs its own tag or its own OCI repository.

### Security Considerations

* **Write access stays in one place.** Only the commit server pushes, and it receives the write credentials per request
  from the controller, exactly as for Git. Push and pull credentials remain separate secrets.
* **New outbound traffic.** The commit server will open connections to OCI registries. It uses the same TLS, proxy and
  credential handling as the existing OCI read path.
* **Project restrictions still apply.** The `oci://` repository must be permitted by the AppProject, for both pushing
  and syncing.
* **Artifacts are unsigned.** See [Risks and Mitigations](#risks-and-mitigations).
* **Rendered secrets.** The existing prerequisite holds: do not use the hydrator with tools that inject secrets during
  rendering. Such secrets would be published to the registry.
* **Protect the tag.** As with branch protection for Git, Argo CD should be the only writer of a hydrated tag. Restrict
  push access to the repository accordingly.

### Risks and Mitigations

#### Commit signing does not cover OCI

The commit server can [GPG-sign hydrated commits](../user-guide/source-hydrator.md#signing-hydrated-commits), with no
unsigned fallback. That guarantee is specific to Git.

> [!WARNING]
> OCI artifacts are pushed unsigned, including when commit signing is enabled on the commit server.

Mitigation: the limitation is documented, and the commit server logs a warning. Signing artifacts (for example with
cosign or notation) is future work. Whether a signing-enabled commit server should refuse OCI destinations is an
[open question](#open-questions).

#### No equivalent of the Git-note attestation

For Git, the hydrator records each processed dry SHA in a git note, including when the manifests did not change and no
commit was made. There is no OCI equivalent in this proposal. When a push is skipped, the artifact keeps the older dry
SHA in its annotations and metadata files, while the Application status records the newer one.

Mitigation: this is documented. Tooling that needs to know which dry commit was last processed should read the
Application status rather than the artifact.

#### Concurrent pushes

Git rejects a push which is not a fast-forward, so of two hydrations racing for one branch, the one based on the older
state fails and is retried. A registry has no such check: whichever push to a tag arrives last wins, and nothing in
this proposal orders pushes by dry commit.

Pushes to one tag are serialized in two places, and neither covers every case:

* The commit server locks per repository and tag, but only within one replica.
* The controller never hydrates one hydration group twice at the same time, but only within one controller shard. The
  Applications of a group may be spread over shards, and every shard that owns one of them hydrates the whole group.

The exposure is therefore two controller shards (or two Argo CD instances) hydrating the same tag at the same time
after resolving the dry revision to different commits. If the push for the older commit arrives last, the tag holds the
older manifests while Application status may already report the newer dry commit. Nothing notices the difference
until the next hydration of the tag, for example the next dry commit or a hard refresh.

Concurrent pushes of the same dry commit are harmless: archives are deterministic, so they produce the same artifact.
The [ownership rule](#tag-ownership) keeps different dry sources away from one tag within one Argo CD instance.

Mitigation: this is documented as a limitation, and a tag should be hydrated by a single Argo CD instance. Closing the
window needs a mechanism that holds across controller shards and commit server replicas, which is an
[open question](#open-questions).

#### Tag mutability

A mutable tag keeps no history. There is nothing like `git log` for the previous states of a tag, and rolling back
means pinning a digest rather than reverting a commit. `syncSource.targetRevision` only accepts a tag, so pinning is
done outside the hydrator, for example with a regular OCI `source` that references the digest.

Mitigation: the digest of each hydration is recorded in the Application status. Two registry behaviours are outside
Argo CD's control and worth knowing: a repository that forbids overwriting tags cannot be used as a hydration
destination, and whether a digest remains available after the tag moves on depends on the registry's retention policy.

#### Version skew

A controller with this change talking to a commit server without it sends an `oci://` URL to the Git code path. The
hydration fails with a Git error and nothing is written.

Mitigation: upgrade the commit server together with the controller, as for any release.

### Upgrade / Downgrade Strategy

**Upgrade.** The change is additive. Existing Applications need no changes and keep their behaviour:
`syncSource.targetBranch` is still required for Git sync sources, and the root path is still rejected for them. To use
the feature, a cluster needs the updated Application and ApplicationSet CRDs, the updated controller, API server and
commit server, and the two repository secrets described above. The repo-server needs no configuration.

**Downgrade.** Earlier versions do not know `syncSource.targetRevision` and require `targetBranch`. Before
downgrading, Applications that use an OCI sync source must be switched back to Git-backed hydration or to a regular
`source`. Artifacts that were already published remain in the registry and can still be consumed as ordinary OCI
sources.

## Drawbacks

* **It gives up what Git provides.** A reviewable history of hydrated manifests is one of the main reasons the Source
  Hydrator exists. A mutable tag has no history, no diff and no pull request. Users who pick OCI trade those for
  registry-based distribution.
* **Weaker guarantees than the Git backend.** No signing, no per-dry-commit attestation, and no rejection of
  conflicting pushes.
* **A second backend to maintain.** The commit server gains a code path that has nothing to do with commits, and every
  future hydrator feature has to consider both backends.
* **One tag per hydration group.** Setups that today hydrate several dry revisions into one shared branch need several
  tags or repositories with OCI.
* **Two fields for one concept.** `targetBranch` and `targetRevision` both mean "where to sync from", and which one is
  valid depends on the URL scheme.

## Alternatives

### Render and push from external CI

This is what users do today, and it needs no change to Argo CD. It leaves each organization to build and maintain its
own rendering pipeline, which is the cost the Source Hydrator is meant to remove.

### Semver tags with auto-increment (deferred)

`syncSource.targetRevision` could hold a semver constraint such as `1.*`: the hydrator would find the latest matching
tag and push to the next version. `@blakepettersson` described this variant on the issue. `@crenshaw-dev` pointed out
a race between the Git commit and the tag update. With a fixed semver tag, updating Git first overwrites the old tag,
while updating the target revision first publishes the new tag from the old Git revision. Auto-increment avoids this
only while the wildcard field is the one that changes; any other version bump has the same race. Making it atomic would
require pinning both the dry and the sync target revision and updating them together.

### Tag with the dry commit SHA (deferred)

`@blakepettersson` suggested allowing `targetRevision` to be unset for OCI, in which case the artifact would be tagged
with the Git SHA of the dry commit. This would give each dry commit its own tag. It is deferred rather than rejected:
as `@crenshaw-dev` noted on the issue, the long-lived-tag baseline is a foundation that auto-tagging by semver pattern
or dry SHA can be added to later, either inside Argo CD or by external tooling.

### Overlay onto the existing artifact (rejected for now)

To let several hydration groups share one tag, as they can share a Git branch, the commit server could pull the
current artifact, replace only the paths it owns, and push the result. This is a pull-modify-push cycle with no
atomicity: the registry cannot reject a push that was based on a stale artifact, so concurrent groups could silently
drop each other's content. The one-group-per-tag rule avoids the problem.

### Staging tags via `hydrateTo` (deferred)

`hydrateTo` could name a second tag that the hydrator pushes to, leaving promotion to the sync tag to an external
process, as it does for Git branches. It is left out of this iteration and rejected by validation. External tooling can
already copy or re-tag a published digest.
