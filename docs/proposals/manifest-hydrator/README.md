# Argo CD Manifest Hydrator

Most Argo CD Applications don't directly use plain Kubernetes manifests. They reference a Helm chart or some Kustomize manifests, and then Argo CD transforms those sources into their final form (plain Kubernetes manifests).

Having Argo CD quietly do this transformation behind the scenes is convenient. But it can make it harder for developers to understand the full state of their application, both current and past. Hydrating (also known as "rendering") the sources and pushing the hydrated manifests to git is a common technique to preserve a full history of an Application's state.

Argo CD provides first-class tooling to hydrate manifests and push them to git. This document explains how to take advantage of that tooling.

## Setting up git Push Access

To use Argo CD's source hydration tooling, you have to grant Argo CD push access to all the repositories for apps using the source hydrator.

### Security Considerations

Argo CD stores git push secrets separately from the main Argo CD components and separately from git pull credentials to minimize the possibility of a malicious actor stealing the secrets or hijacking Argo CD components to push malicious changes.

Pushing hydrated manifests to git can improve security by ensuring that all state changes are stored and auditable. If a malicious actor does manage to produce malicious changes in manifests, those changes will be discoverable in git instead of living only in the live cluster state.

You should use your SCM's security mechanisms to ensure that Argo CD can only push to the allowed repositories and branches.

### Adding the Access Credentials

To set up push access, add a secret to the `argocd-push` namespace with the following format:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: argocd-example-apps
  labels:
    # Note that this is "repository-push" instead of "repository". The same secret should never be used for both push and pull access.
    argocd.argoproj.io/secret-type: repository-push
type: Opaque
stringData:
  url: https://github.com/argoproj/argocd-example-apps.git
  username: '****'
  password: '****'
```

Once the secret is available, any Application which has pull access to a given repo will be able to use the source hydration tooling to also push to that repo.

## Using the `sourceHydrator` Field

## Migrating from the `source` or `sources` Field
