# SCM Provider Generator

The SCM Provider generator uses the API of an SCMaaS provider (eg GitHub) to automatically discover repositories within an organization. This fits well with GitOps layout patterns that split microservices across many repositories.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - scmProvider:
      # Which protocol to clone using.
      cloneProtocol: ssh
      # See below for provider specific options.
      github:
        # ...
```

* `cloneProtocol`: Which protocol to use for the SCM URL. Default is provider-specific but ssh if possible. Not all providers necessarily support all protocols, see provider documentation below for available options.

## GitHub

The GitHub mode uses the GitHub API to scan and organization in either github.com or GitHub Enterprise.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - scmProvider:
      github:
        # The GitHub organization to scan.
        organization: myorg
        # For GitHub Enterprise:
        api: https://git.example.com/
        # If true, scan every branch of every repository. If false, scan only the default branch. Defaults to false.
        allBranches: true
        # Reference to a Secret containing an access token. (optional)
        tokenRef:
          secretName: github-token
          key: token
  template:
  # ...
```

* `organization`: Required name of the GitHub organization to scan. If you have multiple orgs, use multiple generators.
* `api`: If using GitHub Enterprise, the URL to access it.
* `allBranches`: By default (false) the template will only be evaluated for the default branch of each repo. If this is true, every branch of every repository will be passed to the filters. If using this flag, you likely want to use a `branchMatch` filter.
* `tokenRef`: A `Secret` name and key containing the GitHub access token to use for requests. If not specified, will make anonymous requests which have a lower rate limit and can only see public repositories.

For label filtering, the repository topics are used.

Available clone protocols are `ssh` and `https`.

## Gitlab

The Gitlab mode uses the Gitlab API to scan and organization in either gitlab.com or self-hosted gitlab.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - scmProvider:
      gitlab:
        # The base Gitlab group to scan.  You can either use the group id or the full namespaced path.
        group: "8675309"
        # For GitLab Enterprise:
        api: https://gitlab.example.com/
        # If true, scan every branch of every repository. If false, scan only the default branch. Defaults to false.
        allBranches: true
        # If true, recurses through subgroups. If false, it searches only in the base group. Defaults to false.
        includeSubgroups: true
        # Reference to a Secret containing an access token. (optional)
        tokenRef:
          secretName: gitlab-token
          key: token
  template:
  # ...
```

* `group`: Required name of the base Gitlab group to scan. If you have multiple base groups, use multiple generators.
* `api`: If using GitHub Enterprise, the URL to access it.
* `allBranches`: By default (false) the template will only be evaluated for the default branch of each repo. If this is true, every branch of every repository will be passed to the filters. If using this flag, you likely want to use a `branchMatch` filter.
* `includeSubgroups`: By default (false) the controller will only search for repos directly in the base group. If this is true, it will recurse through all the subgroups searching for repos to scan.
* `tokenRef`: A `Secret` name and key containing the Gitlab access token to use for requests. If not specified, will make anonymous requests which have a lower rate limit and can only see public repositories.

For label filtering, the repository tags are used.

Available clone protocols are `ssh` and `https`.

## Filters

Filters allow selecting which repositories to generate for. Each filter can declare one or more conditions, all of which must pass. If multiple filters are present, any can match for a repository to be included. If no filters are specified, all repositories will be processed.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - scmProvider:
      filters:
      # Include any repository starting with "myapp" AND including a Kustomize config AND labeled with "deploy-ok" ...
      - repositoryMatch: ^myapp
        pathsExist: [kubernetes/kustomization.yaml]
        labelMatch: deploy-ok
      # ... OR any repository starting with "otherapp" AND a Helm folder.
      - repositoryMatch: ^otherapp
        pathsExist: [helm]
  template:
  # ...
```

* `repositoryMatch`: A regexp matched against the repository name.
* `pathsExist`: An array of paths within the repository that must exist. Can be a file or directory, but do not include the trailing `/` for directories.
* `labelMatch`: A regexp matched against repository labels. If any label matches, the repository is included.
* `branchMatch`: A regexp matched against branch names.

## Template

As with all generators, several parameters are generated for use within the `ApplicationSet` resource template.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - scmProvider:
    # ...
  template:
    metadata:
      name: '{{ repository }}'
    spec:
      source:
        repoURL: '{{ url }}'
        targetRevision: '{{ branch }}'
        path: kubernetes/
      project: default
      destination:
        server: https://kubernetes.default.svc
        namespace: default
```

* `organization`: The name of the organization the repository is in.
* `repository`: The name of the repository.
* `url`: The clone URL for the repository.
* `branch`: The default branch of the repository.
* `sha`: The Git commit SHA for the branch
* `labels`: A comma-separated list of repository labels