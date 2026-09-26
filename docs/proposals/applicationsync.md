---
title: ApplicationSync - declarative, ordered syncs of groups of Applications
authors:
  - "@blakepettersson"
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2026-09-25
last-updated: 2026-09-25
---

# ApplicationSync

A namespaced `ApplicationSync` resource requests a one-off sync of one or more Applications, ordered by
dependencies and gated on health. Manual syncs from the API and UI create one too, so every manual sync leaves a
record. A later `ApplicationSyncPolicy` resource creates an ApplicationSync whenever its apps drift, checked on
the same period as auto-sync today. That makes it auto-sync for a group, and ApplicationSet RollingSync can be
rebuilt on top of it.

## Open Questions

1. **Revision overrides through kubectl.** `application.sync.requireOverridePrivilegeForRevisionSync` cannot be
   enforced on ApplicationSyncs created through the Kubernetes API (see [Authorization](#authorization)). Should
   those still be allowed to pin `revisions`, as writing `operation` allows today? Or should revision pinning be
   limited to ApplicationSyncs created by Argo CD's own components? The prototype takes a middle path: with the
   setting on, a pin must resolve to the same revision as `targetRevision`, so rollouts can still pin but nobody
   can pick an arbitrary revision through kubectl.
2. **Busy apps from the API.** Today the Sync API rejects a request when an operation is already running. Should
   the API keep rejecting, or queue the ApplicationSync the way kubectl-created ones are queued? The prototype
   keeps rejecting.
3. **Selectors in groups.** Groups list apps by name. Should a group also accept a label selector, as RollingSync
   steps do, or should selectors stay in the ApplicationSet controller, which resolves them to names? This also
   decides where globs and patterns ([#14724](https://github.com/argoproj/argo-cd/issues/14724)) and policies spanning several ApplicationSets ([#14458](https://github.com/argoproj/argo-cd/issues/14458))
   are handled.
4. **Reconciler.** ApplicationSync and ApplicationSyncPolicy are reconciled either in the application
   controller or in a new, dedicated controller. They are not reconciled in the ApplicationSet controller;
   in [RollingSync on ApplicationSync](#rollingsync-on-applicationsync) it only creates ApplicationSyncs. Which of the two?
   - **Application controller**, as in the prototype: no new component, and it reuses the existing sharding.
     A policy is then evaluated on the shard that owns the first app of the first group, the same rule that decides
     ApplicationSync ownership.
   - **Separate controller** with leader election: one clear owner per object, and no extra load or policy
     logic in a sharded component. It is one more deployment to run, and it still writes `app.operation`
     through the API.
5. **Freshness grace period.** RollingSync waits `--refresh-grace-period-seconds` (default 30) for apps to
   reconcile on their own before forcing a refresh. Does the application controller need the same grace
   period, or can it refresh the apps straight away, since it owns the refresh queue? The prototype refreshes
   straight away.

## Summary

Today a sync is requested through the API, CLI or UI, or by writing the `operation` field on the Application
([Sync Applications with Kubectl](../user-guide/sync-kubectl.md)). That covers one app at a time. It mutates
the Application object, and nothing survives as a record of the request once the next operation replaces
`status.operationState`.

`ApplicationSync` is a separate, immutable resource. It lists Applications in groups, and a group can depend on
other groups. The controller syncs a group's apps in parallel once the groups it depends on have synced and are
Healthy, and reports progress per group and per app in its status. Completed ApplicationSyncs stay around as history until garbage collection removes them.

## Motivation

- **GitOps-friendly sync requests.** Patching `operation` onto an Application that is itself managed from Git,
  by an app-of-apps or an ApplicationSet, is an out-of-band edit to a managed object. A separate object can be
  applied, committed or templated without touching the Application.
- **Least privilege.** Syncing through kubectl today needs `patch` on `applications`, which also allows
  changing the Application's spec. `create` on `applicationsyncs` grants only "sync these apps".
- **One path for manual syncs.** API, UI and kubectl syncs all create the same object, with the same semantics
  and one history.
- **Ordering across apps.** The options today are ApplicationSet RollingSync, which is tied to one
  ApplicationSet and its label selectors, or app-of-apps with sync waves. The latter needs the Application
  health check restored (see [upgrading to 1.8](../operator-manual/upgrading/1.7-1.8.md)) and a parent
  Application.
- **Moving progressive sync out of the ApplicationSet.** In the discussion on [#28927](https://github.com/argoproj/argo-cd/issues/28927) about promoting
  Progressive Sync to stable, maintainers supported eventually giving it its own CRD and controller, as an
  independent proposal. This is that proposal. It keeps the `rollingSync` API as a front end, so users don't
  have to migrate their configuration. Several open RollingSync bugs come from the ApplicationSet controller
  rebuilding a sync state machine out of Application status fields; see
  [Known RollingSync issues](#known-rollingsync-issues).
- **History and group auto-sync.** No object records a sync request. Auto-sync also works one app at a time,
  with no way to auto-sync a group of apps in dependency order.

### Goals

- Sync groups of Applications with `kubectl apply`, ordered by group dependencies and gated on health.
- One immutable object per request, with per-app status, kept as bounded history.
- API and UI syncs keep enforcing Argo CD RBAC exactly as today, and create an ApplicationSync once allowed.
- Enforce the same guards as an API sync: sync windows, project permissions and `server.sync.replace.allowed`.
- Work with sharded application controllers.
- Provide the building block that RollingSync and group auto-sync are built on.

### Non-Goals

- Removing `operation` from the Application. It stays as the mechanism the controller uses. Auto-sync,
  rollbacks and syncs with local manifests keep writing it directly.
- Enforcing Argo CD RBAC on ApplicationSyncs created through the Kubernetes API. Like writing `operation` today,
  access through the Kubernetes API is governed by Kubernetes RBAC (see [Alternatives](#alternatives)).
- Syncing Applications across namespaces from one `ApplicationSync`.
- A general workflow engine: no arbitrary steps, approvals or hooks between apps.
- Rollout analysis beyond health, such as metric checks or analysis runs ([#11932](https://github.com/argoproj/argo-cd/issues/11932)). This is future
  work, and a group finishing is the natural point to hook it in.
- Rolling back ApplicationSet template changes. Reverting a template is done in Git. Rolling back to earlier
  revisions is covered (see [History and rollback](#history-and-rollback)).

## Proposal

The proposal has five parts. They are designed to ship together, and are described separately only for readability.

| Part | Scope |
|---|---|
| [The ApplicationSync resource](#the-applicationsync-resource) | Groups with `dependsOn`, health gating, garbage collection. |
| [Rollout controls](#rollout-controls) | What RollingSync and the Sync API need: a concurrency cap, per-app revisions, failure and window handling, freshness, status and metrics. |
| [API and UI syncs](#api-and-ui-syncs) | Manual syncs from the API and UI create an ApplicationSync. |
| [ApplicationSyncPolicy](#applicationsyncpolicy) | Auto-sync for a group, on the same period as auto-sync today. |
| [RollingSync on ApplicationSync](#rollingsync-on-applicationsync) | ApplicationSet RollingSync rebuilt on top of ApplicationSync. |

### Use cases

#### Use case 1: ordered release

As an operator, I apply one manifest that syncs `db`, then `api` and `worker` in parallel, then `web`. Each app
only starts once the apps it depends on are Healthy.

#### Use case 2: sync from a pipeline without mutating Applications

As a CI pipeline, I create an ApplicationSync with my Kubernetes service account and wait on `status.phase`. I
don't need to write to the Application, and I don't need an Argo CD API token. A Kubernetes Role grants my
service account `create` on `applicationsyncs` in the apps' namespace.

#### Use case 3: sync history

As an operator, I click Sync in the UI. The sync is recorded as an ApplicationSync, so I can list the recent
syncs of an app, with who requested them and how each one ended.

#### Use case 4: auto-sync a group

As a platform team, I want a set of apps to auto-sync as a unit in dependency order whenever they drift,
instead of each app auto-syncing on its own. Auto-sync is off on the individual apps.

#### Use case 5: progressive rollout of an ApplicationSet

As an ApplicationSet user, I get the same RollingSync behavior as today. The rollout runs as `ApplicationSync`s,
so each run is visible and kept as history.

### The ApplicationSync resource

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSync
metadata:
  name: release-42
  namespace: argocd
spec:            # immutable, except spec.paused
  prune: true    # also: dryRun, syncStrategy, syncOptions, retry, applied to every app
  groups:
  - name: data
    apps: [{name: db}]
  - name: backend
    dependsOn: [data]
    apps: [{name: api}, {name: worker}]
  - name: frontend
    dependsOn: [backend]
    apps: [{name: web}]
status:
  phase: Running # Pending | Running | Succeeded | Failed
  startedAt: "2026-09-25T09:00:00Z"
  groups:
  - name: data
    phase: Succeeded
    apps:
    - {name: db, phase: Succeeded, revisions: [0b1c2d3]}
  - name: backend
    phase: Running
    apps:
    - name: api
      phase: Progressing
      message: waiting for application to become Healthy (currently Progressing)
    - {name: worker, phase: Running}
  - name: frontend
    phase: Pending
    message: waiting for group "backend"
    apps:
    - {name: web, phase: Pending}
```

A single app is a single group with one entry:

```yaml
spec:
  groups:
  - name: guestbook
    apps: [{name: guestbook}]
```

Semantics:

- **Groups.** `spec.groups` is the only way to list apps. Each group has a unique `name`, its `apps`, and
  optional `dependsOn` naming other groups. A group starts once every group it depends on has succeeded, and
  its apps sync in parallel. A group succeeds when all its apps have, and a group with no apps counts as
  complete. Ordering between individual apps is expressed by putting them in separate groups.
- **Per-app phases.** Each app moves through `Pending → Running → Progressing → Succeeded`, or ends in
  `Failed` or `Skipped`. `Progressing` means the sync succeeded and the app is not yet Healthy.
- **Health gate.** An app is done when its sync operation succeeded and the app is Synced and `Healthy`, as in
  RollingSync. An app whose sync succeeds but stays OutOfSync doesn't pass. Synced is judged against the pinned
  revisions (see [Revisions](#revisions)). Sync and health status are only read once `status.reconciledAt` is at
  or after the operation's `finishedAt`, and only when it reflects the app's current spec (see
  [freshness](#rollout-controls)). A `Degraded` app fails, unless `onFailure: Wait` is set. `doneWhen: Healthy`
  drops the Synced requirement. `dryRun` skips the health gate.
- **Failures.** When an app fails, the other apps in its group still finish, and the group then fails. Every
  group that depends on it, directly or indirectly, is marked `Skipped`, along with its apps. Groups that don't
  depend on it keep going, and the ApplicationSync ends as `Failed`.
- **Waiting.** An app that does not exist yet, or already has an operation running, waits. So a single
  `kubectl apply -f dir/` can create the Applications and the ApplicationSync together.
- **Validation.** An unknown group in `dependsOn`, a cycle, or an app listed in more than one group fails the
  ApplicationSync immediately. The CRD rejects duplicate group names and an empty `groups` list.
- **Immutability.** Every field of `spec` except `paused` is immutable, and `status` is a subresource. CEL can't
  compare a struct minus one field, so the rule compares each field, including whether optional fields are set.
  Re-applying an unchanged file is a no-op; re-applying a changed one is rejected. To sync again, create a new
  ApplicationSync. Deleting one cancels it: operations already running finish, and nothing new starts.
- **No auto-sync.** Apps in an ApplicationSync must not have auto-sync enabled, whoever created the
  ApplicationSync: a user, a policy or the ApplicationSet controller. Auto-sync would otherwise sync an app ahead of
  the groups it depends on, or undo a pinned revision. Instead of pausing auto-sync while an ApplicationSync runs,
  the controller refuses to sync an app that has `syncPolicy.automated` enabled, and says so in the app's status.
  To have a group of apps kept in sync automatically, use an [ApplicationSyncPolicy](#applicationsyncpolicy).
- **Garbage collection.** The 20 most recent completed ApplicationSyncs are kept per set of apps, taken across
  all groups, regardless of how the apps are grouped or ordered. Older ones are deleted. The limit is set with
  `controller.applicationsync.history.limit` in `argocd-cmd-params-cm`, and `0` disables deletion.

#### History and rollback

Completed ApplicationSyncs are the sync history of their apps, and each one records the concrete revisions it
synced in `status.groups[].apps[].revisions`. Rolling back means creating a new ApplicationSync that pins those
revisions ([#25741](https://github.com/argoproj/argo-cd/issues/25741)). A tool or the UI can offer this as "re-run with the revisions from this run".
Rolling back an ApplicationSet template change is not covered; the template is reverted in Git.

### Authorization

There are two ways to create an ApplicationSync, and each keeps the authorization model it has today:

| Created through | Who decides | Checks |
|---|---|---|
| Kubernetes API (`kubectl`, CI, GitOps) | Kubernetes RBAC | `create` on `applicationsyncs` in the apps' namespace. Argo CD RBAC is not consulted, the same as writing `operation` today. |
| Argo CD API or UI ([API and UI syncs](#api-and-ui-syncs)) | Argo CD RBAC | The Sync API's existing checks (`get`, `sync`, and `override` where required), before `argocd-server` creates the object. |

Either way, the controller then enforces the project's sync windows, destinations and source integrity, and
`server.sync.replace.allowed`, the same as for any sync.

Granting `create` on `applicationsyncs` is therefore equivalent to granting sync on every Application in that
namespace. The docs must say so plainly.

### Rollout controls

The core resource covers ordered one-off syncs. Replacing RollingSync and the Sync API also needs the controls
below. Each item names the RollingSync behavior it matches, as implemented in `applicationset/progressivesync`.

```yaml
spec:
  initiatedBy: {username: applicationset-controller, automated: true}
  onFailure: Wait        # Fail (default) | Wait
  doneWhen: SyncedAndHealthy  # SyncedAndHealthy (default) | Healthy
  skipIfSynced: true
  retry: {limit: 5}
  groups:
  - name: step-1
    maxParallel: 1
    apps:
    - name: guestbook-dev
      revisions:
      - revision: 4f1c2e9
      prune: false
```

**Rollout shape**

- **Concurrency cap.** Add `maxParallel` on a group, a number or percentage, matching `maxUpdate`. It counts the
  apps in the group that are `Running` or `Progressing`. A percentage that rounds down to 0 becomes 1, except
  `0%`. `0` or `0%` pauses the group, which RollingSync users rely on as a pause switch.
- **Pause and resume.** `spec.paused` is the only field that can change after creation. While it is `true`, no
  new app syncs start, and syncs already running finish. Setting it back to `false` resumes the run where it
  stopped. When a RollingSync user sets `maxUpdate` to 0, the ApplicationSet controller pauses the current run
  this way instead of replacing it, so the rollout doesn't restart. Any other `maxUpdate` change only applies to
  the next run, because `maxParallel` can't change on a running one.

**What gets synced**

- **Per-app revisions.** Add `revisions` per app, so each source of each app can be pinned. RollingSync records
  `TargetRevisions` per app, so a commit that lands mid-rollout can't reach apps that haven't synced yet. The
  rules are in [Revisions](#revisions).
- **Skip if synced.** Add `skipIfSynced: true`. An app that is already Synced at its pinned revision is not
  synced again, but still waits to be Healthy before it counts as done. RollingSync does the same: a Synced app
  moves straight to `Healthy`, or to `Progressing` if it isn't Healthy yet.
- **Per-app sync settings.** Add `prune` per app, overriding the spec-level value. RollingSync takes prune from
  each app's own `automated.prune` before forcing auto-sync off, so the ApplicationSet controller copies that
  value in. Retry and sync options already come from each app's `syncPolicy` unless the spec overrides them.
  Add `retry` per app too: RollingSync uses each app's own retry or a limit of 5, which a single spec-level
  `retry` can't express. The ApplicationSet controller sets `retry: {limit: 5}` on apps without their own.
- **Operation info.** Add `info`, copied into each app's `operation.info` after the ApplicationSync tag, so the
  Sync API's `infos` and RollingSync's "Reason" survive.
- **Partial syncs.** Add `resources` per app, matching the Sync API's `resources`, so a UI sync of selected
  resources can be expressed.

**When an app counts as done**

- **Synced and Healthy.** The [health gate](#the-applicationsync-resource) requires Synced as well as Healthy,
  as RollingSync does. The prototype only checks that the operation succeeded and the app is Healthy, and needs
  updating.
- **`doneWhen`.** Add `doneWhen: SyncedAndHealthy | Healthy`, defaulting to `SyncedAndHealthy`. With `Healthy`,
  an app that is Healthy but stays OutOfSync still counts as done, for example when resources are skipped with
  `SkipDryRunOnMissingResource` ([#19771](https://github.com/argoproj/argo-cd/issues/19771)).
- **Freshness before deciding.** Before an app is skipped or counted as done, its status must reflect its current
  spec and revisions. Starting a sync doesn't need it, since the sync resolves its own revisions. That holds when all of these are true:
  - `status.reconciledAt` is at or after the time the ApplicationSync was accepted;
  - `status.sync.comparedTo` matches the app's current sources, destination and `ignoreDifferences`, using the
    comparison `needRefreshAppStatus` already uses to force a refresh after a spec change. The prototype adds
    `Application.ComparedToCurrentSpec()` so both controllers share it;
  - `status.sync.revisions` matches the pinned revisions, for sources that are pinned.

  The controller requests a refresh for apps that don't pass. RollingSync does the same with refresh annotations
  after `--refresh-grace-period-seconds` (Open Question 5). Without this check, stale status lets a step finish
  early, or lets `skipIfSynced` skip an app that still needs a sync ([#29410](https://github.com/argoproj/argo-cd/issues/29410), [#27949](https://github.com/argoproj/argo-cd/issues/27949)). A refresh
  that read an old copy of the spec records that old spec in `comparedTo`, so it is rejected whatever its timing.
  `comparedTo` does not cover every spec field, such as the project or sync policy, but those don't change the
  rendered manifests. If a future Application API adds `status.observedGeneration`, the check can use it instead.
- **Waiting instead of failing.** Add `onFailure: Fail | Wait`. `Fail` is the default, described in [The ApplicationSync resource](#the-applicationsync-resource). With `Wait`, an app
  whose sync failed, that is `Degraded`, or that has an `InvalidSpecError` or `UnknownError` condition stays
  `Progressing` until it is Synced and Healthy. That happens through retries or a manual fix, and its
  dependents keep waiting. Nothing is ever `Failed` or `Skipped`. This is how RollingSync behaves today: "it is
  not the appset's job to retry failed syncs".

**Sync windows and identity**

- **Automatic syncs.** Add `initiatedBy.automated`. When it is `true`, the operation is marked automatic, and the
  controller checks sync windows with automatic semantics (`controller/sync.go`, `syncWindowPreventsSync`), as it
  does for RollingSync's operations today. A window that allows only manual syncs blocks it.
- **Closed windows wait without starting an operation.** Before setting an app's operation, the controller
  checks the project's sync windows (`CanSync`, with automatic semantics when `initiatedBy.automated` is set).
  If a window denies the sync, the app stays `Pending` with a message naming the window, no operation is written,
  and it doesn't take a `maxParallel` slot. The app starts once the window allows it. RollingSync today writes the
  operation anyway and lets the sync path park it, which keeps the `maxUpdate` slot occupied ([#29307](https://github.com/argoproj/argo-cd/issues/29307)).
  An operation that already started inside an allowed window is governed by the window's `syncOverrun` setting, as
  it is today. The Sync API keeps rejecting up front (see [API and UI syncs](#api-and-ui-syncs)), so manual API
  syncs behave as they do now.

**Observability**

- **Group status.** Per-group status is part of the core resource. The ApplicationSet controller derives its
  `RolloutProgressing` condition, which names the current step, from the first group that hasn't succeeded.
- **Metrics.** Export ApplicationSync metrics that answer the questions in [#27846](https://github.com/argoproj/argo-cd/issues/27846):
  - how long it took from creating the run to starting the first sync;
  - how long each group took;
  - how many refreshes the freshness check triggered;
  - how many syncs were started;
  - how many runs are in each phase.

  The ApplicationSet controller keeps `argocd_appset_progressive_sync_app_status` and
  `argocd_appset_progressive_sync_syncs_triggered_total`, fed from the ApplicationSync status.
- **UI.** Each Application shows its recent ApplicationSyncs as sync history. An ApplicationSync view shows its
  groups in dependency order, the apps in each, and which are running. The ApplicationSet view reads rollout
  progress from its current ApplicationSync. That covers most of [#27879](https://github.com/argoproj/argo-cd/issues/27879): ordering apps by step,
  highlighting the step in progress, and giving the rollout its own detail section.
- **Logs.** The controller logs when a group or app changes phase, not on every reconcile ([#18240](https://github.com/argoproj/argo-cd/issues/18240)).

#### Revisions

Each app can pin the revision of each of its sources:

```yaml
groups:
- name: platform
  apps:
  - name: guestbook        # single source: one entry, with no source or position
    revisions:
    - revision: v1.4.0
  - name: monitoring       # three sources
    revisions:
    - source: chart        # matches spec.sources[].name
      revision: 45.7.1
    - position: 2          # 1-based index into spec.sources, as in the Sync API's sourcePositions
      revision: main
    # the third source is not listed, so it is not pinned
```

**Which source an entry applies to**

- **Single-source apps** (`spec.source`) take at most one entry, with neither `source` nor `position`.
- **Multi-source apps** name each source either by `source`, its `spec.sources[].name`, or by `position`, its
  1-based index. An entry has exactly one of the two. `source` is preferred, because it still points at the right
  source if `spec.sources` is reordered.
- **Matching happens when the app is first seen,** because that is when listed revisions are resolved. The app
  fails, with a message naming the entry, if an entry matches no source, if two entries match the same source,
  or if a single-source app's entry has a `source` or `position`. When the sync starts, the entries are matched
  again, and the app fails if its sources changed in a way that moves a pin to a different source.
- **Ref sources**, which only provide values files through `ref:`, are pinned like any other source. Pinning one
  pins the version of the values files it provides.

**When each revision is decided**

| Source | Resolved to a concrete revision | Meaning |
|---|---|---|
| Listed | Once, when the ApplicationSync is accepted, or for an app created later, when the controller first sees it. Recorded in status. | `revision: main` means `main` at that moment, for the rest of the ApplicationSync. |
| Not listed | When the app's sync starts, from the source's `targetRevision`. | The latest revision at the time the app syncs. |

- **Concrete revisions.** "Concrete" means a commit SHA for Git, an exact version for a Helm chart, and a digest
  for OCI. Resolution uses the repo server's `ResolveRevision`, the same call the Sync API makes. A revision that
  can't be resolved fails the app; with `onFailure: Wait` the controller keeps retrying.
- **Status.** The resolved values are recorded in `status.groups[].apps[].pinnedRevisions`, one per source in
  `spec.sources` order, with an empty entry for each source that isn't listed. After the sync, `revisions` records
  what each source actually synced to.
- **The operation.** For a single source, the pinned revision goes into `operation.sync.revision`. For multiple
  sources, it goes into `operation.sync.revisions`, one entry per source in `spec.sources` order, empty for
  sources that aren't listed. The controller already treats an empty entry as "use `targetRevision`"
  (`controller/state.go`).
- **Unlisted sources float on purpose.** An app queued behind a long-running group should sync to the latest
  revision, not one from when the run was accepted. To pin every source to what it resolves to right now, list
  every source, for example with its branch name.
- **Who writes concrete revisions.** [API and UI syncs](#api-and-ui-syncs) keep today's behavior: `argocd-server` resolves
  every source, listed or not, when the request arrives and writes the concrete revisions into the
  ApplicationSync. The ApplicationSet controller and policies also write a concrete revision
  for every source, as RollingSync pins every source today.

**Pinning a revision other than `targetRevision`**

- **What it does.** The app syncs to that revision, like `argocd app sync --revision` today. Afterwards the app
  reports OutOfSync against its `targetRevision`, as it does today.
- **Auto-sync.** No conflict arises: apps in an ApplicationSync never have auto-sync enabled (see
  [The ApplicationSync resource](#the-applicationsync-resource)).
- **When it counts as done.** Synced is judged against the pinned revisions. An app is done when all of these
  hold:
  - its sync to the pinned revisions succeeded;
  - it has been refreshed since;
  - it is Healthy;
  - it is Synced, if `status.sync.revisions` still compares against the pinned revisions.

  If `targetRevision` moved past a pinned revision during the run, the app still counts as done while OutOfSync.
  Whoever created the ApplicationSync decides whether to start a new run; the ApplicationSet controller does, as
  RollingSync does today.
- **Who may do it.** Whether kubectl users may pin a revision other than `targetRevision` is Open Question 1.
- **Hydrated apps.** Apps that use `spec.sourceHydrator` are not supported by this proposal. Any entry fails the app.

### API and UI syncs

`argocd-server`'s Sync handler keeps every check it has today: Argo CD RBAC, sync windows, auto-sync and
revision conflicts, and the replace setting. Once the request is allowed, the handler creates a single-app
ApplicationSync instead of writing `operation`. It is opt-in during rollout, through `--sync-with-applicationsync`
(`server.sync.applicationsync.enabled` in `argocd-cmd-params-cm`):

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSync
metadata:
  generateName: guestbook-
  namespace: argocd
spec:
  initiatedBy:
    username: alice@example.com  # the Argo CD user; informational only
  info:                          # the request's infos
  - {name: Reason, value: release}
  prune: true
  groups:
  - name: guestbook              # the server names the single group after the app
    apps:
    - name: guestbook
      revisions:
      - revision: 4f1c2e9d0a7b   # resolved by the server from targetRevision when the request arrived
      resources:                 # only for a partial sync
      - {group: apps, kind: Deployment, name: guestbook-ui}
```

- **Clients don't change.** The API still returns the Application. The CLI and UI keep watching
  `status.operationState`, which the controller still fills in. To keep that true, the handler waits until the
  application controller has set the tagged operation before it returns, so a client never sees the previous
  operation's result as this sync's. If the controller doesn't start the sync within 30 seconds, the request
  fails and says the ApplicationSync is still pending.
- **Revision overrides keep writing `operation`.** With `application.sync.requireOverridePrivilegeForRevisionSync`
  on, the controller only accepts pins that match `targetRevision` (Open Question 1). A user with the `override`
  privilege who syncs to another revision has passed a check the controller can't repeat, so for that request the
  server writes `operation` directly, as today.
- **Identity.** `spec.initiatedBy.username` is copied into `operation.initiatedBy.username`, so history and
  events show the Argo CD user as today. Anyone who can create ApplicationSyncs directly can write any value
  there, so it is informational and never used for authorization.
- **What stays on `operation`.** Syncs with local manifests (`argocd app sync --local`) don't belong in a CR and
  keep writing `operation` directly. So do rollbacks, which override the source, and auto-sync. So does a manual
  sync of an app that has auto-sync enabled, a common action in the UI: ApplicationSyncs don't take auto-synced
  apps, so for that app the server writes `operation` itself, as today.
- **Permissions.** `argocd-server`'s Role gains `create`, `get` and `list` on `applicationsyncs`.
- **History.** Each manual sync is now an object, and garbage collection keeps 20 per app.

### ApplicationSyncPolicy

A long-lived resource that creates ApplicationSyncs. In effect it is auto-sync for a group of apps:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSyncPolicy
metadata:
  name: platform
  namespace: argocd
spec:
  # In a later iteration we could add schedule, where we would be able to set an interval with cron syntax. For now we 
  # stick to the global `timeout.reconciliation` interval
  # schedule: 0 * * * *
  concurrencyPolicy: Forbid  # Forbid | Replace (delete the running ApplicationSync, start a new one)
  template:                  # an ApplicationSync spec
    skipIfSynced: true
    groups:
    - name: data
      apps: [{name: db}]
    - name: backend
      dependsOn: [data]
      apps: [{name: api}]
```

- **Same period as auto-sync.** The policy is evaluated whenever one of its apps is refreshed, which is on the
  global `timeout.reconciliation` period (default `120s`, plus `timeout.reconciliation.jitter`) or when a
  webhook or a manual refresh arrives. If any listed app is OutOfSync, a new ApplicationSync is created,
  subject to `concurrencyPolicy`. A policy has no period of its own.
- **Auto-sync belongs to the group.** Apps managed by a policy must have `syncPolicy.automated` disabled, as for any
  ApplicationSync and as RollingSync requires today. The policy decides when they sync.
- **Apps from anywhere.** A policy can list apps from several ApplicationSets, or from none, which covers a shared
  rollout strategy across ApplicationSets ([#14458](https://github.com/argoproj/argo-cd/issues/14458)). Until Open Question 3 is settled, apps are listed
  by name.
- **History.** Runs are ordinary ApplicationSyncs, labelled with the policy name and owned by the policy.
  Garbage collection per set of apps already bounds how many are kept.
- **Authorization.** Creating a policy is governed by Kubernetes RBAC on `applicationsyncpolicies`. A policy
  decides when its apps sync, so granting `create` on it is comparable to granting `update` on those
  Applications.
- **Runs are automatic syncs.** A run the policy creates sets `initiatedBy.automated: true` and is checked
  against sync windows as automatic (`CanSync(false)`), the same as auto-sync. So a window that allows only
  manual syncs does not let policy runs through. An app whose window is closed waits for it to open rather
  than failing the run, just as auto-sync waits.

> [!NOTE]
> **Future work: explicit schedules.** A per-policy cron schedule, for example `schedule: "0 2 * * *"` to sync
> a group every night, would be a great addition, and is left out on purpose to keep the policy small. It would
> add a second trigger next to drift, and sync windows would still apply to it.

### RollingSync on ApplicationSync

The ApplicationSet API doesn't change. What changes is who runs the rollout:

| Stays in the ApplicationSet controller | Moves to ApplicationSync |
|---|---|
| Generating Applications and forcing auto-sync off on them | Setting `operation` on each app |
| Detecting a changed target revision, spec, or set of apps | Refreshing apps before deciding anything |
| Matching apps to steps, and validating the steps (`InvalidRolloutConfig`) | Waiting for each step to be Synced and Healthy |
| Turning `steps` into groups, and `maxUpdate` into `maxParallel` | Enforcing `maxParallel`, including `0` as a pause |
| Copying each app's `automated.prune`, and the retry default of 5 | Pinning each app's revisions for the whole rollout |
| Deleting apps in reverse order (`deletionOrder: Reverse`) | Holding failed or `Degraded` apps until they recover |
| Showing progress (`status.applicationStatus`, `RolloutProgressing`, metrics) | Recording each rollout as an object |

That removes the sync-triggering and step-tracking parts of `applicationset/progressivesync`. The new engine is
opt-in during rollout, through `--progressive-syncs-with-applicationsync`
(`applicationsetcontroller.progressive.syncs.applicationsync`), next to the existing `--enable-progressive-syncs`.
The ApplicationSet controller also:

- **writes the generated Application specs before it creates the run**, so the run's freshness check sees the
  new spec ([#27949](https://github.com/argoproj/argo-cd/issues/27949));
- **starts a run only for changes its `applicationsSync` policy lets it apply**. A `create-only` or
  `create-delete` policy never rewrites existing Applications, so a template change can't start a rollout for
  them ([#29142](https://github.com/argoproj/argo-cd/issues/29142));
- **leaves out apps annotated with `argocd.argoproj.io/skip-rollout: "true"`**, so a single app can be kept out
  of rollouts temporarily ([#22001](https://github.com/argoproj/argo-cd/issues/22001));
- **keeps reporting apps that match no step** in an ApplicationSet condition, as added in [#27820](https://github.com/argoproj/argo-cd/pull/27820).

#### What counts as a change

A rollout needs a reliable answer to "which change is being rolled out", even though apps can use many sources
across many repositories. With ApplicationSync, the run is that answer: the ApplicationSet controller pins a
concrete revision for every source of every app when it creates the run, and status records exactly what each
app synced to.

- **One repository and revision for all apps.** One commit is one run. Every app syncs to that commit.
- **Several shared sources (multi-source).** One set of source revisions is one run.
- **Different repositories.** There is no time window that groups commits in different repositories into one
  run. Each run pins what every source resolved to when it was created. A commit that lands afterwards starts
  the next run: it replaces the current one, or with `concurrencyPolicy: Forbid` in a policy, waits for it to
  finish. The run's status shows exactly which combination was rolled out.
- **Template and parameter changes.** The ApplicationSet controller writes the new specs first and then creates
  the run. The freshness check makes sure every app's status reflects the new spec before anything is skipped or
  counted as done.

The docs should spell out these patterns and which of them are reliable, as asked in [#28927](https://github.com/argoproj/argo-cd/issues/28927).

#### Example flow

An ApplicationSet with a two-step RollingSync, written as today:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
  namespace: argocd
spec:
  generators:
  - list:
      elements:
      - {cluster: dev, env: env-dev}
      - {cluster: prod-us, env: env-prod}
      - {cluster: prod-eu, env: env-prod}
  strategy:
    type: RollingSync
    rollingSync:
      steps:
      - matchExpressions:
        - {key: envLabel, operator: In, values: [env-dev]}
      - matchExpressions:
        - {key: envLabel, operator: In, values: [env-prod]}
        maxUpdate: 50%
  template:
    metadata:
      name: 'guestbook-{{cluster}}'
      labels:
        envLabel: '{{env}}'
    spec:
      project: default
      source:
        repoURL: https://github.com/argoproj/argocd-example-apps.git
        targetRevision: HEAD
        path: guestbook
      destination:
        name: '{{cluster}}'
        namespace: guestbook
```

1. **A commit lands.** Commit `4f1c2e9` is pushed to `HEAD`. All three Applications go OutOfSync. Auto-sync is off
   on them, as RollingSync requires today, so nothing syncs yet.
2. **The ApplicationSet controller starts a rollout.** It notices the new target revision, using the same
   detection RollingSync uses today, and creates an owned ApplicationSync. Each app is pinned to its own
   resolved revision; they match here only because all three apps share one source:

   ```yaml
   apiVersion: argoproj.io/v1alpha1
   kind: ApplicationSync
   metadata:
     generateName: guestbook-
     namespace: argocd
     labels:
       argocd.argoproj.io/applicationset: guestbook
     ownerReferences:
     - {apiVersion: argoproj.io/v1alpha1, kind: ApplicationSet, name: guestbook, uid: <uid>, controller: true}
   spec:
     initiatedBy: {username: applicationset-controller, automated: true}
     onFailure: Wait
     skipIfSynced: true
     retry: {limit: 5}
     groups:
     - name: step-1
       apps:
       - {name: guestbook-dev, revisions: [{revision: 4f1c2e9}]}
     - name: step-2
       dependsOn: [step-1]
       maxParallel: 50%
       apps:
       - {name: guestbook-prod-us, revisions: [{revision: 4f1c2e9}]}
       - {name: guestbook-prod-eu, revisions: [{revision: 4f1c2e9}]}
   ```

3. **The rollout runs.** The ApplicationSet shows the same statuses as today, translated from the
   ApplicationSync. `Pending` becomes `Waiting`. `Running` becomes `Pending` until the operation starts, then
   `Progressing`. `Progressing` stays `Progressing`, and `Succeeded` becomes `Healthy`. With `onFailure: Wait`
   no app is ever `Failed` or `Skipped`, so every ApplicationSync phase has a RollingSync equivalent:

   | Time | `guestbook-dev` | `guestbook-prod-us` | `guestbook-prod-eu` |
   |---|---|---|---|
   | t0 | Running, operation not started yet (`Pending`) | Pending (`Waiting`) | Pending (`Waiting`) |
   | t1 | Progressing (`Progressing`) | Pending (`Waiting`) | Pending (`Waiting`) |
   | t2 | Succeeded (`Healthy`) | Running (`Progressing`) | Pending (`Waiting`), held by `maxParallel: 50%` |
   | t3 | Succeeded (`Healthy`) | Succeeded (`Healthy`) | Running (`Progressing`) |
   | t4 | Succeeded (`Healthy`) | Succeeded (`Healthy`) | Succeeded (`Healthy`) |

   The ApplicationSync ends as `Succeeded` and stays as the record of this rollout.
4. **A new commit lands mid-rollout.** Say commit `9a8b7c6` arrives at t2. The controller deletes the running
   ApplicationSync; the sync already running on `guestbook-prod-us` finishes, and nothing new starts. It then
   creates a new ApplicationSync pinned to `9a8b7c6`, starting again from `step-1`. That matches RollingSync
   today, which puts apps back to `Waiting` when their target revision changes. `skipIfSynced` lets apps that
   are already Synced and Healthy pass straight through, as RollingSync moves them straight to `Healthy`. A
   change to the generated spec, such as a new generator parameter, starts a new rollout the same way. So does a
   change to the set of apps: RollingSync matches apps to steps on every reconcile, so a newly generated app
   joins its step. Here the controller replaces the run with one that includes the new app.
5. **Sync windows use automatic semantics, as today.** RollingSync's operations are marked
   `initiatedBy.automated: true`, and so are these ApplicationSyncs. One thing changes: a closed window now keeps
   the app `Pending` without writing an operation, instead of writing one and leaving it parked in a
   `maxUpdate` slot ([#29307](https://github.com/argoproj/argo-cd/issues/29307); see [Rollout controls](#rollout-controls)). The RollingSync docs say
   windows apply "as if a user had clicked Sync". That is not accurate for windows with `manualSync` enabled, and
   should be corrected separately.
6. **A failure holds the rollout.** If `guestbook-prod-us` fails to sync or becomes `Degraded`, it stays
   `Progressing` and `step-2` doesn't finish until it recovers, through retries or a manual fix. That is
   today's behavior.

#### Known RollingSync issues

How the open `feature:progressive-sync` issues listed from [#28927](https://github.com/argoproj/argo-cd/issues/28927) fare under this design:

| Issue | Addressed by |
|---|---|
| [#29307](https://github.com/argoproj/argo-cd/issues/29307) operations started while a window denies sync | Windows are checked before an operation is written; see [Rollout controls](#rollout-controls). |
| [#29410](https://github.com/argoproj/argo-cd/issues/29410) step completion judged on stale status | The freshness check, and groups advancing only from the ApplicationSync's own recorded state. |
| [#29347](https://github.com/argoproj/argo-cd/issues/29347) steps out of order when refreshes arrive out of order | Order comes from the ApplicationSync's own state, not from app status. A later group can't start before the groups it depends on have succeeded. |
| [#29280](https://github.com/argoproj/argo-cd/issues/29280) stale merge-patch leaves a partial `operation` | Operations are written with `SetAppOperation`, a full `Update` with conflict detection. |
| [#27949](https://github.com/argoproj/argo-cd/issues/27949) spec-only change reported `Healthy` before it was applied | Specs are written before the run is created, and the freshness check compares `status.sync.comparedTo` with the spec. |
| [#21502](https://github.com/argoproj/argo-cd/issues/21502) extra sync right after a sync | Each app's operation is tagged and started exactly once. |
| [#19771](https://github.com/argoproj/argo-cd/issues/19771) blocked on Healthy-but-OutOfSync apps | `doneWhen: Healthy`. |
| [#29142](https://github.com/argoproj/argo-cd/issues/29142) create-only policies never settle | Runs start only for changes the ApplicationSet's policy applies. |
| [#15371](https://github.com/argoproj/argo-cd/issues/15371) empty apps never progress | Probably fixed, because ApplicationSync syncs directly rather than through auto-sync's `allowEmpty`. Needs an e2e test to confirm. |
| [#22001](https://github.com/argoproj/argo-cd/issues/22001) skip specific apps | The `argocd.argoproj.io/skip-rollout` annotation. |
| [#25741](https://github.com/argoproj/argo-cd/issues/25741) history and rollback | ApplicationSyncs are the history; rollback re-runs recorded revisions. |
| [#14458](https://github.com/argoproj/argo-cd/issues/14458) one strategy across ApplicationSets | An ApplicationSyncPolicy can list apps from several ApplicationSets. |
| [#14724](https://github.com/argoproj/argo-cd/issues/14724) globs in steps | Open Question 3. |
| [#27846](https://github.com/argoproj/argo-cd/issues/27846) metrics | Required metrics are listed under Observability. |
| [#27879](https://github.com/argoproj/argo-cd/issues/27879) UI | ApplicationSync views; see Observability. |
| [#18240](https://github.com/argoproj/argo-cd/issues/18240) log spam | Logging only on phase changes. |
| [#11932](https://github.com/argoproj/argo-cd/issues/11932) rollout analysis | Out of scope; see Non-Goals. |

### Implementation details

All five parts are prototyped on the `feat/applicationsync` branch, with unit tests, e2e tests and docs. See
[Validation](#validation) for what that proved and what it didn't.

- **Names.** The kinds are `ApplicationSync` and `ApplicationSyncPolicy` because `Sync` alone would clash with
  existing `v1alpha1` type names (`SyncStatus`, `SyncPolicy`, ...).
- **Reconciler.** An ApplicationSync informer runs in the application controller, indexed by the apps each
  ApplicationSync references. A change to an app wakes up the ApplicationSyncs that list it.
- **Sharding.** The shard that owns the first app of the first group owns the ApplicationSync. It writes `app.operation`
  through the API, and the shard that owns each app still runs that app's sync. Every shard's informer already
  sees all Applications. Progress is kept in the ApplicationSync's status, so if the first app moves to another
  shard during a run, the new owner carries on.
- **Matching results.** The operation is tagged with an `Info` entry `ApplicationSync: <name>`. Its result is
  read back from `status.operationState` when that entry matches and `startedAt` is not before the
  ApplicationSync was created.
- **Crash safety.** If setting the operation succeeds but the status update doesn't, the next pass finds the
  tagged operation and continues. It never starts a second sync. If the informer hasn't caught up with an
  operation that was just set, the controller reads the Application live before deciding the operation was
  lost.
- **Guards.** The controller enforces sync windows, the project, and `server.sync.replace.allowed`. The env var
  for the latter is now also wired into the controller.
- **Shared tag.** The `Info` name the controller writes is the constant `ApplicationSyncOperationInfoName`, because
  `argocd-server` also reads it to detect the handoff.
- **Policies don't retry a change.** A policy records, on each run, which revisions and sources it rolled out, and
  doesn't start another run for the same change. Otherwise an app that keeps failing would start a new run on
  every reconciliation, just as auto-sync would without its "already attempted this revision" check.

#### Validation

The prototype was checked in three ways:

- **Unit tests** for every control in this proposal, the policy, the Sync API path and the ApplicationSet engine.
  The existing RollingSync unit tests, which run on the old engine, still pass.
- **CRD behavior on a real API server.** On a k3s 1.35 cluster: `spec.paused` can be toggled, every other change
  is rejected (including setting a field that was absent), duplicate group and app names are rejected, and a
  revision with both `source` and `position` is rejected.
- **e2e on a local stack**, with all three flags on. The new tests for group ordering, the policy, and API syncs
  pass, and so do the six existing RollingSync e2e tests, now running on the ApplicationSync engine. That run
  created 16 rollouts and started 37 syncs through ApplicationSyncs. One RollingSync test,
  `TestProgressiveSyncMultipleAppsPerStepWithReverseDeletionOrder`, failed once and passed on two re-runs. Its
  finalizer-removal helper races the application controller removing `resources-finalizer`, and reverse deletion
  isn't part of this change.

Things the prototype found that the design didn't cover are folded into the sections above: the override
fallback for API syncs, the handoff wait, per-app `retry`, `info`, when revisions are matched, and why a policy
must not re-run a change.

Not yet validated:

- **Migrating a rollout that's in flight** from the old engine to the new one.
- **Metrics and UI.** The prototype has neither.
- **Rollout churn.** In one e2e test, the ApplicationSet replaced its rollout three times within a few seconds
  while its generated Applications settled. This is harmless, because syncs are idempotent and `skipIfSynced`
  passes apps that are already done, but it adds history entries. Waiting briefly before starting a run may be
  worth it.
- **Scale.** Nothing was tested with many apps or many shards.

### Security Considerations

- **Kubernetes API access bypasses Argo CD RBAC, as today.** Anyone who can create `applicationsyncs` in a
  namespace can sync every Application in it. That matches today's kubectl path, which bypasses Argo CD RBAC
  through `patch applications`, and it is narrower, because it can't edit the Application. The docs must make
  this explicit.
- **API and UI syncs are unchanged.** `argocd-server` enforces Argo CD RBAC before creating the object.
- **The recorded identity is not proof.** `spec.initiatedBy` can be written by anyone who can create the object,
  so it is shown in history but never used to authorize anything.
- **Fields are restricted.** Local manifests and source overrides are not exposed. Revision pinning is covered by
  Open Question 1.
- **Project limits still apply.** Sync windows, destinations and source integrity are checked by the existing
  sync path.

### Risks and Mitigations

- **Over-broad grants.** Administrators might grant `create` on `applicationsyncs` without realizing it means
  sync. Mitigation: document it, and don't include it in any aggregated or default roles.
- **Apps that still auto-sync.** Users moving existing apps into ApplicationSyncs may leave auto-sync on. Mitigation:
  the app fails, or waits with `onFailure: Wait`, with a message naming the setting to change.
- **Object churn.** API and UI syncs create one object per manual sync. Mitigation: garbage collection keeps 20 per app.
- **Status size.** Status grows with the number of apps. Mitigation: add `maxItems` limits on `spec.groups`
  and on the apps in each group (the prototype has none), and keep the per-app status small (phase, message, revisions, timestamps).
- **Apps that never become Synced.** An app only counts as done when it is Synced and Healthy. An
  app with a diff that never goes away blocks its dependents, as it blocks a RollingSync step today, and with
  `onFailure: Wait` it blocks indefinitely. Mitigation: show the reason in the app's
  `status.groups[].apps[].message`, point to `ignoreDifferences`, and offer `doneWhen: Healthy`.
- **Competing ApplicationSyncs.** A hand-written ApplicationSync can list apps that a RollingSync rollout also
  manages. Queueing keeps two operations from running at once, but the two could still interleave. Mitigation:
  document that apps managed by RollingSync must not be listed in other ApplicationSyncs.

### Upgrade / Downgrade Strategy

- **Upgrade.** The new CRD is purely additive, and nothing changes unless ApplicationSyncs are created. The
  controller needs `get/list/watch/update` on `applicationsyncs` and `applicationsyncs/status`, and those are
  added to its Role. `argocd-server` needs `create/get/list` on `applicationsyncs`.
- **RollingSync migration.** `--enable-progressive-syncs` and `strategy.type: RollingSync` keep working
  unchanged. A rollout that is under way at upgrade time is picked up from `status.applicationStatus`. The first
  ApplicationSync keeps the recorded `TargetRevisions` as its pinned revisions, one `position` entry per source, and
  uses `skipIfSynced`, so:
  - apps that are already `Healthy` pass straight through;
  - apps in `Progressing` that are already Synced only wait for health;
  - apps in `Pending` wait for the operation the old code set, then are skipped if it left them Synced.

  No app is synced twice for the same revision.
- **Downgrade.** The CRD and its objects stay, but nothing acts on them. Operations already running on
  Applications finish normally. An older `argocd-server`
  writes `operation` directly again.

## Drawbacks

- One more CRD, and one more way to trigger a sync.
- Syncs through the Kubernetes API are authorized by Kubernetes RBAC, not by Argo CD RBAC.
- Every manual sync creates an object.
- It changes how RollingSync works internally, so upgrades must migrate rollouts that are in flight (see
  Upgrade / Downgrade Strategy).

## Alternatives

- **Status quo.** Keep writing `operation` on the Application, and use app-of-apps sync waves for ordering. That
  mutates managed objects and needs a parent app plus the Application health check.
- **Enforce Argo CD RBAC on kubectl-created ApplicationSyncs.** A mutating admission webhook would record the
  creator's Kubernetes identity in the immutable spec, and the controller would check Argo CD RBAC against it
  before starting each app. Deferred because it needs:
  - a mapping from Kubernetes users and groups to Argo CD subjects;
  - a way to trust the recorded identity if the webhook is removed;
  - webhook availability and certificates;
  - care with server-side apply, because changes a mutating webhook makes are credited to the requester's
    field manager.

  It can be added later without changing the API beyond `spec.initiatedBy`.
- **Extend RollingSync.** It stays bound to one ApplicationSet and label selectors, and still has no record of
  each run.
- **External orchestration.** Argo Workflows, or CI calling the Argo CD API. That works today, but needs API
  tokens, and the ordering and health-gating logic lives outside Argo CD.
- **A list of operations on the Application.** Queue several operations on the Application instead of one. That
  still mutates the Application and doesn't address ordering across apps.
