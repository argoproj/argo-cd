# Tracking and Deployment Strategies

An Argo CD application spec provides several different ways of tracking Kubernetes resource manifests.

In all tracking strategies, the app has the option to sync automatically. If [auto-sync](auto_sync.md)
is configured, the new resources manifests will be applied automatically -- as soon as a difference
is detected.

!!! note
    In all tracking strategies, any [parameter overrides](parameters.md) take precedence over the Git state.

## Helm

For Helm, all versions are [Semantic Versions](https://semver.org/). As a result, you can either version ranges:

| Use Case | How | Examples |
|-|-|-|
| Pin to a version (e.g. in production) | Use the version number | `1.2.0` |
| Track patches (e.g. in pre-production) | Use a range | `1.2.*` or `>=1.2.0 <1.3.0` |
| Track minor releases (e.g. in QA) | Use a range | `1.*` or `>=1.0.0 <2.0.0` |
| Use the latest (e.g. in local development) | Use star range |  `*` or `>=0.0.0` |

[Read about version ranges](https://www.telerik.com/blogs/the-mystical-magical-semver-ranges-used-by-npm-bower)

## Git

For Git, all versions are Git references:

| Use Case | How | Notes |
|-|-|-|
| Pin to a version (e.g. in production) | Either (a) tag the commit with (e.g. `v1.2.0`) and use that tag, or (b) using commit SHA. | See [commit pinning](#commit-pinning). |
| Track patches (e.g. in pre-production) | Tag/re-tag the commit, e.g. (e.g. `v1.2`) and use that tag. | See [tag tracking](#tag-tracking) |
| Track minor releases (e.g. in QA) | Re-tag the commit as (e.g. `v1`) and use that tag. | See [tag tracking](#tag-tracking) |
| Use the latest (e.g. in local development) | Use `HEAD` or `master` (assuming `master` is your master branch). | See [HEAD / Branch Tracking](#head-branch-tracking) |


### HEAD / Branch Tracking

If a branch name, or a symbolic reference (like HEAD) is specified, Argo CD will continually compare
live state against the resource manifests defined at the tip of the specified branch or the
resolved commit of the symbolic reference.

To redeploy an app,  makes a changes to your manifests, commit/push to the branch/symbolic reference. They will then detected by Argo CD.

### Tag Tracking

If a tag is specified, the manifests at the specified Git tag will be used to perform the sync
comparison. This provides some advantages over branch tracking in that a tag is generally considered
more stable, and less frequently updated, with some manual judgement of what constitutes a tag.

To redeploy an app, the user uses Git to change the meaning of a tag by retagging it to a
different commit SHA. Argo CD will detect the new meaning of the tag when performing the
comparison/sync.

### Commit Pinning

If a Git commit SHA is specified, the app is effectively pinned to the manifests defined at
the specified commit. This is the most restrictive of the techniques and is typically used to
control production environments.

Since commit SHAs cannot change meaning, the only way to change the live state of an app
which is pinned to a commit, is by updating the tracking revision in the application to a different
commit containing the new manifests. Note that [parameter overrides](parameters.md) can still be set
on an app which is pinned to a revision.

