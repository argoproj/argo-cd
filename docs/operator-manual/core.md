# Argo CD Core

## Introduction

Argo CD Core is mainly a different installation that makes Argo CD run
in headless mode. With this installation you will have a fully
functional gitops engine capable of getting the desired state from Git
repositories and applying it in Kubernetes.

The following groups of features won't be available in this
installation:

- Argo CD RBAC model
- Argo CD API
- OIDC based authentication
- Argo CD web application
- Argo CD CLI
- Multi-tenancy

A few use-cases that justify running Argo CD core are:

- As a cluster admin, I want to rely on Kubernetes RBAC only.
- As a devops engineer, I don't want to learn a new API or depend on
  another CLI to automate my deployments. I wan't instead rely in
  Kubernetes API only.
- As a cluster admin, I don't want to provide Argo CD UI or Argo CD
  CLI to developers.

## Architecture

Because Argo CD is designed with a component based architecture in
mind, it is possible to have a more minimalistic installation. In this
case fewer components are installed and yet the main gitops
functinality remains operational.

The diagram below shows the components that will be installed while
opting for Argo CD Core:

![Argo CD Core](../assets/argocd-core-components.png)

## Installing

Argo CD Core can be installed by applying a single manifest file.
Example:

```
export ARGOCD_VERSION=<desired argo cd release version>
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/$ARGOCD_VERSION/manifests/core-install.yaml
```

It can also be installed by referencing the Kustomize project found
in:
https://github.com/argoproj/argo-cd/blob/master/manifests/core-install/kustomization.yaml

Example:

```
kubectl create namespace argocd
kustomize build https://github.com/argoproj/argo-cd//manifests/core-install | kubectl apply -n argocd -f -
```

Note that in this case it will always install the latest version. The
recomended approach in this case is to define a Kustomize overlay that
defines the desired version.

Example:

```

```

## Using

The end-users need Kubernetes access to manage Argo CD. The `argocd` CLI has to be configured using the following commands:

```bash
kubectl config set-context --current --namespace=argocd # change current kube context to argocd namespace
argocd login --core
```

The Web UI is also available and can be started using the `argocd admin dashboard` command.


