---
title: Multiple DrySources for SourceHydrator
authors:
  - "@frankkerschbaumer"
sponsors:
  - TBD
reviewers:
  - "@crenshaw-dev"
  - TBD
approvers:
  - "@crenshaw-dev"
  - TBD

creation-date: 2026-06-04
last-updated: 2026-06-11
---

# Multiple DrySources for SourceHydrator

Support more than one dry source for an Application using the Source Hydrator.

Related Issues:

* [Multiple DrySources for an Application (#26804)](https://github.com/argoproj/argo-cd/issues/26804)
* [Multiple Sources for Application (#677)](https://github.com/argoproj/argo-cd/issues/677) — the original `sources` proposal that this mirrors
* [Future-Proofing the Source Hydrator API](https://hackmd.io/@crenshaw-dev/SygQ3BNx1x) — @crenshaw-dev's API shape comparison, recommending Proposal 1 (`sourceHydrator.drySources`), which this proposal implements

## Open Questions

* How should the commit message template handle multiple dry source repos? This proposal uses the first dry source's commit metadata, but a more descriptive format may be preferred.
* The current `HydrationQueueKey` uses plain struct equality (`!=`) with string fields. With multiple dry sources, the key needs to represent a variable-length source list in a comparable form. Options include a serialized string, a hash, or switching to `reflect.DeepEqual` for key comparison. The right approach depends on performance tradeoffs with large app lists.

## Summary

This proposal has two parts:

1. **Bring `DrySource` to parity with `ApplicationSource`** — add the missing `Chart`, `Ref`, and `Name` fields to the `DrySource` struct, enabling OCI/Helm chart hydration and cross-source value file references. This benefits both singular `drySource` and plural `drySources` users since both use the same struct.

2. **Add `drySources` (plural) to `SourceHydrator`** — following the same `source`/`sources` pattern already established on `ApplicationSpec`. When `drySources` is specified, Argo CD ignores `drySource` (singular), fetches and renders manifests from all dry sources, merges them into a single manifest set, and commits the combined result to the sync target.

## Motivation

Organizations frequently compose applications from multiple repositories — a Helm chart in one repo, shared configuration values in another, and platform-level base manifests in a third. Today, the only way to combine these with the hydrator is to pre-merge them externally (e.g. in CI) before pointing `drySource` at the result. This forces prospective users away from using the source hydrator back to the CI-based hydration pattern that the hydrator was designed to replace.

The `sources` (plural) field on `ApplicationSpec` already solves this problem for direct sync. This proposal brings the same capability to the hydrator pipeline, allowing users to declare multiple dry sources and have Argo CD handle the rendering and merging.

### Goals

1. **Bring `DrySource` to full field parity with `ApplicationSource`** — add `Chart`, `Ref`, and `Name` to the `DrySource` struct. This explicitly includes OCI/Helm chart repo support (via `Chart`) and cross-source value file references (via `Ref`). These fields benefit both `drySource` (singular) and `drySources` (plural) users since both share the same struct.

2. **Support multiple dry sources per Application** — users can specify a `drySources` array on `SourceHydrator`. Argo CD compiles manifests from all dry sources and commits the combined output.

3. **Follow the established `source`/`sources` pattern** — the API design, precedence logic, helper methods, and validation mirror the existing `ApplicationSpec.Source`/`Sources` pattern to minimize surprise for users and maintainers.

4. **Backward compatibility for `drySource` (singular)** — existing applications using `drySource` (singular) continue to work. The `GetDrySources()` helper wraps it transparently.

### Non-Goals

* **Partial hydration** — when any dry source changes, all sources are re-rendered and committed together. Incremental/partial hydration of individual sources is not a goal.
* **Per-source sync paths** — all dry sources are merged into a single manifest set committed to one `syncSource` path. Routing individual dry sources to different sync paths or branches is not supported.
* **Backward compatibility for hydrator status fields and metadata schema** — the hydrator is beta. `DrySHA` is replaced by `DryRevision`/`DryRevisions` (matching the `Revision`/`Revisions` convention on `SyncOperation`). The `hydrator.metadata` file schema changes from a flat object to a list. These are breaking changes.
* **References contract for non-git sources** — today, `references` in the hydrator metadata is populated from git trailers on the dry commit. Extending this to OCI metadata or Helm chart metadata is deferred to a follow-up proposal.
* **UI changes** — all UI updates (models, utility functions, display components, create/edit panels) are out of scope and will be covered in a separate proposal.

> **Note**: Helm/OCI chart support and cross-source `$ref` value file references are explicitly **in scope** as part of the `DrySource` parity goal (Goal 1). The intent is for `drySources` to be functionally identical to `sources` — anything you can do with an `ApplicationSource` entry in `sources`, you can do with a `DrySource` entry in `drySources`.

## Proposal

### Use Cases

#### Use case 1: Application composed from multiple repos

As a user, I want to compose an application from manifests spread across multiple repositories — app manifests, shared configmaps, and platform base layers — and have the hydrator merge them.

```yaml
spec:
  sourceHydrator:
    drySources:
      - repoURL: https://github.com/mycompany/billing-app.git
        path: manifests
        targetRevision: v2.1.0
      - repoURL: https://github.com/mycompany/common-settings.git
        path: configmaps-billing
        targetRevision: HEAD
      - repoURL: https://github.com/mycompany/platform-base.git
        path: overlays/prod
        targetRevision: main
        kustomize:
          namePrefix: billing-
    syncSource:
      repoURL: https://github.com/mycompany/billing-app.git
      targetBranch: environments/prod
      path: billing-hydrated
```

#### Use case 2: Helm chart with values from another repo using `$ref`

As a user, I want to deploy a Helm chart from one repository using values files stored in a separate configuration repository, referencing them via `$ref`, and have the hydrator render and commit the combined result.

```yaml
spec:
  sourceHydrator:
    drySources:
      - repoURL: https://github.com/mycompany/billing-app.git
        path: charts/billing
        targetRevision: v2.1.0
        helm:
          releaseName: billing
          valueFiles:
            - values-prod.yaml
            - $common/values-shared.yaml
      - repoURL: https://github.com/mycompany/common-settings.git
        ref: common
        path: helm-values
        targetRevision: HEAD
    syncSource:
      repoURL: https://github.com/mycompany/billing-app.git
      targetBranch: environments/prod-next
      path: billing-hydrated
```

#### Use case 3: OCI Helm chart with external values (push-to-stage)

As a user, I want to hydrate an OCI-hosted Helm chart using values files from a separate git repo, push to a staging branch for review, and sync from the production branch after approval.

```yaml
spec:
  sourceHydrator:
    drySources:
      - repoURL: https://github.com/mycompany/infra.git
        targetRevision: main
        ref: app-values
      - repoURL: us-docker.pkg.dev/mycompany/helm-charts
        targetRevision: 0.2.2
        chart: my-app
        helm:
          valueFiles:
            - $app-values/environments/prod/values.yaml
    syncSource:
      repoURL: https://github.com/mycompany/infra.git
      targetBranch: environments/prod
      path: my-app-hydrated
    hydrateTo:
      targetBranch: environments/prod-next
```

### Implementation Details

#### API Changes

**`SourceHydrator` struct** — add a `DrySources []DrySource` field (protobuf field 4). `DrySource` changes from a value type to a pointer (`*DrySource`) to support `omitempty` correctly, matching the `Source *ApplicationSource` pattern on `ApplicationSpec`. When `drySources` is set, `drySource` is ignored for hydration purposes (same precedence as `sources` over `source`). Users set one or the other, never both — mutual exclusion is enforced by validation.

**`DrySource` struct** — add three new fields to reach parity with `ApplicationSource`:

* `Chart` — a Helm chart name, enabling Helm repo and OCI registry sources
* `Ref` — a reference key enabling cross-source Helm value file references (e.g. `$ref-name/path/to/values.yaml`)
* `Name` — display name for the UI

**Status types** — `SyncOperation` tracks revisions with both `Revision string` (singular, legacy) and `Revisions []string` (plural, for multi-source). The singular field exists only because it predates multi-source support. Since the hydrator is beta, we skip the legacy singular field and go straight to a slice:

* `HydrateOperation` and `SuccessfulHydrateOperation`: `DrySHA string` is replaced by `DryRevision string` (singular, for `drySource`) and `DryRevisions []string` (plural, for `drySources`). This matches the `Revision`/`Revisions` pattern on `SyncOperation`. Each entry is the resolved revision for the corresponding dry source — a git SHA, Helm chart version, or OCI digest depending on source type. When `drySources` is used, `DryRevision` holds the first source's revision. `HydratedSHA` remains unchanged (it's a single git SHA on the hydrated branch).
* `SourceHydratorStatus`: `LastComparedDryRevision string` remains (for `drySource`), `LastComparedDryRevisions []string` is added (for `drySources`).

Per-source detail beyond the revision string (author, commands, etc.) is handled by the list-based `hydrator.metadata` file (see Commit Server Changes below).

**`SyncSource.RepoURL`** already exists as an optional field. No changes needed.

#### Helper Methods

New methods on `SourceHydrator`, following the `ApplicationSpec.GetSources()`/`HasMultipleSources()` pattern:

* `HasMultipleDrySources()` — returns `len(s.DrySources) > 0`
* `GetDrySources()` — returns all dry sources as `ApplicationSources`. When plural is set, converts each via `ToApplicationSource()`. Otherwise wraps the singular `drySource` in a single-element slice.

New method on `DrySource`:

* `ToApplicationSource()` — converts a `DrySource` to an `ApplicationSource`, mapping all fields including the new `Chart`, `Ref`, and `Name`. Existing `GetDrySource()` refactored to call this.

`GetSyncSource()` and `GetHydrateToSource()` updated: when `HasMultipleDrySources()` and `SyncSource.RepoURL` is empty, fall back to `DrySources[0].RepoURL`.

`DeepEquals()` updated to compare both `DrySource` (nil-safe pointer comparison) and `DrySources` (element-wise via `DrySource.Equals()`).

#### Validation

* **At least one source required**: either `drySource` must be non-nil with a non-empty `repoURL`, or `drySources` must have at least one entry. The existing `validateSourceHydrator()` check (`DrySource.RepoURL == ""` → error) must change to `DrySource == nil && len(DrySources) == 0` → error.
* **Precedence**: if both `drySource` and `drySources` are set, `drySources` silently takes precedence. This matches how `source`/`sources` works on `ApplicationSpec` — no mutual exclusion error.
* **Per-entry validation**: each entry in `drySources` must have a non-empty `repoURL`.
* **Ref uniqueness**: `ref` keys must be unique across all dry sources and match `^[a-zA-Z0-9_-]+$` (same validation as `ApplicationSource.Ref`).
* **Project permissions**: each dry source's RepoURL must be permitted by the Application's project. `ValidatePermissions()` must iterate over `GetDrySources()` rather than calling `GetDrySource()` once.

#### Hydration Flow with Multiple Dry Sources

When any dry source has a new revision, the full set is re-hydrated:

1. `appNeedsHydration()` calls `newRevisionHasChanges()` which evaluates each dry source via `EvaluateAppRevisionsChanges()`. If any source has changes, hydration is triggered.

2. `getManifests()` fetches and renders manifests from all dry sources. Each dry source is rendered independently via a separate `GenerateManifest()` call (one per source). Dry sources with a `ref` field are resolved into a `RefTargetRevisionMapping` and passed to the rendering call, enabling Helm value file references like `$ref-name/path/to/values.yaml`. All manifest objects are appended into a single `PathDetails` in source order (same append semantics as `sources` in the sync path). If two dry sources produce resources with the same GVK+namespace+name, both are included — conflict detection is handled at sync time, not hydration time.

3. `hydrate()` resolves static revisions for all dry sources from the first app, then renders remaining apps pinned to those same revisions. For singular `drySource`, the resolved revision is stored in `DryRevision`. For plural `drySources`, revisions are stored in `DryRevisions` (one per source, by array index). De-duplication compares against `LastSuccessfulOperation` — single string comparison for `DryRevision`, element-by-element for `DryRevisions`. Each source's full metadata (author, commands, etc.) is written to the `hydrator.metadata` file (see Commit Server Changes).

4. All paths are committed in a single commit to the sync target branch.

#### Hydration Queue Key

The current queue key groups apps by `{SourceRepoURL, SourceTargetRevision, DestinationRepoURL, DestinationBranch}`. With multiple dry sources, the key is derived from the full `drySources` list plus the destination. Apps with the same set of dry sources (same URLs and target revisions) going to the same destination are batched together — same dedup behavior as today, extended to multiple sources.

#### Changes to Controller Dependencies

The `Dependencies` interface methods `GetRepoObjs` and `EvaluateAppRevisionsChanges` remain single-source — they continue to accept one `ApplicationSource` and one `revision string`. The hydrator owns the multi-source orchestration: `getManifests()` iterates over `GetDrySources()`, calling `GetRepoObjs` once per dry source, then merges the results. Similarly, `newRevisionHasChanges()` calls `EvaluateAppRevisionsChanges` per source and returns true if any source has changes. Error collection and manifest merging happen in the hydrator, not in the dependencies layer.

#### Commit Server Changes

The current `hydrator.metadata` file is a flat object with fields (`repoURL`, `drySha`, `author`, `subject`, `date`, `commands`) that all relate to a single git source. This schema doesn't extend cleanly to multiple sources because:

* Not every source type has a "SHA" — git sources resolve to commit SHAs, Helm chart repos have versions, OCI registries have digests.
* The commit metadata fields (`author`, `subject`, `date`) are specific to a single git commit and have no meaning for non-git sources.

To address this, the `hydrator.metadata` schema is replaced with a **list of per-source objects**. Each object carries only the fields relevant to its source type:

```json
{
  "sources": [
    {
      "repoURL": "us-docker.pkg.dev/mycompany/helm-charts",
      "chart": "my-app",
      "revision": "0.2.2",
      "commands": ["helm template ..."]
    },
    {
      "repoURL": "https://github.com/mycompany/infra.git",
      "revision": "bb2222",
      "author": "Jane Doe",
      "date": "2026-06-11T10:00:00Z",
      "subject": "Update prod values",
      "commands": []
    }
  ]
}
```

The git commit note (`refs/notes/hydrator.metadata`) uses the same list schema. For single-source apps, the list has one entry — functionally identical to the current flat object, just wrapped in `{"sources": [...]}`.

`CommitHydratedManifestsRequest` in `commit.proto` is updated to carry per-source metadata as a repeated message field (field number 11+, since the highest existing field is 10). The `HydratorCommitMetadata` struct in `util/hydrator/hydrator.go` is refactored to represent a list of source metadata entries, with `GetCommitMetadata()` accepting a slice of sources and returning a slice of per-source metadata. The `WriteForPaths()` function in `hydratorhelper.go` is updated to accept the sources list instead of scalar `repoUrl` and `drySha` parameters.

The README template (configurable via `argocd-cm`) currently references `.RepoURL`, `.DrySHA`, and `.Commands` directly from a flat metadata object. With the list-based schema, the template is updated to iterate over `.Sources` or use the first source's fields. The default template is updated accordingly.

#### GitOps Promoter Compatibility

The [GitOps Promoter](https://github.com/argoproj-labs/gitops-promoter) reads the `hydrator.metadata` file and git notes to track promotion lineage. It currently expects a flat object with a top-level `drySha` field.

With the list-based schema, the promoter's `HydratorMetadata` struct needs to be updated to read `sources[]` and extract per-source revisions. For lineage tracking, the promoter can compute a composite revision (hash of all per-source revisions) or track individual source revisions — depending on what granularity is needed for promotion rules.

This is a breaking change for the promoter's metadata parsing, but the promoter is maintained by the same team (@crenshaw-dev, @zachaller) and can be updated in coordination with them.

### Security Considerations

* Each dry source repository must be permitted by the Application's `AppProject`. The validation logic is extended to check all dry sources, not just the first.
* Write credentials for the sync target are resolved the same way as today — only the destination repo requires write access.
* No new authentication mechanisms are introduced. Each dry source repo uses existing Argo CD repository credentials.

### Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Increased hydration latency with many dry sources | Manifest generation for different dry sources can be parallelized (the existing `errgroup` pattern in `hydrate()` already does this for multiple apps). |
| Queue key change causes brief re-hydration during rolling upgrade | The queue key is internal and self-healing — the next `ProcessAppHydrateQueueItem` call re-enqueues with the new key format. Hydration is idempotent. |
| `hydrator.metadata` schema change breaks existing consumers | The hydrator is still beta. The GitOps Promoter is maintained by the same team and can be updated in coordination. Single-source apps produce a one-element list, making migration straightforward. |
| Custom commit message templates break silently | Users who customized the hydrator commit message template in `argocd-cm` reference `.RepoURL`, `.DrySHA`, and `.Commands` as top-level fields. These change with the list-based schema. The upgrade docs should call out that custom templates need updating. |

### Upgrade / Downgrade Strategy

**Upgrade:**

`drySources` is a new feature — there are no existing users to migrate. For existing `drySource` (singular) apps:

* `drySource` continues to work unchanged. `GetDrySources()` wraps it into a single-element slice transparently.
* `DrySHA` is replaced by `DryRevision` (same value, just renamed to match the `Revision` convention). On first reconciliation after upgrade, existing apps will have an empty `DryRevision` and trigger a one-time re-hydration (idempotent, self-healing).
* The `hydrator.metadata` file changes from a flat object to `{"sources": [...]}` with a single entry. External consumers (GitOps Promoter) must be updated to read the new schema.

**Downgrade:**

The hydrator is beta. Downgrading is not expected to be seamless:

* `DryRevision`/`DryRevisions` replace `DrySHA`. Older versions will see an empty `DrySHA` and trigger re-hydration (self-healing).
* The `hydrator.metadata` schema is not backward compatible. The GitOps Promoter must be updated before or alongside the Argo CD upgrade.

## Drawbacks

* **Increased complexity in the hydrator pipeline** — every code path that touches `DrySource` must now handle the plural form. This is mitigated by the `GetDrySources()` helper which abstracts the singular/plural distinction, matching the proven `GetSources()` pattern.

* **All-or-nothing re-hydration** — when any single dry source changes, all sources are re-rendered. For applications with many large dry sources, this may be slower than necessary. However, this matches how `sources` works for sync and keeps the implementation simple. Partial hydration could be explored as a future optimization.

## Alternatives

1. **Use ApplicationSets to compose from multiple repos** — users can create separate Applications per repo and use an ApplicationSet to coordinate them. This works but loses the single-Application view and requires more complex orchestration for deployment ordering.

2. **Pre-merge in CI** — users can use CI pipelines to merge manifests from multiple repos into a single repo, then point `drySource` at the merged result. This is the status quo workaround and defeats the purpose of the hydrator.

3. **Extend `sources` to work with the hydrator** — instead of adding `drySources` to `SourceHydrator`, extend the top-level `sources` field to work with hydration. This was rejected because it conflates two different concepts (sync sources vs. dry sources) and would require significant changes to how `sources` interacts with the sync pipeline.
