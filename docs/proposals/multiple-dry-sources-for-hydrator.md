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
* The manifest hydrator proposal is explicitly opinionated against nondeterministic configuration injection. Multiple dry sources from different repos mean the combination of source versions is held externally to any single git repo. Is this an acceptable trade-off given that the same pattern already exists with `sources` in the non-hydrator path?

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

4. **Maintain full backward compatibility** — existing applications using `drySource` (singular) continue to work with zero changes.

### Non-Goals

* **Partial hydration** — when any dry source changes, all sources are re-rendered and committed together. Incremental/partial hydration of individual sources is not a goal.
* **Per-source sync paths** — all dry sources are merged into a single manifest set committed to one `syncSource` path. Routing individual dry sources to different sync paths or branches is not supported.
* **UI changes** — all UI updates (models, utility functions, display components, create/edit panels) are out of scope and will be covered in a separate proposal.

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

**Status types** — `HydrateOperation` and `SuccessfulHydrateOperation` gain a `DrySHAs []string` field alongside the existing singular `DrySHA`. `SourceHydratorStatus` gains `LastComparedDryRevisions []string` alongside the existing `LastComparedDryRevision`. Singular fields continue to be populated (first entry or composite hash) for backward compatibility.

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
* **Mutual exclusion**: setting both `drySource` (non-nil) and `drySources` (non-empty) is a validation error.
* **Per-entry validation**: each entry in `drySources` must have a non-empty `repoURL`.
* **Ref uniqueness**: `ref` keys must be unique across all dry sources and match `^[a-zA-Z0-9_-]+$` (same validation as `ApplicationSource.Ref`).
* **Project permissions**: each dry source's RepoURL must be permitted by the Application's project. `ValidatePermissions()` must iterate over `GetDrySources()` rather than calling `GetDrySource()` once.

#### Hydration Flow with Multiple Dry Sources

When any dry source has a new revision, the full set is re-hydrated:

1. `appNeedsHydration()` calls `newRevisionHasChanges()` which evaluates each dry source via `EvaluateAppRevisionsChanges()`. If any source has changes, hydration is triggered.

2. `getManifests()` fetches and renders manifests from all dry sources. Each dry source is rendered independently via a separate `GenerateManifest()` call (one per source). Dry sources with a `ref` field are resolved into a `RefTargetRevisionMapping` and passed to the rendering call, enabling Helm value file references like `$ref-name/path/to/values.yaml`. All manifest objects are appended into a single `PathDetails` in source order (same append semantics as `sources` in the sync path). If two dry sources produce resources with the same GVK+namespace+name, both are included — conflict detection is handled at sync time, not hydration time.

3. `hydrate()` resolves static SHAs for all dry sources from the first app, then renders remaining apps pinned to those same SHAs. Individual SHAs are stored in `DrySHAs` as an ordered vector (one per `drySources` entry, by array index). De-duplication compares this vector element-by-element against `LastSuccessfulOperation.DrySHAs`. A composite SHA (deterministic SHA-256 hash of all individual SHAs in order) is computed for the commit note and `hydrator.metadata` file.

4. All paths are committed in a single commit to the sync target branch.

#### Hydration Queue Key

The current queue key groups apps by `{SourceRepoURL, SourceTargetRevision, DestinationRepoURL, DestinationBranch}`. With multiple dry sources, `SourceRepoURL` and `SourceTargetRevision` are replaced by a `SourcesFingerprint` — a deterministic hash of all normalized dry source URLs and target revisions. The key retains `DestinationRepoURL` and `DestinationBranch`.

#### Changes to Controller Dependencies

The `Dependencies` interface methods `GetRepoObjs` and `EvaluateAppRevisionsChanges` are updated to accept slices (`sources []ApplicationSource`, `revisions []string`) instead of single values. `GetRepoObjs` already wraps the single dry source into a slice internally — this change makes the slice the public API and removes the `len(resp) != 1` assertion. `EvaluateAppRevisionsChanges` return type changes from `(bool, string, error)` to `(bool, []string, error)` to return all resolved revisions.

This cascades through `newRevisionHasChanges()` (now evaluates all dry sources), `appNeedsHydration()` (receives `[]string` resolved revisions), and `ProcessAppHydrateQueueItem()` (persists `LastComparedDryRevisions` alongside `LastComparedDryRevision`).

#### Commit Server Changes

`CommitHydratedManifestsRequest` in `commit.proto` gains `repeated string dryShas` and `repeated string repoURLs` fields alongside the existing singular `drySha`. The `HydratorCommitMetadata` struct in `util/hydrator/hydrator.go` is extended with `DrySHAs` and `RepoURLs` fields. `GetCommitMetadata()` is updated to accept slices and populate both singular (first entry) and plural fields.

