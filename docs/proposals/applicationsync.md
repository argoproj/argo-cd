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
last-updated: 2026-10-23
---

# ApplicationSync

## Summary

Today, a sync is requested through the API, CLI or UI, or by writing `operation` on the Application. Each request
covers one app and changes the Application. Only successful syncs are recorded, per app, in `status.history`; a failed
sync is gone once the next operation replaces `status.operationState`, and nothing records which syncs belong
together. Ordering syncs across apps needs ApplicationSet RollingSync, which is tied to one ApplicationSet, or
app-of-apps sync waves.

This proposal adds two resources:

- **`ApplicationSync`**: a one-off request to sync groups of Applications in dependency order, gated on health,
  with per-app status. Completed requests are kept as history.
- **`ApplicationSyncPolicy`**: creates an ApplicationSync whenever its apps drift, which makes it auto-sync for a
  group of apps.

RollingSync could later be rebuilt on top of them (see [Future work](#future-work)).

This proposal asks for the controller to ship with Argo CD. The prototype, which runs next to an existing installation
(see [Appendix: prototype notes](#appendix-prototype-notes)), validates the design until then.

## Open Questions

1. **Revision pins through kubectl.** `requireOverridePrivilegeForRevisionSync` can't be enforced on ApplicationSyncs
   created through the Kubernetes API. Should they be able to pin any revision, as writing `operation` can today?
2. **Branch and tag pins.** Should the controller resolve them once through the repo server, so every app gets the
   same commit, or should they float, leaving users who need one commit to pin a SHA?
3. **Selectors in groups.** Should a group accept label selectors, as RollingSync steps do, or only app names?
4. **Self-heal in the application controller.** Should self-heal move to the application controller, which would need
   a way to enable self-heal without `automated`, so that policies only roll out changes?

## Motivation

- **Sync requests that fit GitOps.** Today, syncing with kubectl means patching `operation` onto the Application.
  When that Application is itself managed from Git, by an app-of-apps or an ApplicationSet, this is an out-of-band
  edit to a managed object. A separate request object can be applied, committed or templated instead.
- **Ordering across apps.** Today, the options are RollingSync, which is tied to one ApplicationSet and its label
  selectors, or app-of-apps sync waves, which need a parent Application and the Application health check restored
  (see [upgrading to 1.8](../operator-manual/upgrading/1.7-1.8.md)).
- **Progressive sync outside the ApplicationSet.** In [#28927](https://github.com/argoproj/argo-cd/issues/28927),
  maintainers supported eventually giving Progressive Sync its own CRD and controller. Several open RollingSync bugs
  come from the ApplicationSet controller rebuilding a sync state machine out of Application status fields.
- **History and group auto-sync.** Today, only successful syncs are recorded, per app, and auto-sync works one app at
  a time.

### Goals

- Sync groups of Applications with `kubectl apply`, in dependency order, gated on health.
- Keep one record per request, with per-app status, as bounded history.
- Apply the same guards as an API sync: sync windows, project permissions and `server.sync.replace.allowed`.
- Provide the building block for group auto-sync, and later RollingSync.

### Non-Goals

- Removing `operation` from the Application: the controller still writes it, and auto-sync, rollbacks and local
  syncs keep using it directly.
- Enforcing Argo CD RBAC on requests made through the Kubernetes API (see [Alternatives](#alternatives)). In-tree,
  the API checks Argo CD RBAC (see [Security Considerations](#security-considerations)).
- Syncing Applications in more than one namespace from one ApplicationSync.
- A workflow engine: no arbitrary steps, approvals or hooks between apps.
- Rollout analysis beyond health, such as metric checks

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
nothing records which revisions belonged together. This restores revisions only: if an app's source settings (path,
Helm parameters) changed since, they stay as they are now, unlike `argocd app rollback` (see
[Future work](#future-work)).

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
spec:
  prune: true                      # also dryRun, syncStrategy, syncOptions, retry: applied to every app
  activeDeadlineSeconds: 3600      # optional: fail the run if it hasn't finished within an hour
  ttlSecondsAfterFinished: 604800  # optional: delete it a week after it finishes
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
  phase: Running # Running | Succeeded | Failed; a run with no phase yet counts as unfinished
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

- **Ordering.** A group starts once every group in its `dependsOn` has succeeded, and syncs its apps in parallel.
- **Done.** An app is done when its sync succeeded and it is Synced and Healthy, as in RollingSync. `doneWhen:
  Healthy` drops the Synced requirement.
- **Failure.** A failed app fails its group, groups that depend on it are Skipped, and independent groups carry on.
  With `onFailure: Wait`, a failed or Degraded app holds its dependents until it recovers instead.
- **Waiting.** An app that doesn't exist yet waits, so the Applications and the ApplicationSync can be applied
  together. The app's status and an event say why it waits.
- **Busy apps.** As the Sync API rejects a sync today, an app that already has an operation running fails, unless
  `onFailure: Wait` is set, in which case it waits for the operation to finish. Policy runs always wait, since a
  policy doesn't retry a failed change.
- **Deadline.** `activeDeadlineSeconds` fails a run that hasn't finished in time, as for a Job: nothing new starts,
  and operations already running finish. Without it, a run can wait indefinitely.
- **Immutable.** The spec can't change after creation, apart from the TTL. To stop a run, delete it: operations
  already running finish and nothing new starts. To sync again, create a new one.
- **Runs managed from Git.** As with Jobs, commit a run as a sync hook with `generateName`: each sync of the parent
  app creates one run, which Argo CD then neither prunes nor recreates, and a health check for ApplicationSync makes
  the parent's sync wait for it. A run committed as an ordinary resource would be pruned or rerun. For ongoing
  rollouts, use a policy.
- **No auto-sync on listed apps.** Auto-sync could otherwise sync an app ahead of its dependencies, so the controller
  refuses such an app. Group auto-sync is the [ApplicationSyncPolicy](#applicationsyncpolicy).

#### Lifecycle

| Stage | ApplicationSync | Applications |
|---|---|---|
| Created | By a user, CI or a GitOps tool, or by a [policy](#applicationsyncpolicy). It has no status until the controller first sees it. | Untouched. |
| Running | The controller sets `Running` and `startedAt`, or `Failed` if validation fails, then starts apps group by group. An app waits while it doesn't exist yet, or its sync window is closed. A busy app fails, or with `onFailure: Wait` waits. | The controller may request a refresh, then writes `operation` tagged with the run's name and UID. The application controller runs the sync as usual and records it in `status.operationState` and `status.history`. |
| Completed | `Succeeded` or `Failed`, with `finishedAt`. Only `status.observedGeneration` changes after this, on a spec edit; the controller comes back to it only to delete it, once newer runs push it past the history limit or its TTL passes. | Untouched. |
| Deleted | By garbage collection, once it is outside the newest runs kept for each of its apps or its TTL has passed; by the Kubernetes garbage collector when its policy is deleted; by its policy when `concurrencyPolicy: Replace` starts a new run, or when the run is unfinished and [not recorded by the policy](#appendix-prototype-notes); or by a user. There is no finalizer: deleting a running ApplicationSync cancels it at once, operations it already wrote finish, and nothing new starts. | Keep their operation state and history. |

#### History

Completed ApplicationSyncs are the sync history of their apps: which apps were synced, in what order, to which
revisions, and how each ended. Rolling back means creating a new ApplicationSync pinned to an earlier run's revisions,
which a tool or the UI can offer as "re-run with these revisions".

Each app keeps its 20 most recent completed runs (configurable), and `ttlSecondsAfterFinished` deletes a run a set
time after it finishes. Once a run is gone, each Application's own `status.history` still records the revisions it
synced.

### Rollout controls

Ordered rollouts need a few controls on top of ordering. Each matches something RollingSync does today:

| Control | Effect | RollingSync today |
|---|---|---|
| `maxParallel` (group) | Caps how many of the group's apps sync at once; at least 1 | `maxUpdate` |
| `revisions` (app) | Pins sources, so a commit that lands mid-rollout can't reach apps that haven't synced yet | `TargetRevisions` per app |
| `skipIfSynced` | Doesn't resync an app already at its pins, but still waits for it to be Healthy | Synced apps skip the sync |
| `onFailure: Wait` | A failed or Degraded app holds its dependents until it recovers | Same |
| `doneWhen: Healthy` | An app that stays OutOfSync still counts as done | None |
| `prune`, `retry`, `resources` (app) | Per-app overrides; `resources` is a partial sync | Per-app `automated.prune` and retry |
| `info`, `initiatedBy` | Notes copied into each operation, and who asked (informational only) | "Reason" operation info |

The controls rely on three behaviors:

- **Fresh status.** An app is only skipped or counted as done once its status reflects its current spec and pins;
  otherwise the controller requests a refresh. Today, RollingSync can act on stale status
  ([#29410](https://github.com/argoproj/argo-cd/issues/29410),
  [#27949](https://github.com/argoproj/argo-cd/issues/27949)).
- **Sync windows.** Policy runs count as automatic and other runs as manual, and windows apply as they do today: the
  controller marks a run's operations as automatic (`operation.initiatedBy.automated`) when a policy owns the run. A
  closed window delays an app without starting an operation or using a `maxParallel` slot. Today, RollingSync writes
  the operation anyway ([#29307](https://github.com/argoproj/argo-cd/issues/29307)).
- **Empty syncs.** In a policy run with prune, an app whose resources would all be pruned fails unless `allowEmpty` is
  set, as `automated.allowEmpty` does today.

Observability: metrics for [#27846](https://github.com/argoproj/argo-cd/issues/27846) (time to first sync, time per
group, syncs started, runs per phase), an event when an app starts waiting, and logs on phase changes
([#18240](https://github.com/argoproj/argo-cd/issues/18240)).

### Revisions

Each app can pin each of its sources, by name or by position:

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

- **Listed sources** are fixed when the run starts, so `revision: main` means `main` at that moment. **Unlisted
  sources** float: they sync to `targetRevision` as it stands when the app's sync starts. Policy runs pin every
  source to the revision the app last compared against.
- Status records each app's pins and what it actually synced to.
- An app pinned to something other than its `targetRevision` reports OutOfSync afterwards, as `argocd app sync
  --revision` does today.
- How a branch or tag pin becomes one commit is Open Question 2. Whether kubectl users may pin something other than
  `targetRevision` is Open Question 1. Hydrated apps aren't supported.

### ApplicationSyncPolicy

A policy creates ApplicationSyncs ("runs") from a template whenever its apps drift. In effect it is auto-sync for a
group of apps:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSyncPolicy
metadata:
  name: platform
  namespace: argocd
spec:
  concurrencyPolicy: Forbid  # Forbid | Replace (delete the running ApplicationSync, start a new one)
  suspend: false             # true stops the policy from starting runs
  selfHeal: true
  template:                  # an ApplicationSync spec
    skipIfSynced: true
    activeDeadlineSeconds: 3600
    groups:
    - name: data
      apps: [{name: db}]
    - name: backend
      dependsOn: [data]
      apps: [{name: api}]
```

- **When it starts a run.** Today, auto-sync acts on one app when a reconcile finds it OutOfSync. A policy does the
  same for its group: when a listed app is OutOfSync, it starts a run with every source pinned to the revision the app
  was last compared against. It has no schedule of its own.
- **One run per change.** As auto-sync does today, a policy rolls out each change once: drift at revisions it already
  rolled out only starts a run with `selfHeal: true`, and a change whose run failed isn't retried. Deleting that run
  makes the change new again, which is how to retry it.
- **Concurrency.** With `Forbid`, a new change waits for the current run to finish. With `Replace`, the current run is
  canceled and a new one starts.
- **Self-heal.** With `selfHeal: true`, drift after a successful run starts a run that syncs only what drifted, with
  the same backoff as `automated.selfHeal` today. Open Question 4 asks whether this belongs in the application
  controller instead.
- **Auto-sync stays off.** A policy starts no run while one of its apps has `syncPolicy.automated` on, and says so in
  its status.
- **Suspend.** As with a CronJob, `suspend: true` stops the policy from starting runs. A run already going continues;
  delete it to stop the rollout.
- **Runs.** Runs are owned by their policy and deleted with it, as Jobs are with their CronJob, and count as automatic
  for sync windows. A policy keeps its latest run, whatever its TTL, until a newer one replaces it, because that run
  records what the policy last rolled out.
- **Apps from anywhere.** A policy can list apps from several ApplicationSets, or from none
  ([#14458](https://github.com/argoproj/argo-cd/issues/14458)). Until Open Question 3 is settled, apps are listed by
  name.

### Future work

None of this is part of the proposal; each item depends on `ApplicationSync` moving into Argo CD.

- **UI.** Each Application lists its recent runs, and a run view shows its groups in order. With RollingSync on
  ApplicationSync, the ApplicationSet view would show its current run, covering most of
  [#27879](https://github.com/argoproj/argo-cd/issues/27879).
- **API and UI syncs as ApplicationSyncs.** The Sync handler could create a single-app run instead of writing
  `operation`, so every sync leaves a record, and new endpoints would create runs and policies for users, checking
  Argo CD RBAC; a busy app still fails, and a timed-out request deletes its run. Rollbacks restore the source settings
  recorded in history (path, Helm parameters, …), not only the revision, so a run would need a way to name a history
  entry. Local syncs keep writing `operation`, at least for now.
- **Auto-sync as a generated policy.** The application controller could keep a single-app policy for each auto-synced
  app, so auto-sync and group auto-sync share one engine. The cost is one policy per app and one run per auto-sync.
- **RollingSync as a generated policy.** RollingSync is evaluated on every ApplicationSet reconcile, so the
  ApplicationSet controller would generate a policy with `concurrencyPolicy: Replace`: steps become groups, and
  `maxUpdate` becomes `maxParallel`. That addresses most open progressive-sync issues in
  [#28927](https://github.com/argoproj/argo-cd/issues/28927), such as stale status
  ([#29410](https://github.com/argoproj/argo-cd/issues/29410)), operations started in a closed window
  ([#29307](https://github.com/argoproj/argo-cd/issues/29307)) and missing history
  ([#25741](https://github.com/argoproj/argo-cd/issues/25741)). Open design points: holding a step (`maxUpdate: 0`),
  template edits mid-rollout, `create-only` and `create-delete`
  ([#29142](https://github.com/argoproj/argo-cd/issues/29142)), and migrating rollouts already in flight.
- **Schedules.** A cron schedule on a policy, as a second trigger next to drift.

### Security Considerations

- **Kubernetes RBAC for now.** Out of tree, runs and policies are created through the Kubernetes API, so Kubernetes
  RBAC governs them, as for writing `operation` today. Without [apps in any
  namespace](../operator-manual/app-any-namespace.md), every Application lives in the control-plane namespace, so
  `create` on either is effectively admin. This is a stopgap: in-tree, `argocd-server` creates them for users and
  checks Argo CD RBAC.
- **Other verbs matter too.** `update` or `patch` can set a run's TTL to 0, which deletes it, and can add or remove a
  policy owner reference, which changes whether sync windows treat the run as automatic. `delete` on a policy's
  latest run makes the policy roll out its change again. Grant these as carefully as `create`.
- **Existing guards still apply.** Before starting a sync, the controller applies the same checks as an API sync: sync
  windows, `server.sync.replace.allowed`, and refusing an app that is being deleted. In-tree, these should be one
  check shared with the Sync API rather than a copy. The application controller still enforces project destinations
  and source integrity, as today. Local manifests and source overrides aren't exposed, and the username in
  `spec.initiatedBy` is informational only.
- **The controller's own access.** It can write `operation` on the Applications it watches, as the application
  controller can, and reads AppProjects and Argo CD's ConfigMaps. Resolving branch pins (Open Question 2) would add
  read access to repository credentials.

### Risks and Mitigations

- **Object count.** Every run is an object in etcd. Each app keeps at most 20 completed runs by default, so 10,000
  apps hold up to 200,000 single-app runs, about 280 MB. App names that come and go, as with pull request generators,
  are only bounded by a TTL. Mitigation: a configurable history limit, `ttlSecondsAfterFinished`, and
  [Prior Art](#prior-art) options such as separate failure limits or history outside etcd.
- **Stuck runs.** An app that never becomes Synced or Healthy, or never appears, holds its dependents, and a `Forbid`
  policy waits behind it. Mitigation: the reason shows in status and events; `doneWhen: Healthy`,
  `activeDeadlineSeconds`, and suspending the policy.
- **Apps in two runs.** Runs don't lock apps. A run that finds an app busy fails it, or with `onFailure: Wait` waits
  for it, so runs can interleave across apps. Mitigation: document that a policy's apps shouldn't be listed in other
  runs.
- **Apps that still auto-sync.** Apps moved into ApplicationSyncs may keep auto-sync on. Mitigation: the app fails,
  or waits, with a message naming the setting.

### Upgrade / Downgrade Strategy

- **Upgrade.** Additive: two CRDs and a controller Deployment, and nothing changes until ApplicationSyncs or policies
  are created. The controller needs to write Applications and its own resources in the app namespaces, and to read
  AppProjects and Argo CD's ConfigMaps.
- **Downgrade.** Removing the controller stops new runs. Operations already started finish, and existing objects stay
  as inert history.

## Drawbacks

- Two more CRDs, another controller to run, and another way to trigger a sync.
- Syncs through the Kubernetes API are authorized by Kubernetes RBAC, not Argo CD RBAC.

## Alternatives

- **Status quo.** Keep writing `operation` and using app-of-apps sync waves. That edits managed objects and needs a
  parent app.
- **A long-lived rollout object.** Editing its spec would roll out again, with its status keeping the last few runs.
  But syncing again needs a dummy edit, two requests close together can overwrite each other, and history lives in one
  size-limited status. The policy is that long-lived object for auto-sync; one-off requests stay separate, immutable
  runs.
- **Argo CD RBAC on kubectl-created runs.** An admission webhook could record the creator, and the controller could
  check Argo CD RBAC before each sync. Deferred: it needs a mapping from Kubernetes identities to Argo CD subjects, a
  way to trust the recorded identity, and a webhook to run. 
- **Reconcile in the application controller.** No new Deployment, but a run spans apps on different shards, and the
  sharded controller takes on rollout logic and load.
- **Extend RollingSync.** It stays tied to one ApplicationSet and its selectors, with no record per run.
- **External orchestration.** Argo Workflows, or CI calling the Argo CD API, works today, but needs API tokens and
  keeps the ordering logic outside Argo CD.
- **Queue operations on the Application.** It still edits the Application and doesn't order apps.

## Prior Art

Recording each run as its own object, and capping how many are kept, is a common Kubernetes pattern:

| Project | Object per run | What bounds it |
|---|---|---|
| Kubernetes | `Job` | A CronJob keeps 3 successful and 1 failed Job; `ttlSecondsAfterFinished` deletes a Job after it finishes |
| Argo Rollouts | `AnalysisRun` | A Rollout keeps 5 successful and 5 unsuccessful runs |
| Kargo | `Promotion` | 20 per Stage, none deleted before it is two weeks old |
| Argo Workflows | `Workflow` | `ttlStrategy`; the [workflow archive](https://argo-workflows.readthedocs.io/en/latest/workflow-archive/) keeps history in SQL |

This proposal counts history per app rather than per owner, so runs created by hand are bounded too; standalone
AnalysisRuns, which no Rollout owns, are never collected
([argo-rollouts#3285](https://github.com/argoproj/argo-rollouts/issues/3285)). It also takes the Job's TTL. Separate
limits for failed runs, and history outside etcd, are left for later.

## Appendix: prototype notes

The prototype, `argo-cd-sync-controller`, is a standalone controller that runs next to Argo CD and talks to it only
through the Kubernetes API. These notes describe how it implements the proposal; the implementation may differ.

- **Dedicated controller.** Leader-elected, so each object has one owner and no sharded component takes on the load.
  Runs and policies are indexed by the apps they list.
- **Names.** `ApplicationSync` and `ApplicationSyncPolicy`, because `Sync` alone clashes with `v1alpha1` type names.
- **Operations.** It writes `operation` as the Sync API does, tagged with the run's name and UID, and reads the result
  from `status.operationState`. After a crash it carries on with the operation it wrote rather than starting another,
  and it checks live objects before writing, so a stale cache never acts on a deleted run or a recreated app.
- **Deleted apps.** An app deleted or recreated after its sync started fails. Before that, a missing app waits and a
  recreated one is synced.
- **Revisions.** Pins are matched to sources when the app is first seen and again when its sync starts. An entry that
  matches no source, or would move to another source, fails the app. Branch pins are passed as given (Open Question 2).
- **History.** Runs are ordered by `finishedAt` for the history limit. Runs that list no apps are only removed by
  their TTL, and unfinished runs are never collected.
- **Policies.** A policy records its latest run's name, UID and change in its status, trusts only runs it recorded, and
  cancels unrecorded runs that name it as owner. Self-heal runs keep the last run's pins, sync only OutOfSync
  resources, and leave out apps that run left OutOfSync.
- **Events.** Routine waits (a refresh, or a `maxParallel` slot) aren't reported.
- **RBAC.** It needs `applications` (write), `applicationsyncs` and `applicationsyncpolicies` with their status, and
  `events` in the app namespaces; `update` on `applicationsyncpolicies/finalizers`, since runs carry a blocking owner
  reference to their policy; `appprojects` and `configmaps` (read) and a `lease` in Argo CD's namespace.
