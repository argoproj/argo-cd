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

!!! note
    Know the security implications of using SCM generators. [Only admins may create ApplicationSets](./Security.md#only-admins-may-createupdatedelete-applicationsets)
    to avoid leaking Secrets, and [only admins may create repos/branches](./Security.md#templated-project-field) if the
    `project` field of an ApplicationSet with an SCM generator is templated, to avoid granting management of
    out-of-bounds resources.

## GitHub

The GitHub mode uses the GitHub API to scan an organization in either github.com or GitHub Enterprise.

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
        # (optional) use a GitHub App to access the API instead of a PAT.
        appSecretName: gh-app-repo-creds
  template:
  # ...
```

* `organization`: Required name of the GitHub organization to scan. If you have multiple organizations, use multiple generators.
* `api`: If using GitHub Enterprise, the URL to access it.
* `allBranches`: By default (false) the template will only be evaluated for the default branch of each repo. If this is true, every branch of every repository will be passed to the filters. If using this flag, you likely want to use a `branchMatch` filter.
* `tokenRef`: A `Secret` name and key containing the GitHub access token to use for requests. If not specified, will make anonymous requests which have a lower rate limit and can only see public repositories.
* `appSecretName`: A `Secret` name containing a GitHub App secret in [repo-creds format][repo-creds].

[repo-creds]: ../declarative-setup.md#repository-credentials

For label filtering, the repository topics are used.

Available clone protocols are `ssh` and `https`.

## Gitlab

The GitLab mode uses the GitLab API to scan and organization in either gitlab.com or self-hosted GitLab.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - scmProvider:
      gitlab:
        # The base GitLab group to scan.  You can either use the group id or the full namespaced path.
        group: "8675309"
        # For self-hosted GitLab:
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

* `group`: Required name of the base GitLab group to scan. If you have multiple base groups, use multiple generators.
* `api`: If using self-hosted GitLab, the URL to access it.
* `allBranches`: By default (false) the template will only be evaluated for the default branch of each repo. If this is true, every branch of every repository will be passed to the filters. If using this flag, you likely want to use a `branchMatch` filter.
* `includeSubgroups`: By default (false) the controller will only search for repos directly in the base group. If this is true, it will recurse through all the subgroups searching for repos to scan.
* `tokenRef`: A `Secret` name and key containing the GitLab access token to use for requests. If not specified, will make anonymous requests which have a lower rate limit and can only see public repositories.

For label filtering, the repository tags are used.

Available clone protocols are `ssh` and `https`.

## Gitea

The Gitea mode uses the Gitea API to scan organizations in your instance

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - scmProvider:
      gitea:
        # The Gitea owner to scan.
        owner: myorg
        # The Gitea instance url
        api: https://gitea.mydomain.com/
        # If true, scan every branch of every repository. If false, scan only the default branch. Defaults to false.
        allBranches: true
        # Reference to a Secret containing an access token. (optional)
        tokenRef:
          secretName: gitea-token
          key: token
  template:
  # ...
```

* `owner`: Required name of the Gitea organization to scan. If you have multiple organizations, use multiple generators.
* `api`: The URL of the Gitea instance you are using.
* `allBranches`: By default (false) the template will only be evaluated for the default branch of each repo. If this is true, every branch of every repository will be passed to the filters. If using this flag, you likely want to use a `branchMatch` filter.
* `tokenRef`: A `Secret` name and key containing the Gitea access token to use for requests. If not specified, will make anonymous requests which have a lower rate limit and can only see public repositories.
* `insecure`: Allow for self-signed TLS certificates.

This SCM provider does not yet support label filtering

Available clone protocols are `ssh` and `https`.

## Bitbucket Server

Use the Bitbucket Server API (1.0) to scan repos in a project. Note that Bitbucket Server is not to same as Bitbucket Cloud (API 2.0)

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - scmProvider:
      bitbucketServer:
        project: myproject
        # URL of the Bitbucket Server. Required.
        api: https://mycompany.bitbucket.org
        # If true, scan every branch of every repository. If false, scan only the default branch. Defaults to false.
        allBranches: true
        # Credentials for Basic authentication. Required for private repositories.
        basicAuth:
          # The username to authenticate with
          username: myuser
          # Reference to a Secret containing the password or personal access token.
          passwordRef:
            secretName: mypassword
            key: password
        # Support for filtering by labels is TODO. Bitbucket server labels are not supported for PRs, but they are for repos
  template:
  # ...
```

* `project`: Required name of the Bitbucket project
* `api`: Required URL to access the Bitbucket REST api.
* `allBranches`: By default (false) the template will only be evaluated for the default branch of each repo. If this is true, every branch of every repository will be passed to the filters. If using this flag, you likely want to use a `branchMatch` filter.

If you want to access a private repository, you must also provide the credentials for Basic auth (this is the only auth supported currently):
* `username`: The username to authenticate with. It only needs read access to the relevant repo.
* `passwordRef`: A `Secret` name and key containing the password or personal access token to use for requests.

Available clone protocols are `ssh` and `https`.

## Azure DevOps

Uses the Azure DevOps API to look up eligible repositories based on a team project within an Azure DevOps organization.
The default Azure DevOps URL is `https://dev.azure.com`, but this can be overridden with the field `azureDevOps.api`.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - scmProvider:
      azureDevOps:
        # The Azure DevOps organization.
        organization: myorg
        # URL to Azure DevOps. Optional. Defaults to https://dev.azure.com.
        api: https://dev.azure.com
        # If true, scan every branch of eligible repositories. If false, check only the default branch of the eligible repositories. Defaults to false.
        allBranches: true
        # The team project within the specified Azure DevOps organization.
        teamProject: myProject
        # Reference to a Secret containing the Azure DevOps Personal Access Token (PAT) used for accessing Azure DevOps.
        accessTokenRef:
          secretName: azure-devops-scm
          key: accesstoken
  template:
  # ...
```

* `organization`: Required. Name of the Azure DevOps organization.
* `teamProject`: Required. The name of the team project within the specified `organization`.
* `accessTokenRef`: Required. A `Secret` name and key containing the Azure DevOps Personal Access Token (PAT) to use for requests.
* `api`: Optional. URL to Azure DevOps. If not set, `https://dev.azure.com` is used.
* `allBranches`: Optional, default `false`. If `true`, scans every branch of eligible repositories. If `false`, check only the default branch of the eligible repositories.


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
      # ... OR include any repository starting with "otherapp" AND a Helm folder and doesn't have file disabledrepo.txt.
      - repositoryMatch: ^otherapp
        pathsExist: [helm]
        pathsDoNotExist: [disabledrepo.txt]
  template:
  # ...
```

* `repositoryMatch`: A regexp matched against the repository name.
* `pathsExist`: An array of paths within the repository that must exist. Can be a file or directory.
* `pathsDoNotExist`: An array of paths within the repository that must not exist. Can be a file or directory.
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
* `sha`: The Git commit SHA for the branch.
* `short_sha`: The abbreviated Git commit SHA for the branch (8 chars or the length of the `sha` if it's shorter).
* `labels`: A comma-separated list of repository labels.
* `branchNormalized`: The value of `branch` normalized to contain only lowercase alphanumeric characters, '-' or '.'.
