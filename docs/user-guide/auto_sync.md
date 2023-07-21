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

Or by setting the self heal option to true in the automated sync policy:

```yaml
spec:
  syncPolicy:
    automated:
      selfHeal: true
```

## Automated Sync Semantics

* An automated sync will only be performed if the application is OutOfSync. Applications in a
  Synced or error state will not attempt automated sync.
* Automated sync will only attempt one synchronization per unique combination of commit SHA1 and
  application parameters. If the most recent successful sync in the history was already performed
  against the same commit-SHA and parameters, a second sync will not be attempted, unless `selfHeal` flag is set to true.
* If `selfHeal` flag is set to true then sync will be attempted again after self heal timeout (5 seconds by default)
which is controlled by `--self-heal-timeout-seconds` flag of `argocd-application-controller` deployment.
* Automatic sync will not reattempt a sync if the previous sync attempt against the same commit-SHA
  and parameters had failed.

* Rollback cannot be performed against an application with automated sync enabled.
