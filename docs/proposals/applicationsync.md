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
last-updated: 2026-10-21
---

# ApplicationSync

A namespaced `ApplicationSync` resource requests a one-off sync of one or more Applications, ordered by dependencies
and gated on health. An `ApplicationSyncPolicy` creates ApplicationSyncs whenever its apps drift, which makes it
auto-sync for a group. ApplicationSet RollingSync could later be rebuilt on top of ApplicationSync (see
[Future work](#future-work)).

## Open Questions

1. **Revision pins through kubectl.** `application.sync.requireOverridePrivilegeForRevisionSync` can't be enforced
   on ApplicationSyncs created through the Kubernetes API (see [Authorization](#authorization)). Should those still
   be able to pin any revision, as writing `operation` allows today? While the setting is on, the prototype only
   accepts a pin that names `targetRevision` or the revision the app last resolved it to.
2. **Resolving hand-written branch pins.** Policy runs are already fully pinned (see
   [Revisions](#revisions)). That leaves pins written through kubectl or CI: a branch or tag pin only keeps every
   app on the same commit if it is resolved once, when the ApplicationSync is accepted. This proposal leans towards
   the controller doing that through the repo server, as the Sync API does, which needs repo-server access and read
   access to repository credentials; the prototype has neither and passes such pins as given. The alternative is to
   document that they float, so users who need every app on one commit pin a SHA.
3. **Selectors in groups.** Should a group accept a label selector, as RollingSync steps do, or list apps by name
   only, with the ApplicationSet controller resolving selectors? This also decides where globs
   ([#14724](https://github.com/argoproj/argo-cd/issues/14724)) and policies spanning several ApplicationSets
   ([#14458](https://github.com/argoproj/argo-cd/issues/14458)) are handled.
4. **A deadline for `onFailure: Wait`.** With `Wait`, an app that can't recover holds its dependents forever. A pin
   to a bad revision never recovers, because only Synced and Healthy at the pinned revisions ends the wait. Should
   apps get a deadline, like a Deployment's `progressDeadlineSeconds`, after which a waiting app fails? RollingSync,
   if rebuilt on ApplicationSync, would leave it unset, since a new commit already replaces its run.

## Summary

Today a sync is requested through the API, CLI or UI, or by writing the `operation` field on the Application
([Sync Applications with Kubectl](../user-guide/sync-kubectl.md)). Each request covers one app and mutates the
Application, and no record of it survives once the next operation replaces `status.operationState`.

`ApplicationSync` is a separate resource whose spec is fixed once created, apart from `paused`. It lists Applications
in groups, and a group can depend on other groups. The controller syncs a group's apps in parallel once the groups
it depends on have synced and are Healthy, and reports progress per group and per app. Completed ApplicationSyncs stay
as history until garbage collection removes them.

## Motivation

- **GitOps-friendly sync requests.** Patching `operation` onto an Application that is itself managed from Git,
  by an app-of-apps or an ApplicationSet, is an out-of-band edit to a managed object. A separate object can be
  applied, committed or templated without touching the Application.
- **Ordering across apps.** The options today are ApplicationSet RollingSync, which is tied to one
  ApplicationSet and its label selectors, or app-of-apps with sync waves. The latter needs the Application
  health check restored (see [upgrading to 1.8](../operator-manual/upgrading/1.7-1.8.md)) and a parent Application.
- **Moving progressive sync out of the ApplicationSet.** In the discussion on
  [#28927](https://github.com/argoproj/argo-cd/issues/28927) about promoting Progressive Sync to stable, maintainers
  supported eventually giving it its own CRD and controller, as an independent proposal. This proposal provides the
  engine it would run on (see [Future work](#future-work)). Several open RollingSync bugs come from the
  ApplicationSet controller rebuilding a sync state machine out of Application status fields.
- **History and group auto-sync.** No object records a sync request. Auto-sync also works one app at a time,
  with no way to auto-sync a group of apps in dependency order.

### Goals

- Sync groups of Applications with `kubectl apply`, ordered by group dependencies and gated on health.
- One immutable object per request, with per-app status, kept as bounded history.
- Enforce the same guards as an API sync: sync windows, project permissions and `server.sync.replace.allowed`.
- Work with sharded application controllers.
- Provide the building block that group auto-sync, and later RollingSync, are built on.

### Non-Goals

- Removing `operation` from the Application. It stays as the mechanism the controller uses. Auto-sync,
  rollbacks and syncs with local manifests keep writing it directly.
- Enforcing Argo CD RBAC on ApplicationSyncs created through the Kubernetes API. Like writing `operation` today,
  access through the Kubernetes API is governed by Kubernetes RBAC (see [Alternatives](#alternatives)).
- Syncing Applications across namespaces from one `ApplicationSync`.
- A general workflow engine: no arbitrary steps, approvals or hooks between apps.
- Rollout analysis beyond health, such as metric checks or analysis runs
  ([#11932](https://github.com/argoproj/argo-cd/issues/11932)). This is future work, and a group finishing is the
  natural point to hook it in.
- Rolling back ApplicationSet template changes. Reverting a template is done in Git. Rolling back to earlier
  revisions is covered (see [History and rollback](#history-and-rollback)).

## Proposal

The proposal has three parts. They are designed to ship together, and are described separately only for readability.

| Part | Scope |
|---|---|
| [The ApplicationSync resource](#the-applicationsync-resource) | Groups with `dependsOn`, health gating, garbage collection. |
| [Rollout controls](#rollout-controls) | What ordered rollouts need: a concurrency cap, per-app revisions, failure and window handling, freshness, status and metrics. |
| [ApplicationSyncPolicy](#applicationsyncpolicy) | Auto-sync for a group, on the same period as auto-sync today. |

### Use cases

#### Use case 1: ordered release

As an operator, I apply one manifest that syncs `db`, then `api` and `worker` in parallel, then `web`. Each app
only starts once the apps it depends on are Healthy.

#### Use case 2: roll a release back across apps

As an operator, I find that the last release broke something. The ApplicationSync before it records the revisions
each app synced to, so I create a new ApplicationSync that pins those revisions for the same apps and groups, and
the rollback runs in the same order with the same health gates
([#25741](https://github.com/argoproj/argo-cd/issues/25741)). Today I would roll back each app separately, and
nothing records which revisions belonged together.

#### Use case 3: auto-sync a group

As a platform team, I want a set of apps to auto-sync as a unit in dependency order whenever they drift,
instead of each app auto-syncing on its own. Auto-sync is off on the individual apps.

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
  observedGeneration: 1
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

A single app is a single group with one entry, and the group's name can be left out:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSync
metadata:
  name: guestbook-sync
  namespace: argocd
spec:
  groups:
  - apps: [{name: guestbook}]
```

Semantics:

- **Groups.** `spec.groups` is the only way to list apps. Each group has a unique `name`, its `apps`, and
  optional `dependsOn` naming other groups. `name` defaults to `default`, so it can be left out when there is only
  one group; two unnamed groups are rejected as duplicates. A group starts once every group it depends on has
  succeeded, and its apps sync in parallel. A group succeeds when all its apps have; an empty group counts as
  complete.
- **Per-app phases.** `Pending → Running → Progressing → Succeeded`, or `Failed` / `Skipped`. `Progressing` means
  the sync succeeded and the app is not yet Healthy.
- **Health gate.** An app is done when its sync succeeded and it is Synced and `Healthy`, as in RollingSync, judged
  against its pinned revisions and only once its status is fresh (see [Rollout controls](#rollout-controls)). A
  `Degraded` app fails unless `onFailure: Wait` is set. `doneWhen: Healthy` drops the Synced requirement, and
  `dryRun` skips the gate.
- **Failures.** When an app fails, the rest of its group finishes and the group fails. Every group that depends on
  it, directly or indirectly, is `Skipped`; independent groups keep going, and the ApplicationSync ends `Failed`.
- **Waiting.** An app that doesn't exist yet, or already has an operation running, waits, so one
  `kubectl apply -f dir/` can create the Applications and the ApplicationSync together.
- **Validation.** An unknown group in `dependsOn`, a cycle, or an app in more than one group fails the
  ApplicationSync immediately. The CRD rejects duplicate group names and an empty `groups` list.
- **Immutability.** `spec` is immutable except `spec.paused`. To sync again, create a new ApplicationSync; deleting
  one cancels it: running operations finish and nothing new starts.
- **Observed generation.** ApplicationSync and ApplicationSyncPolicy set `status.observedGeneration`, so a `paused`
  change or a policy edit can be seen to have been picked up.
- **No auto-sync.** Apps in an ApplicationSync must have auto-sync disabled, whoever created it; otherwise auto-sync
  could sync an app ahead of its dependencies or undo a pinned revision. The controller refuses such an app and says
  so in its status. To keep a group in sync automatically, use an [ApplicationSyncPolicy](#applicationsyncpolicy).
- **Garbage collection.** Each app keeps its 20 most recent completed ApplicationSyncs: a run is deleted once it is
  among the newest 20 for none of its apps, so the total is bounded by 20 per app. The limit is
  `applicationsynccontroller.history.limit` in `argocd-cmd-params-cm` (`--history-limit`), and `0` disables deletion.

#### History and rollback

Completed ApplicationSyncs are the sync history of their apps, and each records the concrete revisions it synced in
`status.groups[].apps[].revisions`. Rolling back means creating a new ApplicationSync that pins those revisions
([#25741](https://github.com/argoproj/argo-cd/issues/25741)); a tool or the UI can offer this as "re-run with the
revisions from this run".

### Authorization

ApplicationSyncs are created through the Kubernetes API (`kubectl`, CI, GitOps), so Kubernetes RBAC decides:
`create` on `applicationsyncs` in the apps' namespace. Argo CD RBAC is not consulted, the same as writing
`operation` today. Granting `create` on `applicationsyncs` is therefore equivalent to granting sync on every
Application in that namespace, and the docs must say so plainly. The controller checks sync windows and
`server.sync.replace.allowed` before it writes an operation, and the application controller's sync path still
enforces the project's destinations and source integrity.

### Rollout controls

The core resource covers ordered one-off syncs. Ordered rollouts also need the controls below.
Each matches something RollingSync does today, so RollingSync could later be rebuilt on them:

| Control | RollingSync today |
|---|---|
| `maxParallel` | `maxUpdate` |
| `paused` | `maxUpdate: 0` used as a pause switch |
| `revisions` | `TargetRevisions` recorded per app |
| `skipIfSynced` | A Synced app moves straight to `Healthy`, or `Progressing` if not yet Healthy |
| Per-app `prune` and `retry` | Each app's `automated.prune`, and its own retry or a limit of 5 |
| `info` | The "Reason" operation info |
| `onFailure: Wait` | A failed or `Degraded` app holds its step until it recovers |
| Policy runs count as automatic | Operations are marked automatic for sync windows |
| Freshness check | Refresh annotations before acting |

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSync
metadata:
  name: release-43
  namespace: argocd
spec:
  initiatedBy: {username: release-bot}
  onFailure: Wait              # Fail (default) | Wait
  doneWhen: SyncedAndHealthy   # SyncedAndHealthy (default) | Healthy
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

- **`maxParallel`** on a group caps how many of its apps are `Running` or `Progressing` at once, as a number or a
  percentage. A percentage that rounds down to 0 becomes 1; `0` or `0%` pauses the group.
- **`paused`** is the one spec field that can change. While it is `true`, no new syncs start and running ones
  finish; setting it back to `false` resumes the run where it stopped.

**What gets synced**

- **`revisions`** per app pin each source (see [Revisions](#revisions)), so a commit that lands mid-rollout can't
  reach apps that haven't synced yet.
- **`skipIfSynced`**: an app already Synced at its pinned revisions isn't synced again, but still waits to be
  Healthy.
- **Per-app `prune` and `retry`** override the spec-level values. Otherwise retry and sync options come from each
  app's `syncPolicy`.
- **`info`** is copied into each app's `operation.info`, so a sync can carry notes such as a reason.
- **`resources`** per app expresses a partial sync, as the Sync API's `resources` does.
- **`allowEmpty`**: in an automatic (policy) run with prune on, an app whose resources would all be pruned fails without
  syncing unless `allowEmpty` is `true`; with `onFailure: Wait` it waits instead. Manual runs aren't checked, as
  today. An app with no resources at all passes, unlike with auto-sync
  ([#15371](https://github.com/argoproj/argo-cd/issues/15371)).

**When an app counts as done**

- **`doneWhen`**: `SyncedAndHealthy` (default) or `Healthy`. With `Healthy`, an app that stays OutOfSync still
  counts as done, for example when resources are skipped with `SkipDryRunOnMissingResource`
  ([#19771](https://github.com/argoproj/argo-cd/issues/19771)).
- **Freshness.** An app is only skipped or counted as done once its status reflects its current spec and
  revisions: `status.reconciledAt` is after `status.startedAt`, which the controller sets when it accepts the
  ApplicationSync; `status.sync.comparedTo` matches the app's sources, destination and `ignoreDifferences`; and
  `status.sync.revisions` matches its pinned sources.
  Otherwise the controller requests a refresh straight away. Without this, stale status ends a step early or skips
  an app that still needs a sync ([#29410](https://github.com/argoproj/argo-cd/issues/29410),
  [#27949](https://github.com/argoproj/argo-cd/issues/27949)). RollingSync waits `--refresh-grace-period-seconds`
  first because it refreshes every app in the ApplicationSet; this check only refreshes the stale apps it is
  deciding on.
- **`onFailure`**: `Fail` (default, see [The ApplicationSync resource](#the-applicationsync-resource)) or `Wait`.
  With `Wait`, an app whose sync failed, that is `Degraded`, or that has an `InvalidSpecError` or `UnknownError`
  condition stays `Progressing` until it recovers through retries or a manual fix, and its dependents keep waiting.

**Sync windows**

- **Only policy runs are automatic.** A run counts as automatic when an
  [ApplicationSyncPolicy](#applicationsyncpolicy) is its controller owner, and as manual otherwise; there is no spec
  field for it. Windows then apply as they do today: one that allows only manual syncs blocks policy runs but not
  others. Claiming policy ownership falsely only adds restrictions, so the owner reference needs no further check.
- **A closed window doesn't start an operation.** The controller checks the project's windows before writing an
  operation. A denied app stays `Pending` with a message naming the window and doesn't take a `maxParallel` slot.
  RollingSync today writes the operation anyway, which holds a `maxUpdate` slot
  ([#29307](https://github.com/argoproj/argo-cd/issues/29307)). Once started, an operation follows the window's
  `syncOverrun` setting, as today.

**Observability**

- **Metrics** that answer [#27846](https://github.com/argoproj/argo-cd/issues/27846): time from creation to the
  first sync, time per group, refreshes triggered, syncs started, and runs per phase. The ApplicationSet
  controller's existing progressive-sync metrics stay, fed from ApplicationSync status.
- **Logs** only when a group or app changes phase ([#18240](https://github.com/argoproj/argo-cd/issues/18240)).

#### Revisions

Each app can pin the revision of each of its sources:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSync
metadata:
  name: platform-pins
  namespace: argocd
spec:
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

- A single-source app takes at most one entry, with neither `source` nor `position`.
- A multi-source app names each source by `source`, its `spec.sources[].name`, or by its 1-based `position`, but
  not both. `source` is preferred, because it survives `spec.sources` being reordered.
- Entries are matched when the app is first seen and again when its sync starts. The app fails, with a message
  naming the entry, if an entry matches no source, two entries match the same source, or a pin would move to a
  different source.
- Ref sources are pinned like any other, which pins the values files they provide.

**When each revision is decided**

| Source | Resolved to a concrete revision | Meaning |
|---|---|---|
| Listed | Once, when the ApplicationSync is accepted, or for an app created later, when the controller first sees it. Recorded in status. | `revision: main` means `main` at that moment, for the rest of the ApplicationSync. |
| Not listed | When the app's sync starts, from the source's `targetRevision`. | The latest revision at the time the app syncs. |

- **Concrete revisions** are a commit SHA for Git, an exact version for a Helm chart and a digest for OCI. How a
  hand-written branch or tag pin becomes one is Open Question 2. A revision that can't be resolved fails the app;
  with `onFailure: Wait` it is retried.
- **Status** records the resolved pins in `status.groups[].apps[].pinnedRevisions`, in `spec.sources` order with
  an empty entry for each unlisted source, and what each source actually synced to in `revisions`.
- **Unlisted sources float on purpose**, so an app queued behind a long-running group syncs to the latest revision.
  Listing every source pins all of them. Policies pin every source to the revision the app's latest comparison
  already resolved (`status.sync.revisions`), so policy runs are fully pinned without the controller resolving
  anything.

**Pinning a revision other than `targetRevision`**

- The app syncs to that revision, like `argocd app sync --revision`, and afterwards reports OutOfSync against
  `targetRevision`, as today.
- It counts as done once that sync succeeded, it has been refreshed since, and it is Healthy. Synced is only
  required while `status.sync.revisions` still compares against the pins. If `targetRevision` moved on during the
  run, whoever created the ApplicationSync decides whether to start a new one.
- Whether kubectl users may do this is Open Question 1.
- **Hydrated apps** (`spec.sourceHydrator`) aren't supported; any entry fails the app.

### ApplicationSyncPolicy

A long-lived resource that creates ApplicationSyncs from a template whenever its apps drift. In effect it is
auto-sync for a group of apps:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSyncPolicy
metadata:
  name: platform
  namespace: argocd
spec:
  concurrencyPolicy: Forbid  # Forbid | Replace (delete the running ApplicationSync, start a new one)
  selfHeal: true
  template:                  # an ApplicationSync spec
    skipIfSynced: true
    groups:
    - name: data
      apps: [{name: db}]
    - name: backend
      dependsOn: [data]
      apps: [{name: api}]
```

- **Triggered by drift, on auto-sync's period.** A policy has no period of its own. It is evaluated whenever one
  of its apps is reconciled, on the `timeout.reconciliation` period or after a webhook or manual refresh, and
  creates a run if any listed app is OutOfSync, subject to `concurrencyPolicy`.
- **Each run rolls out one change.** Every source of every app is pinned to the revision its latest comparison
  resolved, unless the template pins it, and the policy starts no second run for the same revisions and sources.
- **Self-heal.** With `selfHeal: true`, the policy does start another run for the same change when the last one
  succeeded and an app has drifted since. That run keeps the same pins, skips Synced apps and syncs only the
  OutOfSync resources of the others, as `automated.selfHeal` does. Runs are spaced by the self-heal backoff
  (`controller.self.heal.backoff.*` in `argocd-cmd-params-cm`) and counted in `status.selfHealAttempts`. A failed
  run is never self-healed, and neither is an app the run left OutOfSync under `doneWhen: Healthy`, which would
  otherwise start a run on every backoff.
- **Auto-sync belongs to the policy.** Its apps must have `syncPolicy.automated` disabled; the policy starts no
  run while one has it on, and names that app in `status.message`. `selfHeal`, `allowEmpty` and `prune` come only
  from the policy and its template.
- **Apps from anywhere.** A policy can list apps from several ApplicationSets, or from none, which covers a shared
  rollout strategy across ApplicationSets ([#14458](https://github.com/argoproj/argo-cd/issues/14458)). Until
  Open Question 3 is settled, apps are listed by name.
- **Runs** are ordinary ApplicationSyncs, labeled with the policy name and owned by it. Being owned by the policy
  makes them automatic for sync windows (see [Rollout controls](#rollout-controls)), and an app whose window is
  closed waits for it to open, as with auto-sync. History garbage collection bounds how many are kept.
- **Deleting a policy deletes its runs,** as deleting a CronJob deletes its Jobs: a run in progress is cancelled, and
  its completed runs go with it. `kubectl delete --cascade=orphan` keeps them as history.
- **Authorization.** Creating a policy is governed by Kubernetes RBAC on `applicationsyncpolicies`. A policy
  decides when its apps sync, so granting `create` on it is comparable to granting `update` on those Applications.

> [!NOTE]
> **Future work: explicit schedules.** A per-policy cron schedule, for example `schedule: "0 2 * * *"` to sync a
> group every night, would add a second trigger next to drift, and sync windows would still apply to it. It is left
> out to keep the policy small.

### Future work

These depend on ApplicationSync moving into Argo CD, and none of them is part of this proposal.

**UI.** Each Application lists its recent ApplicationSyncs, and an ApplicationSync view shows its groups in
dependency order and what is running. With RollingSync on ApplicationSync, the ApplicationSet view would show
progress from its current run, which covers most of [#27879](https://github.com/argoproj/argo-cd/issues/27879).

**API and UI syncs.** `argocd-server`'s Sync handler would keep its checks (Argo CD RBAC, sync windows, auto-sync
and revision conflicts, the replace setting) and then create a single-app ApplicationSync instead of writing
`operation`, with every source resolved and the Argo CD user in `spec.initiatedBy.username`. The handler returns
once the controller has written the operation, so clients don't change. If that times out, the handler deletes the
ApplicationSync and looks for its UID in the Application's `operation` and `status.operationState`: if it is there,
the controller got in first and the request succeeds; otherwise it fails. Because the controller reads the
ApplicationSync live just before writing an operation, a sync can then only start in the short window between that
read and the write; closing it fully needs a cancel-and-acknowledge handshake, to be designed with this change.
Local syncs, rollbacks, syncs of auto-synced apps and `override`d revisions keep writing `operation`. Still open:
whether a request for an app with an operation running is rejected, as today, or queued.

**App auto-sync as a generated policy.** While `spec.syncPolicy.automated` is on, the application controller would
keep a single-app ApplicationSyncPolicy for the app, owned by it, with `selfHeal`, `prune` and `allowEmpty` copied
across. `syncPolicy.automated` stays the user-facing API, and edits to the generated policy are reverted. Auto-sync's
"already attempted this revision" check becomes the policy's same-change check, and the generated policy is the one
policy allowed to manage the app. Scale is the main cost: one policy per auto-synced app and one ApplicationSync per
auto-sync, unless the controller evaluates the policy in memory. An older controller ignores generated policies and
auto-syncs as before.

**RollingSync on ApplicationSync.** RollingSync could be rebuilt on ApplicationSync while keeping the `rollingSync`
API. The ApplicationSet controller would turn `steps` into groups and `maxUpdate` into `maxParallel`, and create a
run per change or generate a policy with `concurrencyPolicy: Replace`, leaving operations, pinning, refreshes and
health gating to ApplicationSync. That addresses most of the open `feature:progressive-sync` issues listed in
[#28927](https://github.com/argoproj/argo-cd/issues/28927), such as steps judged on stale status
([#29410](https://github.com/argoproj/argo-cd/issues/29410)), operations started while a window denies sync
([#29307](https://github.com/argoproj/argo-cd/issues/29307)) and missing history
([#25741](https://github.com/argoproj/argo-cd/issues/25741)). It needs its own design for change detection across
sources, `create-only` and `create-delete` policies ([#29142](https://github.com/argoproj/argo-cd/issues/29142)),
migrating rollouts in flight, and sync windows.

### Implementation details

The reference prototype is a standalone controller, `argo-cd-sync-controller`, that runs next to an existing Argo CD
installation and talks to it only through the Kubernetes API. It implements the ApplicationSync resource, rollout
controls, revisions (pins are passed as given; see Open Question 2) and ApplicationSyncPolicy.

- **Names.** The kinds are `ApplicationSync` and `ApplicationSyncPolicy` because `Sync` alone would clash with
  existing `v1alpha1` type names (`SyncStatus`, `SyncPolicy`, ...).
- **Reconciler.** A dedicated controller with leader election, so each object has one owner and no sharded
  component takes on the extra load. ApplicationSyncs and policies are indexed by the apps they list, so a change
  to an app wakes up the ones that reference it.
- **Writing operations.** The controller sets each app's `operation` as the Sync API does, tagged with `Info`
  entries for the ApplicationSync's name and UID, and reads the result back from `status.operationState` when the
  UID matches. Matching on the UID means a same-named ApplicationSync that was deleted and recreated never adopts
  the old run's operation. The application controller runs the sync as usual.
- **Crash safety.** If setting the operation succeeds but the status update doesn't, the next pass finds the
  tagged operation and carries on; it never starts a second sync. Before writing an operation, the controller reads
  the ApplicationSync live, so a deleted one never starts a sync from a stale cache, and before deciding an
  operation was lost, it reads the Application live.
- **Settings.** It reads `application.sync.requireOverridePrivilegeForRevisionSync` from `argocd-cm`, and
  `server.sync.replace.allowed` and the self-heal backoff from `argocd-cmd-params-cm`, so they behave as they do for
  Argo CD's own syncs.
- **Policies don't retry a change.** A policy records which revisions and sources each run rolled out and starts
  no second run for the same change, except to self-heal after a successful run. Otherwise an app that keeps
  failing would start a run on every reconciliation, as auto-sync would without its "already attempted this
  revision" check.
- **Policies only trust runs they recorded.** Labels, owner references and annotations can be set by anyone who can
  create ApplicationSyncs, so a policy identifies its runs through its own status, which only the controller
  writes: it records a run's name before creating it and its UID after, and ignores any run it didn't record.

### Security Considerations

- **Kubernetes RBAC decides, as for kubectl today.** Anyone who can create `applicationsyncs` in a namespace can
  sync every Application in it. That matches today's kubectl path through `patch applications`, and it is
  narrower, because it can't edit the Application. The docs must say so.
- **The controller's own access.** It can write `operation` on any Application it watches, as the application
  controller can, and reads AppProjects and the Argo CD ConfigMaps. Resolving branch pins (Open Question 2) would
  add read access to repository credentials.
- **The recorded identity is not proof.** `spec.initiatedBy` can be written by anyone who can create the object, so
  it is shown in history but never used to authorize anything.
- **Fields are restricted.** Local manifests and source overrides are not exposed; revision pinning is Open
  Question 1.
- **Project limits still apply.** Sync windows are checked before an operation is written, and destinations and
  source integrity by the existing sync path.

### Risks and Mitigations

- **Over-broad grants.** Administrators might grant `create` on `applicationsyncs` without realizing it means sync.
  Mitigation: document it, and don't include it in any aggregated or default roles.
- **Apps that still auto-sync.** Users moving existing apps into ApplicationSyncs may leave auto-sync on.
  Mitigation: the app fails, or waits with `onFailure: Wait`, with a message naming the setting to change.
- **Status size.** Status grows with the number of apps. Mitigation: `maxItems` limits on `spec.groups` and on the
  apps in each group, and a small per-app status (phase, message, revisions, timestamps).
- **Apps that never become Synced.** An app with a diff that never goes away blocks its dependents, as it blocks a
  RollingSync step today, and with `onFailure: Wait` it blocks indefinitely (Open Question 4). Mitigation: show the
  reason in the app's status, point to `ignoreDifferences`, and offer `doneWhen: Healthy`.
- **Competing ApplicationSyncs.** Two ApplicationSyncs, or a hand-written one and a policy's run, can list the same
  app. Queueing keeps two operations from running at once, but the runs could still interleave. Mitigation:
  document that apps managed by a policy must not be listed in other ApplicationSyncs.

### Upgrade / Downgrade Strategy

- **Upgrade.** The CRDs and the controller are additive: nothing changes until ApplicationSyncs or policies are
  created. The controller runs as its own Deployment with leader election, in the same namespace as the application
  controller. AppProjects and Argo CD's ConfigMaps are read from that namespace, so that access is a namespaced
  Role. Applications and ApplicationSyncs need cluster-wide access only when apps live in other namespaces, as with
  `application.namespaces` today.

  | Resources | Verbs | Scope | Why |
  |---|---|---|---|
  | `applications` | `get/list/watch/update/patch` | App namespaces | Write `operation` and request refreshes |
  | `applicationsyncs` | `create/delete/get/list/watch` | App namespaces | Policies create runs, and garbage collection deletes them |
  | `applicationsyncs/status`, `applicationsyncpolicies/status` | `get/update/patch` | App namespaces | Report progress |
  | `applicationsyncpolicies` | `get/list/watch` | App namespaces | Read policies |
  | `appprojects` | `get/list/watch` | Application controller's namespace | Read sync windows |
  | `configmaps` | `get` | Application controller's namespace | Read `argocd-cm` and `argocd-cmd-params-cm` |
  | `leases` (`coordination.k8s.io`) | `get/list/watch/create/update/patch/delete` | Application controller's namespace | Leader election |
  | `events` | `create/patch` | App namespaces | Record phase changes |

- **Downgrade.** Removing the controller stops new runs. Operations it already wrote finish normally, and the CRDs
  and their objects stay as inert history.

## Drawbacks

- Two more CRDs, one more controller to run, and one more way to trigger a sync.
- Syncs through the Kubernetes API are authorized by Kubernetes RBAC, not by Argo CD RBAC.

## Alternatives

- **Status quo.** Keep writing `operation` on the Application, and use app-of-apps sync waves for ordering. That
  mutates managed objects and needs a parent app plus the Application health check.
- **Enforce Argo CD RBAC on kubectl-created ApplicationSyncs.** A mutating admission webhook would record the
  creator's Kubernetes identity in the spec, and the controller would check Argo CD RBAC against it
  before starting each app. Deferred because it needs:
  - a mapping from Kubernetes users and groups to Argo CD subjects;
  - a way to trust the recorded identity if the webhook is removed;
  - webhook availability and certificates;
  - care with server-side apply, because changes a mutating webhook makes are credited to the requester's
    field manager.

  It can be added later without changing the API beyond `spec.initiatedBy`.
- **Reconcile in the application controller.** No new deployment, and it reuses the controller's sharding and
  refresh queue. But an ApplicationSync spans apps on different shards, so ownership needs a rule (the shard owning
  its first app), and the sharded controller takes on rollout logic and load. A dedicated controller keeps one owner
  per object.
- **Extend RollingSync.** It stays bound to one ApplicationSet and label selectors, and still has no record of
  each run.
- **External orchestration.** Argo Workflows, or CI calling the Argo CD API. That works today, but needs API
  tokens, and the ordering and health-gating logic lives outside Argo CD.
- **A list of operations on the Application.** Queue several operations on the Application instead of one. That
  still mutates the Application and doesn't address ordering across apps.
