# Automated Sync Policy

Argo CD has the ability to automatically sync an application when it detects differences between
the desired manifests in Git, and the live state in the cluster. A benefit of automatic sync is that
CI/CD pipelines no longer need direct access to the Argo CD API server to perform the deployment.
Instead, the pipeline makes a commit and push to the Git repository with the changes to the
manifests in the tracking Git repo.

To configure automated sync run:
```bash
argocd app set <APPNAME> --sync-policy automated
```

Alternatively, if creating the application an application manifest, specify a syncPolicy with an
`automated` policy.
```yaml
spec:
  syncPolicy:
    automated: {}
```
Application CRD now also support explicitly setting automated sync to be turned on or off by using `spec.syncPolicy.automated.enabled` flag to true or false. When `enable` field is set to true, Automated Sync is active and when set to false controller will skip automated sync even if `prune`, `self-heal` and `allowEmpty` are set.
```yaml
spec:
  syncPolicy:
    automated:
      enabled: true
```

> [!NOTE]
> Setting the `spec.syncPolicy.automated.enabled` flag to null will be treated as if automated sync is enabled. When the `enabled` field is set to false, fields like `prune`, `selfHeal` and `allowEmpty` can be set without enabling them.

## Temporarily toggling auto-sync for applications managed by ApplicationSets

For a standalone application, toggling auto-sync is performed by changing the application's `spec.syncPolicy.automated` field. For an ApplicationSet managed application, changing the application's `spec.syncPolicy.automated` field will, however, have no effect.
[Controlling Resource Modification](../operator-manual/applicationset/Controlling-Resource-Modification.md) has more details about how to perform the toggling for applications managed by ApplicationSets.

