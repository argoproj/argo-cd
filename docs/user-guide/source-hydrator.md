# Source Hydrator

**Current feature state**: Alpha

Tools like Helm and Kustomize allow users to express their Kubernetes manifests in a more concise and reusable way
(keeping it DRY - Don't Repeat Yourself). However, these tools can obscure the actual Kubernetes manifests that are
applied to the cluster.

The "rendered manifest pattern" is a feature of Argo CD that allows users to push the hydrated manifests to git before syncing them to the cluster. This
allows users to see the actual Kubernetes manifests that are applied to the cluster.

## Enabling the Source Hydrator

The source hydrator is disabled by default.

To enable the source hydrator, you need to enable the "commit server" component and set the `hydrator.enabled` field in
argocd-cmd-params-cm ConfigMap to `"true"`.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
  namespace: argocd
data:
  hydrator.enabled: "true"
```

> [!IMPORTANT]
> After updating the ConfigMap, you must restart the Argo CD controller and API server for the changes to take effect.

If you are using one of the `*-install.yaml` manifests to install Argo CD, you can use the 
`*-install-with-hydrator.yaml` version of that file instead.

For example,

```
Without hydrator: https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
With hydrator:    https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install-with-hydrator.yaml
```

> [!IMPORTANT]
> The `*-with-hydrator-install.yaml` manifests will eventually be removed when the source hydrator is either enabled
> by default or removed. The upgrade guide will note if the `install-with-hydrator.yaml` manifests are no longer
> available.

## Using the Source Hydrator

To use the source hydrator, you must first install a push and a pull secret. This example uses a GitHub App for authentication, but
you can use [any authentication method that Argo CD supports for repository access](../operator-manual/declarative-setup.md#repositories).

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-push-secret
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository-write
type: Opaque
stringData:
  url: "https://github.com/<your org or user>/<your repo>"
  type: "git"
  githubAppID: "<your app ID here>"
  githubAppInstallationID: "<your installation ID here>"
  githubAppPrivateKey: |
    <your private key here>
---
apiVersion: v1
kind: Secret
metadata:
  name: my-pull-secret
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
type: Opaque
stringData:
  url: "https://github.com/<your org or user>/<your repo>"
  type: "git"
  githubAppID: "<your app ID here>"
  githubAppInstallationID: "<your installation ID here>"
  githubAppPrivateKey: |
    <your private key here>
```

The only difference between the secrets above, besides the resource name, is that the push secret contains the label
`argocd.argoproj.io/secret-type: repository-write`, which causes the Secret to be used for pushing manifests to git
instead of pulling from git. Argo CD requires different secrets for pushing and pulling to provide better isolation.

Once your secrets are installed, set the `spec.sourceHydrator` field of the Application. For example:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
spec:
  sourceHydrator:
    drySource:
      repoURL: https://github.com/argoproj/argocd-example-apps
      path: helm-guestbook
      targetRevision: HEAD
    syncSource:
      targetBranch: environments/dev
      path: helm-guestbook
```

In this example, the hydrated manifests will be pushed to the `environments/dev` branch of the `argocd-example-apps`
repository. The `drySource` field tells Argo CD where your original, unrendered configuration lives.
This can be a Helm chart, a Kustomize directory, or plain manifests. Argo CD reads this source, renders the final Kubernetes
manifests from it, and then writes those hydrated manifests into the location specified by `syncSource.path`.

When using source hydration, the `syncSource.path` field is required and must always point to a non-root
directory in the repository. Setting the path to the repository root (for eg. `"."` or `""`) is not
supported. This ensures that hydration is always scoped to a dedicated subdirectory, which avoids unintentionally overwriting or removing files that may exist in the repository root.

During each hydration run, Argo CD cleans the application's configured path before writing out newly generated manifests. This guarantees that old or stale files from previous hydration do not linger in the output directory. However, the repository root is never cleaned, so files such as CI/CD configuration, README files, or other root-level assets remain untouched.

It is important to note that hydration only cleans the currently configured application path. If an application’s path changes, the old directory is not removed automatically. Likewise, if an application is deleted, its output path remains in the repository and must be cleaned up manually by the repository owner if desired. This design is intentional: it prevents accidental deletion of files when applications are restructured or removed, and it protects critical files like CI pipelines that may coexist in the repository.

> [!NOTE] 
> The hydrator triggers only when a new commit is detected in the dry source.  
> Adding or removing Applications does not on its own cause hydration to run.  
> If the set of Applications changes but the dry-source commit does not, hydration will wait until the next commit.  

> [!IMPORTANT]
> **Project-Scoped Repositories**
>

    Repository Secrets may contain a `project` field, making the secret only usable by Applications in that project.
    The source hydrator only supports project-scoped repositories if all Applications writing to the same repository and
    branch are in the same project. If Applications in different projects write to the same repository and branch, the
    source hydrator will not be able to use a project-scoped repository secret and will require a global repository 
    secret. This behavior may change in the future.

If there are multiple repository-write Secrets available for a repo, the source hydrator will non-deterministically
select one of the matching Secrets and log a warning saying "Found multiple credentials for repoURL".

## Source Configuration Options

The source hydrator supports various source types through inline configuration options in the `drySource` field. This allows you to use Helm charts, Kustomize applications, directories, and plugins with environment-specific configurations.

### Helm Charts

You can use Helm charts by specifying the `helm` field in the `drySource`:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-helm-app
spec:
  sourceHydrator:
    drySource:
      repoURL: https://github.com/argoproj/argocd-example-apps
      path: helm-guestbook
      targetRevision: HEAD
      helm:
        valueFiles:
          - values-prod.yaml
        parameters:
          - name: image.tag
            value: v1.2.3
        releaseName: my-release
    syncSource:
      targetBranch: environments/prod
      path: helm-guestbook-hydrated
```

### Kustomize Applications

For Kustomize applications, use the `kustomize` field:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-kustomize-app
spec:
  sourceHydrator:
    drySource:
      repoURL: https://github.com/argoproj/argocd-example-apps
      path: kustomize-guestbook
      targetRevision: HEAD
      kustomize:
        namePrefix: prod-
        nameSuffix: -v1
        images:
          - gcr.io/heptio-images/ks-guestbook-demo:0.2
    syncSource:
      targetBranch: environments/prod
      path: kustomize-guestbook-hydrated
```

### Directory Applications

For plain directory applications with specific options, use the `directory` field:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-directory-app
spec:
  sourceHydrator:
    drySource:
      repoURL: https://github.com/argoproj/argocd-example-apps
      path: guestbook
      targetRevision: HEAD
      directory:
        recurse: true
    syncSource:
      targetBranch: environments/prod
      path: guestbook-hydrated
```

### Config Management Plugins

You can also use Config Management Plugins by specifying the `plugin` field:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-plugin-app
spec:
  sourceHydrator:
    drySource:
      repoURL: https://github.com/argoproj/argocd-example-apps
      path: my-plugin-app
      targetRevision: HEAD
      plugin:
        name: my-custom-plugin
        env:
          - name: ENV_VAR
            value: prod
    syncSource:
      targetBranch: environments/prod
      path: my-plugin-app-hydrated
```

!!! note "Feature Parity"
    The source hydrator supports the same configuration options as the regular Application source field. You can use any combination of these source types with their respective configuration options to match your application's needs.

## Pushing to a "Staging" Branch

The source hydrator can be used to push hydrated manifests to a "staging" branch instead of the `syncSource` branch.
This provides a way to prevent the hydrated manifests from being applied to the cluster until some prerequisite
conditions are met (in effect providing a way to handle environment promotion via Pull Requests).

To use the source hydrator to push to a "staging" branch, set the `spec.sourceHydrator.hydrateTo` field of the
Application. For example:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
spec:
  project: my-project
  destination:
    server: https://kubernetes.default.svc
    namespace: default
  sourceHydrator:
    drySource:
      repoURL: https://github.com/argoproj/argocd-example-apps
      path: helm-guestbook
      targetRevision: HEAD
    syncSource:
      targetBranch: environments/dev
      path: helm-guestbook
    hydrateTo:
      targetBranch: environments/dev-next
```

In this example, the hydrated manifests will be pushed to the `environments/dev-next` branch, and Argo CD will not sync
the changes until something moves them to the `environments/dev` branch.

You could use a CI action to move the hydrated manifests from the `hydrateTo` branch to the `syncSource` branch. To
introduce a gating mechanism, you could require a Pull Request to be opened to merge the changes from the `hydrateTo`
branch to the `syncSource` branch.

Argo CD will only push changes to the `hydrateTo` branch, it will not create a PR or otherwise facilitate moving those 
changes to the `syncSource` branch. You will need to use your own tooling to move the changes from the `hydrateTo` 
branch to the `syncSource` branch.

## Pushing to a Different Repository

The source hydrator supports hydrating manifests to a separate repository, not just a different branch in the same repository. This allows you to keep your dry source repository clean and push rendered manifests to dedicated GitOps repositories.

To hydrate to a different repository, set the `repoURL` field in the `syncSource` configuration:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
spec:
  project: my-project
  destination:
    server: https://kubernetes.default.svc
    namespace: default
  sourceHydrator:
    drySource:
      repoURL: https://github.com/org/gitops-manifests
      path: apps/my-app
      targetRevision: HEAD
    syncSource:
      repoURL: https://github.com/org/gitops-manifests-hydrated
      targetBranch: main
      path: apps/my-app
```

In this example:
- The **DRY source** (unrendered configuration like Helm charts or Kustomize) is in the `gitops-manifests` repository
- The **hydrated manifests** are pushed to the `gitops-manifests-hydrated` repository on the `main` branch
- Argo CD **syncs from** the `gitops-manifests-hydrated` repository - this is the same location where hydrated manifests are pushed

The `syncSource.repoURL` field specifies where hydrated manifests should be pushed to AND where Argo CD will sync from. If `syncSource.repoURL` is not set, it defaults to `drySource.repoURL` (same repository, different branch).

### Benefits of Using Separate Repositories

Using different repositories for dry sources and hydrated manifests provides several advantages:

- **Cross-platform flexibility**: You can use different Git providers for each repository. For example, keep your dry source (Helm charts, Kustomize) in GitHub while pushing hydrated manifests to GitLab, Bitbucket, or any other Git provider. This is useful when teams have different tooling preferences or organizational requirements. Or simply as Recovery/Failover

- **Simpler permission management**: Managing permissions at the repository level is often easier than managing branch-level permissions. Many Git providers offer more granular and straightforward access controls at the repository level, making it simpler to enforce who can read or write to specific repositories.

- **Enhanced security for dry sources**: Since Argo CD only needs **read** credentials for the dry source repository and **write** credentials for the hydrated manifests repository, there is no risk of accidentally overwriting the main branch or any other content in your dry source repository. The dry source repository remains protected because Argo CD simply does not have write access to it.

- **Clear separation of concerns**: Keeping rendered manifests in a dedicated repository makes it obvious which repository contains the "source of truth" (dry manifests) versus the generated output (hydrated manifests). This separation can simplify auditing and compliance workflows.

- **No branch drift**: When using branches within the same repository, the hydrated branch continuously diverges from the main branch over time. It accumulates commits that the main branch doesn't have (hydration commits) while also falling behind on commits from the main branch. This creates an increasingly messy git history with branches that are both "ahead" and "behind" by growing numbers of commits. Using separate repositories eliminates this problem entirely - each repository maintains a clean, linear history for its specific purpose.


> [!NOTE]
> When using different repositories, make sure that:
> - All repositories are permitted in your AppProject
> - You have write credentials configured for the destination repository (the one specified in `syncSource.repoURL`)
> - You have read credentials configured for both the dry source and sync source repositories

## Commit Tracing

It's common for CI or other tooling to push DRY manifest changes after a code change. It's important for users to be
able to trace the hydrated commits back to the original code change that caused the hydration.

Source Hydrator makes use of some custom git commit trailers to facilitate this tracing. A CI job that builds an image
and pushes an image bump to DRY manifests can use the following commit trailers to link the hydrated commit to the
code commit.

```shell
git commit -m "Bump image to v1.2.3" \
  # Must be an RFC 5322 name
  --trailer "Argocd-reference-commit-author: Author Name <author@example.com>" \
  # Must be a hex string 5-40 characters long
  --trailer "Argocd-reference-commit-sha: <code-commit-sha>" \
  # The subject is the first line of the commit message. It cannot contain newlines.
  --trailer "Argocd-reference-commit-subject: Commit message of the code commit" \
   # The body must be a valid JSON string, including opening and closing quotes
  --trailer 'Argocd-reference-commit-body: "Commit message of the code commit\n\nSigned-off-by: Author Name <author@example.com>"' \
   # The repo URL must be a valid URL
  --trailer "Argocd-reference-commit-repourl: https://git.example.com/owner/repo" \
  # The date must by in ISO 8601 format
  --trailer "Argocd-reference-commit-date: 2025-06-09T13:50:18-04:00" 
```

> [!NOTE]
> The commit trailers must not contain newlines. 

So the full CI script might look something like this:

```shell
# Clone code repo
git clone https://git.example.com/owner/repo.git
cd repo

# Build the image and get the new image tag
# <custom build logic here>

# Get the commit information
author=$(git show -s --format="%an <%ae>")
sha=$(git rev-parse HEAD)
subject=$(git show -s --format='%s')
body=$(git show -s --format='%b')
jsonbody=$(jq -n --arg body "$body" '$body')
repourl=$(git remote get-url origin)
date=$(git show -s --format='%aI')

# Clone the dry source repo
git clone https://git.example.com/owner/deployment-repo.git
cd deployment-repo

# Bump the image in the dry manifests
# <custom bump logic here, e.g. `kustomize edit`>

# Commit the changes with the commit trailers
git commit -m "Bump image to v1.2.3" \
  --trailer "Argocd-reference-commit-author: $author" \
  --trailer "Argocd-reference-commit-sha: $sha" \
  --trailer "Argocd-reference-commit-subject: $subject" \
  --trailer "Argocd-reference-commit-body: $jsonbody" \
  --trailer "Argocd-reference-commit-repourl: $repourl" \
  --trailer "Argocd-reference-commit-date: $date"
```

The commit metadata will appear in the hydrated commit's root hydrator.metadata file:

```json
{
  "author": "CI <ci@example.com>",
  "subject": "chore: bump image to b82add2",
  "date": "2025-06-09T13:50:08-04:00",
  "body": "Signed-off-by: CI <ci@example.com>\n",
  "drySha": "6cb951525937865dced818bbdd78c89b2d2b3045",
  "repoURL": "https://git.example.com/owner/manifests-repo",
  "references": [
    {
      "commit": {
        "author": {
          "name": "Author Name",
          "email": "author@example.com"
        },
        "sha": "b82add298aa045d3672880802d5305c5a8aaa46e",
        "subject": "chore: make a change",
        "body": "make a change\n\nSigned-off-by: Author Name <author@example.com>",
        "repoURL": "https://git.example.com/owner/repo",
        "date": "2025-06-09T13:50:18-04:00"
      }
    }
  ]
}
```

The top-level "body" field contains the commit message of the DRY commit minus the subject line and any 
`Argocd-reference-commit-*` trailers that were used in `references`. Unrecognized or invalid trailers are preserved in
the body.

Although `references` is an array, the source hydrator currently only supports a single related commit. If a trailer is
specified more than once, the last one will be used.

All trailers are optional. If a trailer is not specified, the corresponding field in the metadata will be omitted.

## Commit Message Template

The commit message is generated using a [Go text/template](https://pkg.go.dev/text/template), optionally configured by the user via the argocd-cm ConfigMap. The template is rendered using the values from `hydrator.metadata`. The template can be multi-line, allowing users to define a subject line, body and optional trailers. To define the commit message template, you need to set the `sourceHydrator.commitMessageTemplate` field in argocd-cm ConfigMap.

The template may functions from the [Sprig function library](https://github.com/Masterminds/sprig).

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  sourceHydrator.commitMessageTemplate: |
    {{.metadata.drySha | trunc 7}}: {{ .metadata.subject }}
    {{- if .metadata.body }}
    
    {{ .metadata.body }}
    {{- end }}
    {{ range $ref := .metadata.references }}
    {{- if and $ref.commit $ref.commit.author }}
    Co-authored-by: {{ $ref.commit.author }}
    {{- end }}
    {{- end }}
    {{- if .metadata.author }}
    Co-authored-by: {{ .metadata.author }}
    {{- end }}
```

### Credential Templates

Credential templates allow a single credential to be used for multiple repositories. The source hydrator supports credential templates. For example, if you setup credential templates for the URL prefix `https://github.com/argoproj`, these credentials will be used for all repositories with this URL as prefix (e.g. `https://github.com/argoproj/argocd-example-apps`) that do not have their own credentials configured.
For more information please refer [credential-template](private-repositories.md#credential-templates). 
An example of repo-write-creds secret.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: private-repo
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repo-write-creds
stringData:
  type: git
  url: https://github.com/argoproj
  password: my-password
  username: my-username
```

## Limitations

### Signature Verification

The source hydrator **does not currently support signature verification of the DRY sources it hydrates/commits**. It
also does not sign the commits it pushes to git, so if signature verification is enabled, the commits will fail
verification when Argo CD attempts to sync the hydrated manifests.

### Project-Scoped Push Secrets

If all the Applications for a given destination repo/branch are under the same project, then the hydrator will use any
available project-scoped push secrets. If two Applications for a given repo/branch are in different projects, then the
hydrator will not be able to use a project-scoped push secret and will require a global push secret.

### `manifest-generate-paths` Annotation Support

The source hydrator does not currently support the [manifest-generate-paths annotation](../operator-manual/high_availability.md#manifest-paths-annotation) 
for work avoidance on hydration of dry commits. In other words, the source hydrator is not able to skip hydration of dry 
commits that have not changed relevant files.

The application controller _does_ honor the `manifest-generate-paths` annotation when syncing the hydrated manifests.
So if your application hydrates to the `foo` directory, and the `manifest-generate-paths` annotation is set to `foo`, 
then the application controller will not re-hydrate the manifests after a commit that only affects files in the `bar`
directory.

## Prerequisites

### Handle Secrets on the Destination Cluster

Do not use the source hydrator with any tool that injects secrets into your manifests as part of the hydration process
(for example, Helm with SOPS or the Argo CD Vault Plugin). These secrets would be committed to git. Instead, use a
secrets operator that populates the secret values on the destination cluster.

## Best Practices

### Make Hydration Deterministic

The source hydrator should be deterministic. For a given dry source commit, the hydrator should always produce the same
hydrated manifests. This means that the hydrator should not rely on external state or configuration that is not stored
in git.

Examples of non-deterministic hydration:

* A Helm chart using unpinned dependencies
* A Helm chart is using a non-deterministic template function such as `randAlphaNum` or `lookup`
* [Config Management Plugins](../operator-manual/config-management-plugins.md) which retrieve non-git state, such as secrets
* Kustomize manifests referencing unpinned remote bases

### Enable Branch Protection

Argo CD should be the only thing pushing hydrated manifests to the hydrated branches. To prevent other tools or users
from pushing to the hydrated branches, enable branch protection in your SCM.

It is best practice to prefix the hydrated branches with a common prefix, such as `environments/`. This makes it easier
to configure branch protection rules on the destination repository.

> [!NOTE]
> To maintain reproducibility and determinism in the Hydrator’s output,
> Argo CD-specific metadata (such as `argocd.argoproj.io/tracking-id`) is
> not written to Git during hydration. These annotations are added dynamically
> during application sync and comparison.