The `hydrator.metadata` file and git commit note (`refs/notes/hydrator.metadata`) are extended with `dryShas` and `repoURLs` arrays. The existing singular `drySha` and `repoURL` fields are preserved with the composite hash and first source's URL respectively, ensuring backward compatibility with external consumers.

#### GitOps Promoter Compatibility

The [GitOps Promoter](https://github.com/argoproj-labs/gitops-promoter) reads the `hydrator.metadata` file and git notes to track promotion lineage. It uses `HydratorMetadata.DrySha` to determine which dry commit produced a given hydrated branch.

With multiple dry sources, the composite `DrySha` remains a unique identifier of the complete dry state — if any source changes, the composite changes. The promoter can continue using `DrySha` for lineage tracking without modification. The `DryShas` array provides optional granularity for display (e.g. showing which individual sources were promoted).

The `HydratorMetadata` struct in the promoter (`api/v1alpha1/changetransferpolicy_types.go`) would need a `DryShas []string` field added to consume the new array, but this is additive and does not break existing behavior.

### Security Considerations

* Each dry source repository must be permitted by the Application's `AppProject`. The validation logic is extended to check all dry sources, not just the first.
* Write credentials for the sync target are resolved the same way as today — only the destination repo requires write access.
* No new authentication mechanisms are introduced. Each dry source repo uses existing Argo CD repository credentials.

### Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Increased hydration latency with many dry sources | Manifest generation for different dry sources can be parallelized (the existing `errgroup` pattern in `hydrate()` already does this for multiple apps). |
| Queue key change causes brief re-hydration during rolling upgrade | The queue key is internal and self-healing — the next `ProcessAppHydrateQueueItem` call re-enqueues with the new key format. Hydration is idempotent. |
| Composite DrySHA in commit notes is less human-readable | Individual per-source SHAs are preserved in `DrySHAs` status fields for auditability. The composite SHA is only used for the commit note dedup mechanism. |
| `DrySHA` → `DrySHAs` upgrade path during rolling update | De-duplication logic falls back to comparing singular `DrySHA` against `DrySHAs[0]` when `DrySHAs` is empty, ensuring the transition is seamless. |

### Upgrade / Downgrade Strategy

**Upgrade:**

* Existing applications using `drySource` (singular) continue to work unchanged. All new fields are `omitempty`.
* The `GetDrySources()` helper transparently wraps a singular `drySource` into a single-element slice, so all internal code paths work with both forms.
* `DrySHA` (singular) continues to be populated alongside `DrySHAs` (plural) for backward compatibility with any external tooling reading Application status.
* The `HydrationQueueKey` format change is internal. On upgrade, in-flight queue items using the old key format are simply not matched — the next reconciliation loop re-enqueues with the new format. No data loss occurs because hydration is idempotent.

**Downgrade:**

* If a user downgrades after creating applications with `drySources`, the older Argo CD version will ignore the unknown `drySources` field (standard Kubernetes behavior for unknown fields in CRDs).
* The `drySource` (singular) field will be empty for these apps, so the older version will fail validation with "drySource.repoURL is required". Users must convert their apps back to `drySource` before downgrading.
* Status fields `DrySHAs` and `LastComparedDryRevisions` are ignored by older versions — the singular `DrySHA` and `LastComparedDryRevision` fields remain populated and functional.

## Drawbacks

* **Increased complexity in the hydrator pipeline** — every code path that touches `DrySource` must now handle the plural form. This is mitigated by the `GetDrySources()` helper which abstracts the singular/plural distinction, matching the proven `GetSources()` pattern.

* **All-or-nothing re-hydration** — when any single dry source changes, all sources are re-rendered. For applications with many large dry sources, this may be slower than necessary. However, this matches how `sources` works for sync and keeps the implementation simple. Partial hydration could be explored as a future optimization.

## Alternatives

1. **Use ApplicationSets to compose from multiple repos** — users can create separate Applications per repo and use an ApplicationSet to coordinate them. This works but loses the single-Application view and requires more complex orchestration for deployment ordering.

2. **Pre-merge in CI** — users can use CI pipelines to merge manifests from multiple repos into a single repo, then point `drySource` at the merged result. This is the status quo workaround and defeats the purpose of the hydrator.

3. **Extend `sources` to work with the hydrator** — instead of adding `drySources` to `SourceHydrator`, extend the top-level `sources` field to work with hydration. This was rejected because it conflates two different concepts (sync sources vs. dry sources) and would require significant changes to how `sources` interacts with the sync pipeline.
