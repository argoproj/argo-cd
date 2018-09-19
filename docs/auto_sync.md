# Automated Sync Policy

ArgoCD has the ability to automatically sync an application when it detects differences between
the desired manifests in git, and the live state in the cluster. A benefit of automatic sync is that
CI/CD pipelines no longer need direct access to the ArgoCD API server to perform the deployment.
Instead, the pipeline makes a commit and push to the git repository with the changes to the
manifests in the tracking git repo.

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

By default (and as a safety mechanism), automated sync will not delete resources when ArgoCD detects
the resource is no longer defined in git. To prune the resources, a manual sync can always be
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

## Automated Sync Semantics

* An automated sync will only be performed if the application is OutOfSync. Applications in a
  Synced or error state will not attempt automated sync.
* Automated sync will only attempt one synchronization per unique combination of commit SHA1 and
  application parameters. If the most recent successful sync in the history was already performed
  against the same commit-SHA and parameters, a second sync will not be attempted.
* Automatic sync will not reattempt a sync if the previous sync attempt against the same commit-SHA
  and parameters had failed.
* Rollback cannot be performed against an application with automated sync enabled.
