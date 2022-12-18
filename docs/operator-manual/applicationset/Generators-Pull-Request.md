# Pull Request Generator

The Pull Request generator uses the API of an SCMaaS provider (GitHub, Gitea, or Bitbucket Server) to automatically discover open pull requests within a repository. This fits well with the style of building a test environment when you create a pull request.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - pullRequest:
      # When using a Pull Request generator, the ApplicationSet controller polls every `requeueAfterSeconds` interval (defaulting to every 30 minutes) to detect changes.
      requeueAfterSeconds: 1800
      # See below for provider specific options.
      github:
        # ...
```

!!! note
    Know the security implications of PR generators in ApplicationSets.
    [Only admins may create ApplicationSets](./Security.md#only-admins-may-createupdatedelete-applicationsets) to avoid
    leaking Secrets, and [only admins may create PRs](./Security.md#templated-project-field) if the `project` field of
    an ApplicationSet with a PR generator is templated, to avoid granting management of out-of-bounds resources.

## GitHub

Specify the repository from which to fetch the GitHub Pull requests.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - pullRequest:
      github:
        # The GitHub organization or user.
        owner: myorg
        # The Github repository
        repo: myrepository
        # For GitHub Enterprise (optional)
        api: https://git.example.com/
        # Reference to a Secret containing an access token. (optional)
        tokenRef:
          secretName: github-token
          key: token
        # (optional) use a GitHub App to access the API instead of a PAT.
        appSecretName: github-app-repo-creds
        # Labels is used to filter the PRs that you want to target. (optional)
        labels:
        - preview
      requeueAfterSeconds: 1800
  template:
  # ...
```

* `owner`: Required name of the GitHub organization or user.
* `repo`: Required name of the GitHub repository.
* `api`: If using GitHub Enterprise, the URL to access it. (Optional)
* `tokenRef`: A `Secret` name and key containing the GitHub access token to use for requests. If not specified, will make anonymous requests which have a lower rate limit and can only see public repositories. (Optional)
* `labels`: Labels is used to filter the PRs that you want to target. (Optional)
* `appSecretName`: A `Secret` name containing a GitHub App secret in [repo-creds format][repo-creds].

[repo-creds]: ../declarative-setup.md#repository-credentials

## GitLab

Specify the project from which to fetch the GitLab merge requests.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - pullRequest:
      gitlab:
        # The GitLab project.
        project: myproject
        # For self-hosted GitLab (optional)
        api: https://git.example.com/
        # Reference to a Secret containing an access token. (optional)
        tokenRef:
          secretName: gitlab-token
          key: token
        # Labels is used to filter the MRs that you want to target. (optional)
        labels:
        - preview
        # MR state is used to filter MRs only with a certain state. (optional)
        pullRequestState: opened
      requeueAfterSeconds: 1800
  template:
  # ...
```

* `project`: Required name of the GitLab project.
* `api`: If using self-hosted GitLab, the URL to access it. (Optional)
* `tokenRef`: A `Secret` name and key containing the GitLab access token to use for requests. If not specified, will make anonymous requests which have a lower rate limit and can only see public repositories. (Optional)
* `labels`: Labels is used to filter the MRs that you want to target. (Optional)
* `pullRequestState`: PullRequestState is an additional MRs filter to get only those with a certain state. Default: "" (all states)

## Gitea

Specify the repository from which to fetch the Gitea Pull requests.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - pullRequest:
      gitea:
        # The Gitea organization or user.
        owner: myorg
        # The Gitea repository
        repo: myrepository
        # The Gitea url to use
        api: https://gitea.mydomain.com/
        # Reference to a Secret containing an access token. (optional)
        tokenRef:
          secretName: gitea-token
          key: token
        # many gitea deployments use TLS, but many are self-hosted and self-signed certificates
        insecure: true
      requeueAfterSeconds: 1800
  template:
  # ...
```

* `owner`: Required name of the Gitea organization or user.
* `repo`: Required name of the Gitea repository.
* `api`: The url of the Gitea instance.
* `tokenRef`: A `Secret` name and key containing the Gitea access token to use for requests. If not specified, will make anonymous requests which have a lower rate limit and can only see public repositories. (Optional)
* `insecure`: `Allow for self-signed certificates, primarily for testing.`

## Bitbucket Server

Fetch pull requests from a repo hosted on a Bitbucket Server (not the same as Bitbucket Cloud).

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - pullRequest:
      bitbucketServer:
        project: myproject
        repo: myrepository
        # URL of the Bitbucket Server. Required.
        api: https://mycompany.bitbucket.org
        # Credentials for Basic authentication. Required for private repositories.
        basicAuth:
          # The username to authenticate with
          username: myuser
          # Reference to a Secret containing the password or personal access token.
          passwordRef:
            secretName: mypassword
            key: password
      # Labels are not supported by Bitbucket Server, so filtering by label is not possible.
      # Filter PRs using the source branch name. (optional)
      filters:
      - branchMatch: ".*-argocd"
  template:
  # ...
