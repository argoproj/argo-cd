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
        # If true and includeSubgroups is also true, include Shared Projects, which is gitlab API default.
        # If false only search Projects under the same path. Defaults to true.
        includeSharedProjects: false
        # filter projects by topic. A single topic is supported by Gitlab API. Defaults to "" (all topics).
        topic: "my-topic"
        # Reference to a Secret containing an access token. (optional)
        tokenRef:
          secretName: gitlab-token
          key: token
        # If true, skips validating the SCM provider's TLS certificate - useful for self-signed certificates.
        insecure: false
        # Reference to a ConfigMap containing trusted CA certs - useful for self-signed certificates. (optional)
        caRef:
          configMapName: argocd-tls-certs-cm
          key: gitlab-ca
  template:
  # ...
```

* `group`: Required name of the base GitLab group to scan. If you have multiple base groups, use multiple generators.
* `api`: If using self-hosted GitLab, the URL to access it.
* `allBranches`: By default (false) the template will only be evaluated for the default branch of each repo. If this is true, every branch of every repository will be passed to the filters. If using this flag, you likely want to use a `branchMatch` filter.
* `includeSubgroups`: By default (false) the controller will only search for repos directly in the base group. If this is true, it will recurse through all the subgroups searching for repos to scan.
* `includeSharedProjects`: If true and includeSubgroups is also true, include Shared Projects, which is gitlab API default. If false only search Projects under the same path. In general most would want the behaviour when set to false. Defaults to true.
* `topic`: filter projects by topic. A single topic is supported by Gitlab API. Defaults to "" (all topics).
* `tokenRef`: A `Secret` name and key containing the GitLab access token to use for requests. If not specified, will make anonymous requests which have a lower rate limit and can only see public repositories.
* `insecure`: By default (false) - Skip checking the validity of the SCM's certificate - useful for self-signed TLS certificates.
* `caRef`: Optional `ConfigMap` name and key containing the GitLab certificates to trust - useful for self-signed TLS certificates. Possibly reference the ArgoCD CM holding the trusted certs.

For label filtering, the repository topics are used.

Available clone protocols are `ssh` and `https`.

### Self-signed TLS Certificates

As a preferable alternative to setting `insecure` to true, you can configure self-signed TLS certificates for Gitlab.

In order for a self-signed TLS certificate be used by an ApplicationSet's SCM / PR Gitlab Generator, the certificate needs to be mounted on the applicationset-controller. The path of the mounted certificate must be explicitly set using the environment variable `ARGOCD_APPLICATIONSET_CONTROLLER_SCM_ROOT_CA_PATH` or alternatively using parameter `--scm-root-ca-path`. The applicationset controller will read the mounted certificate to create the Gitlab client for SCM/PR Providers

This can be achieved conveniently by setting `applicationsetcontroller.scm.root.ca.path` in the argocd-cmd-params-cm ConfigMap. Be sure to restart the ApplicationSet controller after setting this value.

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
        # Credentials for Basic authentication (App Password). Either basicAuth or bearerToken
        # authentication is required to access private repositories
        basicAuth:
          # The username to authenticate with
          username: myuser
          # Reference to a Secret containing the password or personal access token.
          passwordRef:
            secretName: mypassword
            key: password
        # Credentials for Bearer Token (App Token) authentication. Either basicAuth or bearerToken
        # authentication is required to access private repositories
        bearerToken:
          # Reference to a Secret containing the bearer token.
          tokenRef:
            secretName: repotoken
            key: token
        # If true, skips validating the SCM provider's TLS certificate - useful for self-signed certificates.
        insecure: true
        # Reference to a ConfigMap containing trusted CA certs - useful for self-signed certificates. (optional)
        caRef:
          configMapName: argocd-tls-certs-cm
          key: bitbucket-ca
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

In case of Bitbucket App Token, go with `bearerToken` section.
* `tokenRef`: A `Secret` name and key containing the app token to use for requests.

In case self-signed BitBucket Server certificates, the following options can be usefully:
* `insecure`: By default (false) - Skip checking the validity of the SCM's certificate - useful for self-signed TLS certificates.
* `caRef`: Optional `ConfigMap` name and key containing the BitBucket server certificates to trust - useful for self-signed TLS certificates. Possibly reference the ArgoCD CM holding the trusted certs.

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

## Bitbucket Cloud

The Bitbucket mode uses the Bitbucket API V2 to scan a workspace in bitbucket.org.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - scmProvider:
      bitbucket:
        # The workspace id (slug).  
        owner: "example-owner"
        # The user to use for basic authentication with an app password.
        user: "example-user"
        # If true, scan every branch of every repository. If false, scan only the main branch. Defaults to false.
        allBranches: true
        # Reference to a Secret containing an app password.
        appPasswordRef:
          secretName: appPassword
          key: password
  template:
  # ...
```

* `owner`: The workspace ID (slug) to use when looking up repositories.
* `user`: The user to use for authentication to the Bitbucket API V2 at bitbucket.org.
* `allBranches`: By default (false) the template will only be evaluated for the main branch of each repo. If this is true, every branch of every repository will be passed to the filters. If using this flag, you likely want to use a `branchMatch` filter.
* `appPasswordRef`: A `Secret` name and key containing the bitbucket app password to use for requests.

