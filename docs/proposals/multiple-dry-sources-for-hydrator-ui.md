---
title: "UI: Multiple DrySources for SourceHydrator"
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

creation-date: 2026-06-11
last-updated: 2026-06-11
---

# UI: Multiple DrySources for SourceHydrator

UI changes to support multiple dry sources and `DrySource` field parity with `ApplicationSource` in the Source Hydrator.

**Parent proposal**: [Multiple DrySources for SourceHydrator](multiple-dry-sources-for-hydrator.md)

## Open Questions

* How should multiple dry source revisions be displayed in the status panel — as a list, a collapsed summary ("3 sources hydrated"), or inline with source names?
* Should the create panel support adding `drySources` entries from day one, or only support editing existing `drySources` (created via YAML/kubectl)?

## Summary

The parent proposal adds `drySources` (plural), `chart`, `ref`, and `name` fields to the hydrator API. This companion proposal covers the UI changes required to display, create, and edit applications using these new fields. The changes span TypeScript model interfaces, utility functions, display components, and create/edit forms.

## Motivation

Without UI updates, applications using `drySources` will either render incorrectly (showing only the first source or empty fields) or break utility functions that assume a singular `drySource`. Users who configure `drySources` via YAML or kubectl need the UI to accurately reflect their configuration.

### Goals

1. **Update TypeScript models** — add all new fields from the API changes so the UI can consume them.

2. **Fix utility functions** — ensure helper functions handle `drySources` (plural) with correct fallback to `drySource` (singular).

3. **Update display components** — hydration status, revision links, and operation details render correctly for multi-source applications.

4. **Support create/edit for drySources** — the create panel and summary editor support adding, removing, and editing multiple dry source entries.

### Non-Goals

* **Backend API changes** — covered by the parent proposal.
* **Mobile or responsive layout redesign** — existing layout patterns are reused.

## Proposal

### Affected Files

| Category | File | Change |
|----------|------|--------|
| Models | `ui/src/app/shared/models.ts` | Add new fields to interfaces |
| Utilities | `ui/src/app/applications/components/utils.tsx` | Handle plural dry sources in helpers |
| Tests | `ui/src/app/applications/components/utils.test.tsx` | Add plural test cases |
| Create | `ui/src/app/applications/components/application-create-panel/hydrator-source-panel.tsx` | Multi-source form |
| Create | `ui/src/app/applications/components/application-create-panel/application-create-panel.tsx` | Toggle logic for plural |
| Edit | `ui/src/app/applications/components/application-summary/application-summary.tsx` | List rendering for plural |
| Display | `ui/src/app/applications/components/application-hydrate-operation-state/application-hydrate-operation-state.tsx` | Multiple revision links |
| Display | `ui/src/app/applications/components/application-status-panel/application-status-panel.tsx` | Status message for plural |
| Display | `ui/src/app/applications/components/application-parameters/application-parameters.tsx` | Dry source details |
| Display | `ui/src/app/applications/components/application-details/application-details.tsx` | Hydrate operation state extraction |
| No change | `ui/src/app/applications/components/applications-list/application-table-row.tsx` | Phase icon only — no change |
| No change | `ui/src/app/applications/components/applications-list/application-tile.tsx` | Phase icon only — no change |
| No change | `ui/src/app/applications/components/applications-list/applications-summary.tsx` | Phase stats — no change |

### Use Cases

#### Use case 1: Viewing a multi-source hydrated application

As a user, I want to see all dry sources listed in the application details and status panels, with individual revision links for each source, so I can understand which sources contributed to the hydrated output.

#### Use case 2: Creating an application with multiple dry sources

As a user, I want to add multiple dry source entries in the application create panel — including OCI charts and ref-only entries for shared values.

#### Use case 3: Editing dry sources on an existing application

As a user, I want to add, remove, or reorder dry source entries in the application summary editor, with the same validation the backend enforces (mutual exclusion, ref uniqueness, repoURL required).

### Implementation Details

#### 1. Model Changes (`models.ts`)

All TypeScript interfaces are updated to mirror the Go API changes from the parent proposal:

* `DrySource` — add optional `chart`, `ref`, and `name` fields
* `SourceHydrator` — `drySource` becomes optional, add `drySources?: DrySource[]`
* `HydrateOperation` / `SuccessfulHydrateOperation` — add `dryRevisions?: string[]` alongside existing `drySHA`. Each entry is the resolved revision for the corresponding dry source (git SHA, chart version, or OCI digest). Follows the `Revision`/`Revisions` pattern from `SyncOperation`.
* `SourceHydratorStatus` — add `lastComparedDryRevisions?: string[]`.

