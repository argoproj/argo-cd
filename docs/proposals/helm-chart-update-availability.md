---
title: Show newer Helm chart versions for Helm-sourced Applications
authors:
  - "@Nova-Logic"
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2026-10-13
last-updated: 2026-10-13
---

# Show newer Helm chart versions for Helm-sourced Applications

Surface, read-only, that a newer version of an Application's Helm chart exists in its Helm repository
(classic `index.yaml` or OCI), so users can see stale charts without external tooling.

Tracking issue: [#6127](https://github.com/argoproj/argo-cd/issues/6127). Related: [#2552](https://github.com/argoproj/argo-cd/issues/2552),
[#10618](https://github.com/argoproj/argo-cd/issues/10618), [#11352](https://github.com/argoproj/argo-cd/issues/11352) / PR [#11575](https://github.com/argoproj/argo-cd/pull/11575).

## Open Questions

1. **Default on or opt-in?** This proposal suggests the feature is enabled by default (it is read-only and lazy),
   with an `argocd-cm` switch to disable it. Maintainers may prefer opt-in for the first release.
2. **Cache TTL for version lists.** The Helm index / OCI tags cache currently reuses `--revision-cache-expiration`
   (default 3m). For a purely informational check a longer TTL (proposed default 1h) seems appropriate. Should this be
   a new repo-server flag, or should it reuse `ARGOCD_REPO_CACHE_EXPIRATION`?
3. **Applications list badge.** Is a badge in the Applications list (tiles/table) desirable in a follow-up, given it
   needs either per-app requests or a controller-maintained status field (see Non-Goals)?
4. **Prerelease handling.** Proposed: ignore prereleases unless the currently deployed version is itself a
   prerelease. Is that the expected behavior?

## Summary

For Applications whose source is a Helm chart from a Helm repository, Argo CD will compare the deployed (resolved)
chart version with the versions published in the repository and show, on the Application details page, whether a
newer **patch**, **minor** or **major** version is available, including versions outside the `targetRevision`
constraint. The check is informational only: it never changes `targetRevision`, never triggers a sync, and is not part
of the Application `status`.

## Motivation

Today there is no native signal that a Helm chart is outdated:

- With an **exact pin** (`targetRevision: 1.2.3`), the most controlled and most common setup, Argo CD gives no feedback
  at all when newer versions ship. Internally, `newHelmClientResolveRevision` returns early for exact versions
  (`versions.IsVersion(revision)`), so the repository index is never even read.
- With a **semver range** (the resolution of #2552, available since v1.4), Argo CD auto-upgrades within the range,
  but a range such as `^8.0.0` is bounded `<9.0.0` by design and will never surface `9.x`. Chart versions also do not
  map 1:1 to application versions, so users cannot rely on ranges alone to learn about major releases.
- Users fall back to external tools (Renovate, Dependabot-style bots, custom scripts). These require Git write access,
  separate credentials for private Helm/OCI registries, and do not work in all environments (for example air-gapped
  installs where only the repo-server can reach the registry).

The repo-server already has everything required: repository credentials, Helm index and OCI tag clients
(`nativeHelmChart.GetIndex`, `nativeHelmChart.GetTags`), semver utilities (`util/versions`), and a cache for indexes and
tags (`helm-index|<repo>`). Argo CD already shows chart metadata (#11575, `RevisionChartDetails`). Adding the
version comparison is a small, contained step on top of that.

### Goals

- Show on the Application details page whether a newer chart version exists for each Helm-repository source, split into
  latest patch / minor / major relative to the deployed version.
- Work for classic Helm repositories (`index.yaml`) and OCI Helm repositories, using the credentials Argo CD already has.
- Support single-source and multi-source Applications (per source).
- Be read-only and lazy: no controller changes, no CRD/status changes, no extra work during reconciliation.
- Bound the extra load on Helm repositories through caching.
- Expose the data through the Argo CD API so the CLI and other clients can reuse it.

Success: a user viewing a pinned or range-tracked Helm Application can tell from the UI, without external tools,
that `9.1.0` exists while `8.4.2` is deployed.

### Non-Goals

- Automatically updating `targetRevision`, opening pull requests, or syncing. That remains the domain of semver ranges,
  Renovate, or Argo CD Image Updater-like tools.
- Charts sourced from Git repositories (no version concept beyond Git refs).
- A badge in the Applications list, notifications triggers, or Prometheus metrics. These would need a
  controller-maintained status field or periodic background checks, which have very different cost and API
  implications. They can be proposed separately once this lands.
- Changelog rendering. The UI links to the chart `home` URL (already available through `ChartDetails`) and, when
  present, the index `sources` URL.

## Proposal

### Use cases

#### Use case 1: pinned chart
As an operator who pins `targetRevision: 8.4.2` for change control, I want to see on the Application page that
`8.4.5` (patch), `8.6.0` (minor) and `9.1.0` (major) are available, so I can plan upgrades.

#### Use case 2: semver range
As an operator tracking `^8.0.0` with auto-sync, I want to be told that `9.1.0` exists, because the range will never
pick it up.

#### Use case 3: private OCI registry
As an operator using a private OCI Helm registry reachable only from the cluster, I want the same signal without
configuring a separate bot with registry credentials.

#### Use case 4: API / CLI
As a platform team, I want to query update availability for an Application through the Argo CD API so I can build
reports across many Applications.

### Implementation Details/Notes/Constraints

#### Repo-server: list chart versions

Add an RPC to `RepoServerService` (`reposerver/repository/repository.proto`):

```protobuf
rpc GetHelmChartVersions(HelmChartVersionsRequest) returns (HelmChartVersionsResponse) {}

message HelmChartVersionsRequest {
  github.com.argoproj.argo_cd.v3.pkg.apis.application.v1alpha1.Repository repo = 1;
  string chart = 2;
  bool noCache = 3;
}

message HelmChartVersion {
  string version = 1;
  string appVersion = 2;       // empty for OCI repositories
  google.protobuf.Timestamp created = 3;
}

message HelmChartVersionsResponse {
  repeated HelmChartVersion versions = 1;
}
```

Implementation:

- Classic repositories: `helmClient.GetIndex(ctx, noCache, s.initConstants.HelmRegistryMaxIndexSize)`, then
  `index.GetEntries(chart)`. `util/helm/index.go` `Entry` is extended to also decode `appVersion` and `sources`.
  The raw index is already cached as bytes, so this only changes decoding.
- OCI repositories: `helmClient.GetTags(ctx, chart, noCache)` (tags with `_` mapped back to `+`, as today).
- Results are cached under a new key `helm-chart-versions|<repoURL>|<chart>` with a dedicated expiration
  (see Open Questions; proposed `--helm-chart-versions-cache-expiration`, default `1h`). This avoids re-downloading
  large indexes (some public indexes are tens of MB) on every page view while the 3m revision cache keeps serving
  manifest generation.
- Index downloads keep the existing per-repo lock (`indexLock`) and the `--helm-registry-max-index-size` limit.

The existing `GetHelmCharts` RPC is not reused: it always bypasses the cache (`GetIndex(ctx, true, ...)`), returns
every chart in the repository, and has no OCI branch.

#### API server: update availability per Application source

Add to `ApplicationService` (`server/application/application.proto`):

```protobuf
rpc ChartVersionUpdates(ChartVersionUpdatesQuery) returns (ChartVersionUpdatesResponse) {
  option (google.api.http).get = "/api/v1/applications/{name}/chartupdates";
}

message ChartVersionUpdatesQuery {
  required string name = 1;
  optional string appNamespace = 2;
  optional string project = 3;
  optional int32 sourceIndex = 4;
}

message ChartVersionUpdatesResponse {
  string chart = 1;
  string targetRevision = 2;
  string currentVersion = 3;   // resolved, deployed version
  string latestPatch = 4;      // empty when none newer
  string latestMinor = 5;
  string latestMajor = 6;      // highest version overall
  bool latestMajorInTargetRange = 7;
  repeated HelmChartVersion newerVersions = 8; // newest first, capped (e.g. 20)
}
```

Behavior:

- RBAC: same as `RevisionChartDetails`, `applications, get` on the Application.
- The source is resolved with `getAppSourceBySourceIndexAndVersionId`. Non-Helm sources (no `chart`) return
  `codes.InvalidArgument`.
- `currentVersion` is `status.sync.revision`, or `status.sync.revisions[sourceIndex]` for multi-source Applications,
  which holds the resolved chart version for Helm sources (set in `controller/state.go` `CompareAppState`).
- Comparison uses `github.com/Masterminds/semver/v3` (already used by `util/versions`). Versions that do not parse as
  semver are ignored. Prereleases are ignored unless `currentVersion` is a prerelease.
- `latestPatch` is the highest version with the same major.minor, `latestMinor` the highest with the same major,
  and `latestMajor` the highest overall. Each is only set when it is greater than `currentVersion`.
- `latestMajorInTargetRange` tells the UI whether the newest version would be picked up by the current
  `targetRevision` constraint (for ranges), which explains why it is not deployed yet (for example, a sync is pending).

#### UI

- `application-status-panel/revision-metadata-panel.tsx` currently renders nothing for Helm. For Helm sources it
  will show a small "update available" indicator next to the synced revision (for example
  `8.4.2 · 9.1.0 available`), colored by the highest available class (patch / minor / major).
- `application-details/application-details.tsx` `getContentForChart` (the "Helm Chart" row) and
  `application-summary/application-summary.tsx` (the `TARGET REVISION` row) show the latest patch / minor / major
  with a tooltip and links to the chart `home` URL (`ChartDetails.home`) and, when present, `sources`.
- Data is fetched lazily with `services.applications.chartVersionUpdates(name, appNamespace, sourceIndex)` only when
  these panels are visible, never from the Applications list.
- When the feature is disabled or the request fails, the UI renders nothing (no error banners).

#### Configuration

`argocd-cm`:

```yaml
data:
  # default "true"; set to "false" to disable update checks and hide the UI indicator
  helm.chartUpdateCheck.enabled: "true"
```

When disabled, `ChartVersionUpdates` returns `codes.Unimplemented`, and the UI hides the indicator.

#### CLI (follow-up within this proposal's scope)

`argocd app get <name> --show-chart-updates` prints the same information per source.

### Detailed examples

Application:

```yaml
spec:
  source:
    repoURL: https://charts.example.com
    chart: ingress-nginx
    targetRevision: ^4.10.0
status:
  sync:
    revision: 4.11.2
```

Repository index contains `4.10.0 … 4.11.2, 4.11.3, 4.12.1, 5.0.0, 5.1.0-rc.1`.

`GET /api/v1/applications/ingress-nginx/chartupdates` returns:

```json
{
  "chart": "ingress-nginx",
  "targetRevision": "^4.10.0",
  "currentVersion": "4.11.2",
  "latestPatch": "4.11.3",
  "latestMinor": "4.12.1",
  "latestMajor": "5.0.0",
  "latestMajorInTargetRange": false,
  "newerVersions": [{"version": "5.0.0"}, {"version": "4.12.1"}, {"version": "4.11.3"}]
}
```

`5.1.0-rc.1` is ignored because the deployed version is not a prerelease. The UI shows
`4.11.2 · 5.0.0 available (major)`. The tooltip lists `4.11.3` (patch) and `4.12.1` (minor). Here `4.12.1` satisfies
`^4.10.0`, so an auto-synced Application will move to it on the next reconcile, while `5.0.0` requires a
`targetRevision` change.

### Security Considerations

- No new credentials or network destinations. The repo-server talks only to repositories already configured for the
  Application, with the same credentials and TLS settings it uses for manifest generation.
- Access is guarded by the existing `applications, get` RBAC on the Application. The response only contains version
  strings, timestamps and `appVersion` of a chart the user can already deploy and inspect.
- Untrusted index content: decoding is limited to the existing `--helm-registry-max-index-size`. The new fields
  (`appVersion`, `sources`) are rendered as text. `sources` URLs are only rendered as links if they use `http`/`https`.
- Denial of service against registries: requests are cached per `(repo, chart)` and deduplicated by the existing
  per-repo index lock, so many users opening the same Application cause at most one download per TTL.

### Risks and Mitigations

- **Extra registry traffic for pinned charts.** Pinned charts currently never read the index. Mitigation: requests are
  lazy (only when the details page is opened), use a dedicated, longer-lived cache, and can be disabled globally.
- **Large public indexes.** Downloading a large `index.yaml` is expensive. Mitigation: shared cache with the existing
  index cache where possible, the existing max index size limit, and the per-repo lock.
- **Non-semver chart versions.** These are ignored, and the indicator is not shown. This is documented.
- **Misleading signal with mirrors.** If an Application uses a mirror that lags upstream, the signal reflects the
  mirror. This is expected, because Argo CD can only see what it can deploy.

### Upgrade / Downgrade Strategy

- Upgrade: no action required. The new RPCs are additive, there are no CRD or status changes, and the indicator
  appears automatically (unless disabled in `argocd-cm`).
- Downgrade: the new API endpoint and cache keys disappear. Nothing is persisted outside the cache.
- Mixed versions during rollout (new API server, old repo-server): the API server receives `Unimplemented` from the
  repo-server and returns an empty result, and the UI hides the indicator.

### Test plan

- Unit tests for the version classification (patch/minor/major, prereleases, non-semver tags, constraints).
- Repo-server tests for classic and OCI repositories using the existing fake Helm/OCI fixtures, including cache
  hits and the max index size.
- API server tests for RBAC, multi-source `sourceIndex`, and non-Helm sources.
- UI tests for the status panel and details page rendering, and for hiding the indicator when disabled or on error.
- An e2e test with a local Helm repository fixture publishing a newer chart version.

## Drawbacks

- New API surface (one repo-server RPC, one API endpoint, one config key) for an informational feature.
- Reads Helm repositories for pinned charts, which today never happens, adding some load to registries.
- Overlaps partially with external tools (Renovate) that already solve update discovery and also open pull requests.

## Alternatives

1. **Rely on semver ranges (status quo, #2552).** Ranges never surface major versions and give no feedback for pins.
2. **External tools (Renovate, custom bots).** These need Git write access and separate registry credentials, and
   do not work where only the cluster can reach the registry. They remain complementary.
3. **UI-only implementation using the existing `/repositories/{repo}/helmcharts` endpoint.** It always bypasses the
   cache and downloads the full index on every call, returns all charts in the repository, does not support OCI, and
   requires repository-level permissions that Application viewers may not have.
4. **Controller-maintained status field (for example `status.sourceUpdates`).** This enables list badges,
   notifications and metrics, but it adds periodic registry reads for every Helm Application on every reconcile cycle
   and introduces a CRD/status change. It is deferred to a possible follow-up proposal that can build on the
   repo-server RPC introduced here.