This SCM provider does not yet support label filtering

Available clone protocols are `ssh` and `https`.

## AWS CodeCommit (Alpha)

Uses AWS ResourceGroupsTagging and AWS CodeCommit APIs to scan repos across AWS accounts and regions.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
    - scmProvider:
        awsCodeCommit:
          # AWS region to scan repos.
          # default to the environmental region from ApplicationSet controller.
          region: us-east-1
          # AWS role to assume to scan repos.
          # default to the environmental role from ApplicationSet controller.
          role: arn:aws:iam::111111111111:role/argocd-application-set-discovery
          # If true, scan every branch of every repository. If false, scan only the main branch. Defaults to false.
          allBranches: true
          # AWS resource tags to filter repos with.
          # see https://docs.aws.amazon.com/resourcegroupstagging/latest/APIReference/API_GetResources.html#resourcegrouptagging-GetResources-request-TagFilters for details
          # default to no tagFilters, to include all repos in the region.
          tagFilters:
            - key: organization
              value: platform-engineering
            - key: argo-ready
  template:
  # ...
```

* `region`: (Optional) AWS region to scan repos. By default, use ApplicationSet controller's current region.
* `role`: (Optional) AWS role to assume to scan repos. By default, use ApplicationSet controller's current role.
* `allBranches`: (Optional) If `true`, scans every branch of eligible repositories. If `false`, check only the default branch of the eligible repositories. Default `false`.
* `tagFilters`: (Optional) A list of tagFilters to filter AWS CodeCommit repos with. See [AWS ResourceGroupsTagging API](https://docs.aws.amazon.com/resourcegroupstagging/latest/APIReference/API_GetResources.html#resourcegrouptagging-GetResources-request-TagFilters) for details. By default, no filter is included.

This SCM provider does not support the following features

* label filtering
* `sha`, `short_sha` and `short_sha_7` template parameters

Available clone protocols are `ssh`, `https` and `https-fips`.

### AWS IAM Permission Considerations

In order to call AWS APIs to discover AWS CodeCommit repos, ApplicationSet controller must be configured with valid environmental AWS config, like current AWS region and AWS credentials.
AWS config can be provided via all standard options, like Instance Metadata Service (IMDS), config file, environment variables, or IAM roles for service accounts (IRSA).

Depending on whether `role` is provided in `awsCodeCommit` property, AWS IAM permission requirement is different.

#### Discover AWS CodeCommit Repositories in the same AWS Account as ApplicationSet Controller

Without specifying `role`, ApplicationSet controller will use its own AWS identity to scan AWS CodeCommit repos.
This is suitable when you have a simple setup that all AWS CodeCommit repos reside in the same AWS account as your Argo CD.

As the ApplicationSet controller AWS identity is used directly for repo discovery, it must be granted below AWS permissions.

* `tag:GetResources`
* `codecommit:ListRepositories`
* `codecommit:GetRepository`
* `codecommit:GetFolder`
* `codecommit:ListBranches`

#### Discover AWS CodeCommit Repositories across AWS Accounts and Regions

By specifying `role`, ApplicationSet controller will first assume the `role`, and use it for repo discovery.
This enables more complicated use cases to discover repos from different AWS accounts and regions.

The ApplicationSet controller AWS identity should be granted permission to assume target AWS roles.

* `sts:AssumeRole`

All AWS roles must have repo discovery related permissions.

* `tag:GetResources`
* `codecommit:ListRepositories`
* `codecommit:GetRepository`
* `codecommit:GetFolder`
* `codecommit:ListBranches`

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
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  generators:
  - scmProvider:
    # ...
  template:
    metadata:
      name: '{{ .repository }}'
    spec:
      source:
        repoURL: '{{ .url }}'
        targetRevision: '{{ .branch }}'
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
* `short_sha_7`: The abbreviated Git commit SHA for the branch (7 chars or the length of the `sha` if it's shorter).
* `labels`: A comma-separated list of repository labels in case of Gitea, repository topics in case of Gitlab and Github. Not supported by Bitbucket Cloud, Bitbucket Server, or Azure DevOps.
* `branchNormalized`: The value of `branch` normalized to contain only lowercase alphanumeric characters, '-' or '.'.

## Pass additional key-value pairs via `values` field

You may pass additional, arbitrary string key-value pairs via the `values` field of any SCM generator. Values added via the `values` field are added as `values.(field)`.

In this example, a `name` parameter value is passed. It is interpolated from `organization` and `repository` to generate a different template name.
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  generators:
  - scmProvider:
      bitbucketServer:
        project: myproject
        api: https://mycompany.bitbucket.org
        allBranches: true
        basicAuth:
          username: myuser
          passwordRef:
            secretName: mypassword
            key: password
      values:
        name: "{{.organization}}-{{.repository}}"

  template:
    metadata:
      name: '{{ .values.name }}'
    spec:
      source:
        repoURL: '{{ .url }}'
        targetRevision: '{{ .branch }}'
        path: kubernetes/
      project: default
      destination:
        server: https://kubernetes.default.svc
        namespace: default
```

!!! note
    The `values.` prefix is always prepended to values provided via `generators.scmProvider.values` field. Ensure you include this prefix in the parameter name within the `template` when using it.

In `values` we can also interpolate all fields set by the SCM generator as mentioned above.
