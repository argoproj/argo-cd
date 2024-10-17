# Source Hydrator

**Current feature state**: Alpha

Tools like Helm and Kustomize allow users to express their Kubernetes manifests in a more concise and reusable way
(keeping it DRY - Don't Repeat Yourself). However, these tools can obscure the actual Kubernetes manifests that are
applied to the cluster.

The "rendered manifest pattern" is a way to push the hydrated manifests to git before syncing them to the cluster. This
allows users to see the actual Kubernetes manifests that are applied to the cluster.

The source hydrator is a feature of Argo CD that allows users to push the hydrated manifests to git before syncing them
to the cluster.

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

!!! important
    After updating the ConfigMap, you must restart the Argo CD controller for the changes to take effect.

If you are using one of the `*-install.yaml` manifests to install Argo CD, you can use the 
`*-install-with-hydrator.yaml` version of that file instead.

For example,

```
Without hydrator: https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
With hydrator:    https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install-with-hydrator.yaml
```

!!! important
    The `*-with-hydrator-install.yaml` manifests will eventually be removed when the source hydrator is either enabled
    by default or removed. The upgrade guide will note if the `*-with-hydrator-install.yaml` manifests are no longer
    available.

## Using the Source Hydrator

To use the source hydrator, you must first install a push secret.

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
  url: "https://github.com"
  type: "git"
  githubAppID: "<your app ID here>"
  githubAppInstallationID: "<your installation ID here>"
  githubAppPrivateKey: |
    <your private key here>
```
For now, the source hydrator only supports GitHub Apps. To use the source hydrator, you must first
[create a GitHub App](https://github.com/settings/apps/new) with read/write permissions and install it in the repository
you want to use it in. After you install the app, the installation ID will appear in the URL. The private key is
generated when you create the GitHub App.
Once your push secret is installed, set the `spec.sourceHydrator` field of the Application. For example:

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
repository.

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
* A Helm chart is using a template function such as `randAlphaNum`
* [Config Management Plugins](../operator-manual/config-management-plugins.md) which retrieve non-git state, such as secrets
* Kustomize manifests referencing unpinned remote bases
* Kustomize manifests which use generated names, such as the configMapGenerator

### Enable Branch Protection

Argo CD should be the only thing pushing hydrated manifests to the hydrated branches. To prevent other tools or users
from pushing to the hydrated branches, enable branch protection in your SCM.

It is best practice to prefix the hydrated branches with a common prefix, such as `environment/`. This makes it easier
to configure branch protection rules on the destination repository.