#### 2. Utility Function Changes (`utils.tsx`)

* **`getAppDrySource(app)`** — updated to check `drySources` first and return the first entry when set. Falls back to `drySource` (singular). Both paths return all fields including `chart`, `ref`, `name`, and tool-specific configs.
* **New: `getAppDrySources(app)`** — returns all dry sources as `ApplicationSource[]`, with the same fallback to wrapping the singular `drySource` in a single-element array.
* **`getHydratorSyncSourceRepoURL(sourceHydrator)`** — updated fallback chain: `syncSource.repoURL` → `drySources[0].repoURL` → `drySource.repoURL`.
* **`hydrationStatusMessage(app)`** — updated to show per-source revisions from `dryRevisions` when available, displaying source name or repo URL next to each revision. Handles heterogeneous revision types (git SHA, chart version, OCI digest).

#### 3. Display Component Changes

**`application-hydrate-operation-state.tsx`** — updated to iterate over `dryRevisions` when present, rendering a revision link per source with the source name or repo URL as label. Falls back to the singular `drySHA` for backward compatibility with apps that haven't been re-hydrated.

**`application-status-panel.tsx`** — the `hydrationStatusMessage()` utility fix handles this. No direct component changes needed.

#### 4. Create/Edit Panel Changes

**`hydrator-source-panel.tsx`** — the most significant UI change. Currently renders three static sections (Dry Source, Sync Source, Hydrate To) with hardcoded field paths. Updated to:

* Detect whether `drySources` or `drySource` is in use
* Render a dynamic list of dry source entries with add/remove buttons
* Each entry renders the same field set (repoURL, targetRevision, path, chart, ref, name) plus tool-specific config (helm, kustomize, directory, plugin)
* Follow the same list pattern used by `sources` in the regular create panel

The Sync Source and Hydrate To sections remain unchanged — they are singular regardless of how many dry sources exist.

**`application-create-panel.tsx`** — toggle logic updated:

* On enable: if `spec.sources` exists, convert to `spec.sourceHydrator.drySources`; if `spec.source` exists, convert to `spec.sourceHydrator.drySource`
* On disable: reverse conversion, preserving the singular/plural form

**`application-summary.tsx`** — edit view updated to render dry sources as a list with inline editing, matching the pattern used for `sources` editing in the non-hydrator path.

### Security Considerations

No new security concerns. The UI changes are purely presentational and use existing Argo CD RBAC for any write operations. All validation occurs server-side.

### Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Complex multi-source form is confusing for users | Reuse the existing `sources` create panel UX patterns. Show `drySource` (singular) by default, with an explicit toggle or "Add source" button to switch to `drySources`. |
| Rendering many dry sources clutters the status panel | Show a collapsed summary ("3 dry sources hydrated") with expand-on-click for individual revision details. |
| Backward compatibility with apps created before the API change | Utility functions fall back to `drySource` (singular) when `drySources` is absent. The UI never assumes plural. |

### Upgrade / Downgrade Strategy

**Upgrade**: The UI gracefully handles both forms. When the backend returns `drySources`, the UI renders the list. When it returns `drySource`, the UI renders the single-source view. No user action required.

**Downgrade**: If the UI is downgraded before the backend, the old UI ignores unknown fields (`drySources`, `drySHAs`, etc.) and renders using `drySource` / `drySHA` — which the backend continues to populate for backward compatibility.

## Drawbacks

* **Increased form complexity** — the create/edit panel for hydrator becomes significantly more complex with dynamic source list management. This is mitigated by reusing established patterns from the `sources` create panel.

* **Testing surface** — files across models, utilities, display, and form components need changes. Automated tests for utility functions and snapshot tests for display components help manage regression risk.

## Alternatives

1. **YAML-only for drySources** — support `drySources` only via YAML/kubectl, with no create/edit form. The UI would display but not edit multiple dry sources. Simpler to implement but creates a second-class experience for multi-source hydrator users.

2. **Embed in existing sources UI** — reuse the existing multi-source form components instead of building hydrator-specific ones. Rejected because the hydrator source panel has different fields (syncSource, hydrateTo) that don't map to the regular source panel structure.
