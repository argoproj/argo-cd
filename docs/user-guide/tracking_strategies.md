# Tracking and Deployment Strategies

An Argo CD application spec provides several different ways of tracking Kubernetes resource manifests.

In all tracking strategies, the app has the option to sync automatically. If [auto-sync](auto_sync.md)
is configured, the new resources manifests will be applied automatically -- as soon as a difference
is detected.

> [!NOTE]
> In all tracking strategies, any [parameter overrides](parameters.md) take precedence over the Git state.

## Helm

Helm chart versions are [Semantic Versions](https://semver.org/). As a result, you can use any of the following version ranges:

| Use Case | How | Examples |
|-|-|-|
| Pin to a version (e.g. in production) | Use the version number | `1.2.0` |
| Track patches (e.g. in pre-production) | Use a range | `1.2.*` or `>=1.2.0 <1.3.0` |
| Track minor releases (e.g. in QA) | Use a range | `1.*` or `>=1.0.0 <2.0.0` |
| Use the latest (e.g. in local development) | Use star range |  `*` or `>=0.0.0` |
| Use the latest including pre-releases | Use star range with `-0` suffix |  `*-0` or `>=0.0.0-0` |

[Read about version ranges](https://www.telerik.com/blogs/the-mystical-magical-semver-ranges-used-by-npm-bower)

> [!NOTE]
> If you want Argo CD to include all existing prerelease version tags of a repository in the comparison logic, you explicitly have to add a prerelease `-0` suffix to the version constraint. As mentioned `*-0` will compare against prerelease versions in a repository, `*` will not. The same applies for other constraints e.g. `>=1.2.2` will **not** compare prerelease versions vs. `>=1.2.2-0` which will include prerelease versions in the comparison.

[Read about prerelease version comparison](https://github.com/Masterminds/semver?tab=readme-ov-file#working-with-prerelease-versions)

## Git

For Git, all versions are Git references but tags [Semantic Versions](https://semver.org/) can also be used:

| Use Case | How | Notes |
|-|-|-|
| Pin to a version (e.g. in production) | Either (a) tag the commit with (e.g. `v1.2.0`) and use that tag, or (b) using commit SHA. | See [commit pinning](#commit-pinning). |
| Track patches (e.g. in pre-production) | Use a range (e.g. `1.2.*` or `>=1.2.0 <1.3.0`)                                           | See [tag tracking](#tag-tracking) |
| Track minor releases (e.g. in QA) | Use a range (e.g. `1.*` or `>=1.0.0 <2.0.0`)                                             | See [tag tracking](#tag-tracking) |
| Use the latest (e.g. in local development) | Use `HEAD` or `master` (assuming `master` is your master branch).                        | See [HEAD / Branch Tracking](#head-branch-tracking) |
| Use the latest including pre-releases | Use star range with `-0` suffix | `*-0` or `>=0.0.0-0` |


### HEAD / Branch Tracking

If a branch name or a symbolic reference (like HEAD) is specified, Argo CD will continually compare
live state against the resource manifests defined at the tip of the specified branch or the
resolved commit of the symbolic reference.

To redeploy an app, make a change to (at least) one of your manifests, commit and push to the tracked branch/symbolic reference. The change will then be detected by Argo CD.

### Tag Tracking

If a tag is specified, the manifests at the specified Git tag will be used to perform the sync
comparison. This provides some advantages over branch tracking in that a tag is generally considered
more stable, and less frequently updated, with some manual judgement of what constitutes a tag.

To redeploy an app, the user uses Git to change the meaning of a tag by retagging it to a
different commit SHA. Argo CD will detect the new meaning of the tag when performing the
comparison/sync.

But if you're using semantic versioning you can set the constraint in your service revision
and Argo CD will get the latest version following the constraint rules.

> [!NOTE]
> Semver constraints (those containing `*`, `>`, `<`, `>=`, `<=`, `~`, `^`, or range expressions like `>=1.0.0 <2.0.0`) are **only matched against tags**, never branches. This is by design - semver resolution uses the list of Git tags exclusively.

#### Prefixed Tags

Argo CD supports hierarchical tag prefixes, allowing you to organize tags by application, environment, cluster, or any other criteria. This is particularly useful for:

- **Monorepos** - Tag each application separately (e.g., `app1/v1.0.0`, `app2/v2.0.0`)
- **Multi-cluster deployments** - Organize by application, cluster, and environment for use with ApplicationSet generators

| Tag Format | Constraint | Description |
|-|-|-|
| `app1/v1.0.0`, `app1/v1.0.1` | `app1/v1.0.*` | Track patch releases for app1 in a monorepo |
| `app2/v1.0.0`, `app2/v2.0.0` | `app2/v1.*` | Track minor releases for app2 |
| `app1/cluster1/prod/v1.0.0` | `app1/cluster1/prod/v1.*` | Nested prefixes for app/cluster/environment |
| `app1/v1.0.0`, `app1/v1.0.1` | `app1/*` | All versions for an application |

**Examples:**

```yaml
# Monorepo: Track patch releases for a specific application
spec:
  source:
    targetRevision: app1/v1.0.*

# Multi-cluster: Track versions for a specific app/cluster/environment
spec:
  source:
    targetRevision: app1/cluster1/prod/v1.*

# ApplicationSet generator example - use template variables in prefix
spec:
  source:
    targetRevision: "{{.app}}/{{.cluster}}/{{.env}}/v1.*"
```

> [!NOTE]
> For prerelease versions, use the `-0` suffix: `>=app1/v1.0.0-0` will match prerelease tags like `app1/v1.0.0-rc.1`.

### Commit Pinning

If a Git commit SHA is specified, the app is effectively pinned to the manifests defined at
the specified commit. This is the most restrictive of the techniques and is typically used to
control production environments.

Since commit SHAs cannot change meaning, the only way to change the live state of an app
which is pinned to a commit, is by updating the tracking revision in the application to a different
commit containing the new manifests. Note that [parameter overrides](parameters.md) can still be set
on an app which is pinned to a revision.

### Handling Ambiguous Git References in Argo CD

When deploying applications, Argo CD relies on the `targetRevision` field to determine
which revision of the Git repository to use. This can be a branch, tag, or commit SHA.
Sometimes, multiple Git references can have the same name (eg. a branch and a tag both named `release-1.0`).
These ambiguous references can lead to unexpected behavior, such as constant reconciliation loops.

Today, Argo CD fetches all branches and tags from the repository. If the `targetRevision` matches multiple references, Argo CD
attempts to resolve it and may select a different commit than expected. For example, suppose your repository has the following references:

```text
refs/heads/release-1.0 -> commit B
refs/tags/release-1.0  -> commit A
```

In the above scenario, `release-1.0` refers to both a branch (pointing to commit B) and a tag (pointing to commit A). 
If your application's `targetRevision` is set to `release-1.0`, Argo CD may resolve it to either commit A or commit B.
If the resolved commit differs from what is currently deployed, Argo CD will continuously attempt to sync, causing constant
reconciliation. In order to avoid this ambiguity, you can follow these best practices:

1. Use fully-qualified Git references in the `targetRevision` field. For example, use `refs/heads/release-1.0` for branches
   and `refs/tags/release-1.0` for tags.
2. Avoid using the same name for branches and tags in your Git repository.
