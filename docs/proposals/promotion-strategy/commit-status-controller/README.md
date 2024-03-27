# CommitStatus Controller

The CommitStatus controller provides a Kubernetes interface to the commit git SCM concept of a "commit status".

## Uni-directional and Canonical

A CommitStatus created in Kubernetes will be synced to the configured SCM with the specified settings. The
synchronization is uni-directional: creating a commit status in an SCM using some other mechanism will not cause a
CommitStatus to be created in Kubernetes.

If a CommitStatus creates a commit status in an SCM then, from the CommitStatus controller's perspective, the Kubernetes
resource "owns" that status. External changes to the commit status will not be copied back to the CommitStatus resource.
Instead, an error message is persisted in the CommitStatus's `status` field indicating that there is drift.

## Supported SCMs

The CommitStatus Controller supports BitBucket, GitHub, and GitLab "out of the box".

We could add support for other SCMs later via a plugin interface, similar to Argo Rollouts traffic routers and metrics providers. For now, we'll start with hard-coded support for GitHub and GitLab.

## Generalization of Fields

The CommitStatus API provides a generalized subset of SCMs' commit status fields. Where the most popular SCMs use
different words to mean something similar, the CommitStatus API will use a different synonym to avoid giving the
impression that it necessarily maps perfectly to any SCM's API.

The CommitStatus API is not meant to be a simple proxy to the underlying SCMs' APIs. Rather, it is meant to represent
the common _concept of a commit status_. By holding commit status information behind the CommitStatus abstraction, we
hide the details of each SCM from the user. So instead of needing to know the concept of "GitLab's 'state' field," or
"GitHub's 'status' field," the user needs only to know the CommitStatus's 'phase' field.

### CommitStatus to SCM Field Mappings

#### Field Names

| CommitStatus  | BitBucket | GitLab                                                | GitHub                                                     |
|:--------------|:----------|:------------------------------------------------------|:-----------------------------------------------------------|
| `name`        |           | `name` / `context` (the same value is copied to both) | `name` / `output.title` (the same value is copied to both) |
| `description` |           | `description`                                         | `output.summary`                                           |
| `phase`       |           | `state`                                               | `status`                                                   |

#### Field Values

##### `phase`

| CommitStatus  | BitBucket | GitLab | GitHub |
|:--------------|:----------|:-------|:-------|
| `queued`      |           |        |        |
| `in_progress` |           |        |        |
| `success`     |           |        |        |
| `failure`     |           |        |        |
| `canceled`    |           |        |        |

## CommitStatus API

```yaml
apiVersion: commit-status.argoproj.io/v1alpha1
kind: CommitStatus
spec:
  gitRepoRef:
    name:
  # These fields roughly correspond to the fields available in the GitHub Checks API and the Gitlab Commit "pipeline status" API. 
  sha: xyz321
  # Name is meant to be very brief. Corresponds to the `name` field in GitHub and the `name`/`context` field in Gitlab.
  name: hydration-successful
  # A longer description of the check. For GitHub, corresponds to the output.summary field (`name` will be reused for the output.title). For GitLab, corresponds to the `description` field.
  description:
  # The field is named `phase` to avoid implying it directly corresponds to either GitHub's `status` or GitLab's `state` field.
  phase: success # queued, in_progress, success, failure, or cancelled
  # This field provides a link to details about the commit's status.
  url: 
status:
  updatedAt:
  syncFailure: # optional, non-nil value implies an error
    message: # optional string
  # This field simply indicates whether the below fields match the declared fields in the SCM (as of last sync).
  isSynced: # bool
  # This field tracks the current repo we're tracking in the SCM.
  repo: # maybe a string or maybe an object - depends on how we want to uniquely identify repos
  # This field tracks the current SHA we're tracking in the SCM.
  sha:
  # These fields reflect the actual current values in the SCM:
  name:
  description:
  phase:
  url:
  # These fields represent the internal information the provider requires to maintain its state.
  provider:
    gitlab:
    github:
      checkRunId: # Optional int
    # If we add plugin support later, those plugins' statuses will get their own fields here.
```

## Reusing CommitStatus Resources

A single CommitStatus resource may refer to more than one commit in the lifetime of the resource. The `sha` field, the
`gitRepoRef` reference, and the underlying GitRepo resource may change over time.

Allowing changes to the referenced commit allows CommitStatus API consumers to track the status of a git revision over
time without requiring the CommitStatus owner to maintain a large number of resources or implement some TTL system.

For example, the CommitStatus owner may use the name `my-repo-main-branch` to track the status of the commit at the tip
of the `main` branch in the `my-repo` repo.

### How `sha`/Repo Changes are Handled when `queued` or `in_progress`

If the `sha` changes while the `phase` is set to `queued` or `in_progress`, the CommitStatus controller will attempt to
set the actual phase in the SCM to `canceled` before moving on to handle the new SHA. If the CommitStatus controller
fails to set the phase with the SCM, it will log the error and move on. If your application needs to be 100% certain
that the `canceled` status is set, update the `phase` first and watch `status.phase` until it matches, then proceed to
update the `sha`.

Different SCMs use different information to uniquely identify their concepts of a repository. If the CommitStatus
determines that a changed `gitRepoRef` or a change in the underlying GitRepo represents a change in the selected
repository, it will follow the same procedure described above for `sha` changes to set the previously-referenced commit
`phase` to `cancelled`.