If you only need to disable auto-sync so that you can roll back an application, consider using
[rollback-aware automated sync](#rollback-aware-automated-sync-v37) instead. It lets you roll back while auto-sync
stays enabled, so you don't need to change the ApplicationSet at all.


## Automatic Pruning

By default (and as a safety mechanism), automated sync will not delete resources when Argo CD detects
the resource is no longer defined in Git. To prune the resources, a manual sync can always be
performed (with pruning checked). Pruning can also be enabled to happen automatically as part of the
automated sync by running:

```bash
argocd app set <APPNAME> --auto-prune
```

Or by setting the prune option to true in the automated sync policy:

```yaml
spec:
  syncPolicy:
    automated:
      prune: true
```

## Automatic Pruning with Allow-Empty (v1.8)

By default (and as a safety mechanism), automated sync with prune have a protection from any automation/human errors 
when there are no target resources. It prevents application from having empty resources. To allow applications have empty resources, run:

```bash
argocd app set <APPNAME> --allow-empty
```

Or by setting the allow empty option to true in the automated sync policy:

```yaml
spec:
  syncPolicy:
    automated:
      prune: true
      allowEmpty: true
```

## Automatic Self-Healing
By default, changes that are made to the live cluster will not trigger automated sync. To enable automatic sync 
when the live cluster's state deviates from the state defined in Git, run:

```bash
argocd app set <APPNAME> --self-heal
```

Or by setting the self-heal option to true in the automated sync policy:

```yaml
spec:
  syncPolicy:
    automated:
      selfHeal: true
```

> [!NOTE]
> Disabling self-heal does not guarantee that live cluster changes in multi-source applications will persist. Although one of the resource's sources remains unchanged, changes in another can trigger `autosync`. To handle such cases, consider disabling `autosync`.

## Automatic Retry with a limit

Argo CD can automatically retry a failed sync operation using exponential backoff. To enable, configure the `retry` field in the sync policy:

```yaml
spec:
  syncPolicy:
    retry:
      limit: 5 # number of retries (-1 for unlimited retries)
      backoff:
        duration: 5s # base duration between retries
        factor: 2 # exponential backoff factor
        maxDuration: 3m # maximum duration between retries
```

- `limit`: number of retry attempts. Set to `-1` for unlimited retries.
- `backoff.duration`: base wait time before the first retry.
- `backoff.factor`: multiplier applied after each failed attempt.
- `backoff.maxDuration`: maximum wait time between retries, regardless of the number of attempts.

## Automatic Retry Refresh on new revisions

This feature allows users to configure their applications to refresh on new revisions when the current sync is retrying. To enable automatic refresh during sync retries, run:

```bash
argocd app set <APPNAME> --sync-retry-refresh
```

Or by setting the `retry.refresh` option to `true` in the sync policy:

```yaml
spec:
  syncPolicy:
    retry:
      refresh: true
```

## Automated Sync Semantics

* An automated sync will only be performed if the application is OutOfSync. Applications in a
  Synced or error state will not attempt automated sync.
* Automated sync will only attempt one synchronization per unique combination of commit SHA1 and
  application parameters. If the most recent successful sync in the history was already performed
  against the same commit-SHA and parameters, a second sync will not be attempted, unless `selfHeal` flag is set to true.
* If the `selfHeal` flag is set to true, then the sync will be attempted again after self-heal timeout (5 seconds by default)
which is controlled by `--self-heal-timeout-seconds` flag of `argocd-application-controller` deployment.
* Automatic sync will not reattempt a sync if the previous sync attempt against the same commit-SHA
  and parameters had failed.

* Rollback cannot be performed against an application with automated sync enabled, unless
  [rollback-aware automated sync](#rollback-aware-automated-sync-v37) is enabled for the application.
* The automatic sync interval is determined by [the `timeout.reconciliation` value in the `argocd-cm` ConfigMap](../faq.md#how-often-does-argo-cd-check-for-changes-to-my-git-or-helm-repository), which defaults to `120s` with added jitter of `60s` for a maximum period of 3 minutes.

## Rollback-Aware Automated Sync (v3.7)

By default, a rollback is rejected while automated sync is enabled, because the next automated sync would immediately
re-deploy the revision you just rolled back from. The usual workaround is to disable automated sync, roll back, fix the
offending commit, and re-enable automated sync.

Rollback-aware automated sync removes those manual steps. When it is enabled, a rollback is accepted while automated
sync stays on, and Argo CD remembers the revision it rolled back from. Automated sync then skips that revision until
the application source moves to a different one.

The feature is off by default. Enable it instance-wide in the `argocd-cm` ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
  labels:
    app.kubernetes.io/name: argocd-cm
    app.kubernetes.io/part-of: argocd
data:
  application.rollbackAwareAutoSyncEnabled: "true"
```

Or enable it for a single application, which overrides the instance-wide default in either direction:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
spec:
  syncPolicy:
    automated:
      rollbackAware: true
```

The same field is available from the CLI:

```bash
argocd app set guestbook --sync-policy automated --rollback-aware
```

Applications generated by an ApplicationSet inherit the value from `spec.template.spec.syncPolicy.automated.rollbackAware`.

### How it works

1. You roll back the application to an earlier entry in its history. Automated sync stays enabled.
2. Once the rollback succeeds, Argo CD records the revision the application was rolled back from in
   `status.rolledBackRevision` (`status.rolledBackRevisions` for multi-source applications).
3. The application is now OutOfSync, because the source still points at the rolled-back revision. Automated sync
   skips it and sets an `AutoSyncPausedWarning` condition on the application explaining why.
4. You push a fix. The source resolves to a new revision, so Argo CD clears the record and automated sync deploys the
   fix as usual.

Because the record lives in the application status, it is not affected by an ApplicationSet re-applying the
application spec.

### Checking whether automated sync is paused

While automated sync is skipping a rolled-back revision, the application has an `AutoSyncPausedWarning` condition.
In the UI, the condition is shown on the application details page, and the sync status panel shows which revision is
being skipped. From the CLI, the condition is listed in the output of:

```bash
argocd app get guestbook
```

The condition is removed as soon as the application source points to a new revision.

When the feature is enabled for an application, the rollback dialog in the UI no longer asks you to disable automated
sync before rolling back.

If you want to be alerted when an application stays paused for too long, you can expose the `AutoSyncPausedWarning`
condition as a Prometheus metric. See
[Exposing Application conditions as Prometheus metrics](../operator-manual/metrics.md#exposing-application-conditions-as-prometheus-metrics).

### Semantics and edge cases

* Only the most recent rollback is remembered. Rolling back again replaces the record.
* Any successful sync of a different revision clears the record, including a manual sync. A manual sync to the
  rolled-back revision itself is treated as a deliberate override and also clears the record.
* The record is also cleared when the application becomes Synced without a sync running. This happens when you
  revert the offending commit, because the new revision renders the same manifests as the one you rolled back to.
* Self-heal is paused as well. Self-heal syncs the application to the revision in Git, which is the one you rolled
  back from, so changes made directly in the cluster are not reverted while automated sync is paused. Self-heal
  starts working again once the application source points to a new revision.
* If the rollback operation fails, nothing is recorded and automated sync continues as before.
* Rolling back to the revision that is currently deployed is not a rollback away from anything, so nothing is
  recorded. The revision that gets recorded is always the deployed one, which is not necessarily the revision the
  application source resolves to: if the source has already moved to a newer revision that has never been deployed,
  automated sync stays free to deploy it.
* Automated sync also stays paused while Argo CD cannot resolve the application source at all, for example during a
  repository outage. The record is kept until a successful comparison shows which revision the source points at.
* The record is written whenever rollback-aware automated sync is enabled for the application, even if automated sync
  is disabled at the time of the rollback. Re-enabling automated sync later will therefore not re-deploy the
  rolled-back revision.
* If you disable the feature while a revision is recorded, automated sync stops consulting the record immediately. The
  record stays in the status until the next successful sync clears it.
* `revisionHistoryLimit` has no effect on the record.