```

* `project`: Required name of the Bitbucket project
* `repo`: Required name of the Bitbucket repository.
* `api`: Required URL to access the Bitbucket REST API. For the example above, an API request would be made to `https://mycompany.bitbucket.org/rest/api/1.0/projects/myproject/repos/myrepository/pull-requests`
* `branchMatch`: Optional regexp filter which should match the source branch name. This is an alternative to labels which are not supported by Bitbucket server.

If you want to access a private repository, you must also provide the credentials for Basic auth (this is the only auth supported currently):
* `username`: The username to authenticate with. It only needs read access to the relevant repo.
* `passwordRef`: A `Secret` name and key containing the password or personal access token to use for requests.

## Filters

Filters allow selecting which pull requests to generate for. Each filter can declare one or more conditions, all of which must pass. If multiple filters are present, any can match for a repository to be included. If no filters are specified, all pull requests will be processed.
Currently, only a subset of filters is available when comparing with [SCM provider](Generators-SCM-Provider.md) filters.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - pullRequest:
      # ...
      # Include any pull request ending with "argocd". (optional)
      filters:
      - branchMatch: ".*-argocd"
  template:
  # ...
```

* `branchMatch`: A regexp matched against source branch names.

[GitHub](#github) and [GitLab](#gitlab) also support a `labels` filter.

## Template

As with all generators, several keys are available for replacement in the generated application.

The following is a comprehensive Helm Application example;

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - pullRequest:
    # ...
  template:
    metadata:
      name: 'myapp-{{branch}}-{{number}}'
    spec:
      source:
        repoURL: 'https://github.com/myorg/myrepo.git'
        targetRevision: '{{head_sha}}'
        path: kubernetes/
        helm:
          parameters:
          - name: "image.tag"
            value: "pull-{{head_sha}}"
      project: "my-project"
      destination:
        server: https://kubernetes.default.svc
        namespace: default
```

And, here is a robust Kustomize example;

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myapps
spec:
  generators:
  - pullRequest:
    # ...
  template:
    metadata:
      name: 'myapp-{{branch}}-{{number}}'
    spec:
      source:
        repoURL: 'https://github.com/myorg/myrepo.git'
        targetRevision: '{{head_sha}}'
        path: kubernetes/
        kustomize:
          nameSuffix: {{branch}}
          commonLabels:
            app.kubernetes.io/instance: {{branch}}-{{number}}
          images:
          - ghcr.io/myorg/myrepo:{{head_sha}}
      project: "my-project"
      destination:
        server: https://kubernetes.default.svc
        namespace: default
```

* `number`: The ID number of the pull request.
* `branch`: The name of the branch of the pull request head.
* `branch_slug`: The branch name will be cleaned to be conform to the DNS label standard as defined in [RFC 1123](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names), and truncated to 50 characters to give room to append/suffix-ing it with 13 more characters.
* `head_sha`: This is the SHA of the head of the pull request.
* `head_short_sha`: This is the short SHA of the head of the pull request (8 characters long or the length of the head SHA if it's shorter).

## Webhook Configuration

When using a Pull Request generator, the ApplicationSet controller polls every `requeueAfterSeconds` interval (defaulting to every 30 minutes) to detect changes. To eliminate this delay from polling, the ApplicationSet webhook server can be configured to receive webhook events, which will trigger Application generation by the Pull Request generator.

The configuration is almost the same as the one described [in the Git generator](Generators-Git.md), but there is one difference: if you want to use the Pull Request Generator as well, additionally configure the following settings.

### Github webhook configuration

In section 1, _"Create the webhook in the Git provider"_, add an event so that a webhook request will be sent when a pull request is created, closed, or label changed.

Add Webhook URL with uri `/api/webhook` and select content-type as json
![Add Webhook URL](../../assets/applicationset/webhook-config-pullrequest-generator.png "Add Webhook URL")

Select `Let me select individual events` and enable the checkbox for `Pull requests`.

![Add Webhook](../../assets/applicationset/webhook-config-pull-request.png "Add Webhook Pull Request")

The Pull Request Generator will requeue when the next action occurs.

- `opened`
- `closed`
- `reopened`
- `labeled`
- `unlabeled`
- `synchronized`

For more information about each event, please refer to the [official documentation](https://docs.github.com/en/developers/webhooks-and-events/webhooks/webhook-events-and-payloads).

### Gitlab webhook configuration

Enable checkbox for "Merge request events" in triggers list.

![Add Gitlab Webhook](../../assets/applicationset/webhook-config-merge-request-gitlab.png "Add Gitlab Merge request Webhook")

The Pull Request Generator will requeue when the next action occurs.

- `open`
- `close`
- `reopen`
- `update`
- `merge`

For more information about each event, please refer to the [official documentation](https://docs.gitlab.com/ee/user/project/integrations/webhook_events.html#merge-request-events).

## Lifecycle

An Application will be generated when a Pull Request is discovered when the configured criteria is met - i.e. for GitHub when a Pull Request matches the specified `labels` and/or `pullRequestState`. Application will be removed when a Pull Request no longer meets the specified criteria.
