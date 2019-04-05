# Tracking and Deployment Strategies

An Argo CD application spec provides several different ways of track kubernetes resource manifests in
Git. This document describes the different techniques and the means of deploying those manifests to
the target environment.

## HEAD / Branch Tracking

If a branch name, or a symbolic reference (like HEAD) is specified, Argo CD will continually compare
live state against the resource manifests defined at the tip of the specified branch or the
dereferenced commit of the symbolic reference.

To redeploy an application, a user makes changes to the manifests, and commit/pushes those the
changes to the tracked branch/symbolic reference, which will then be detected by Argo CD controller.

## Tag Tracking

If a tag is specified, the manifests at the specified Git tag will be used to perform the sync
comparison. This provides some advantages over branch tracking in that a tag is generally considered
more stable, and less frequently updated, with some manual judgement of what constitutes a tag.

To redeploy an application, the user uses Git to change the meaning of a tag by retagging it to a
different commit SHA. Argo CD will detect the new meaning of the tag when performing the
comparison/sync.

## Commit Pinning

If a Git commit SHA is specified, the application is effectively pinned to the manifests defined at
the specified commit. This is the most restrictive of the techniques and is typically used to
control production environments.

Since commit SHAs cannot change meaning, the only way to change the live state of an application
which is pinned to a commit, is by updating the tracking revision in the application to a different
commit containing the new manifests. Note that [parameter overrides](parameters.md) can still be set
on an application which is pinned to a revision.

## Automated Sync

In all tracking strategies, the application has the option to sync automatically. If [auto-sync](auto_sync.md)
is configured, the new resources manifests will be applied automatically -- as soon as a difference
is detected between the target state (Git) and live state. If auto-sync is disabled, a manual sync
will be needed using the Argo UI, CLI, or API.

## Parameter Overrides
Note that in all tracking strategies, any [parameter overrides](parameters.md) set in the
application instance take precedence over the Git state.
