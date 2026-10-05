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


## Automatic Pruning

By default (and as a safety mechanism), automated sync will not delete resources when Argo CD detects
the resource is no longer defined in Git. To prune the resources, a manual sync can always be
performed (with pruning checked), or you can enable [default prune for manual sync](#default-manual-sync-options)
via `syncPolicy.manualDefaults.prune`. Pruning can also be enabled to happen automatically as part of the
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

## Default Manual Sync Options

You can configure defaults for **manual** syncs (UI, CLI, or API) without enabling automated
sync. When a sync request omits a value, Argo CD uses `spec.syncPolicy.manualDefaults`:

| Field | Default | Meaning |
|-------|---------|---------|
| `prune` | `true` | Delete resources missing from Git |
| `dryRun` | `false` | Preview-only sync |
| `applyOnly` | `false` | Use apply strategy instead of hooks |
| `force` | `false` | Pass `--force` to kubectl |

An explicit value on the sync request always takes precedence. These settings are independent of
`syncPolicy.automated.*`.

```bash
argocd app set <APPNAME> --sync-prune --sync-dry-run=false --sync-apply-only=false --sync-force=false
```

Or declaratively:

```yaml
spec:
  syncPolicy:
    manualDefaults:
      prune: true
      dryRun: false
      applyOnly: false
      force: false
```

> [!NOTE]
> Manual (and other non-automated) syncs — including ApplicationSet RollingSync — use
> `syncPolicy.manualDefaults`. Automated sync behavior is controlled only by `syncPolicy.automated`.

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

* Rollback cannot be performed against an application with automated sync enabled.
* The automatic sync interval is determined by [the `timeout.reconciliation` value in the `argocd-cm` ConfigMap](../faq.md#how-often-does-argo-cd-check-for-changes-to-my-git-or-helm-repository), which defaults to `120s` with added jitter of `60s` for a maximum period of 3 minutes.
