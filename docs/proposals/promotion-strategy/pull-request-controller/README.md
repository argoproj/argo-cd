# PullRequest Controller

The PullRequest CRD provides an Kubernetes interface to various SCMs' pull request APIs. For now, GitHub and GitLab are the only supported SCMS.

The interface is meant to cover common SCM features rather than SCM-specific tools. Where two SCMs handle a similar concept differently, the PullRequest abstraction will attempt to provide a reasonable common interface.

This is the PullRequest spec:

```yaml
apiVersion: promoter.argoproj.io/v1alpha1
kind: PullRequest
spec:
  # Reference to a GitRepository resource.
  repoRef:
    name:
  # The title of the PR.
  title:
  # The description of the PR.
  description:
  # The branch we're merging from.
  sourceBranch:
  # The branch we're merging into.
  targetBranch:
  # Whether the PR should be open, closed, or draft. If something external changes the state, the controller will return it to the desired state.
  state:
status:
  # Whether the PR is currently open, closed, draft, or merged.
  state:
  # Boolean showing whether the branches differ. If they do not differ, no PR will be opened in the SCM (there's nothing to merge).
  branchesDiffer: 
  # Each provider has its own storage space for SCM-specific PR info, for example the generated numeric ID.
  provider:
    github:
      id:
    gitlab:
      id:
```

If there is no difference between the `sourceBranch` and `targetBranch`, the controller will set `status.branchesDiffer` to `false` and move on.

If the controller opens a PR and then some external system sets it to a state different from `spec.state`, the controller will seek to return the PR to the desired state. For example, if someone closes the PR, the controller will reopen it.

The controller will operate on a 3-minute polling interval and will also support webhooks for better responsiveness to changes in the SCM.
