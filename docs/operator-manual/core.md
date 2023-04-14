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
  another CLI to automate my deployments. I want instead rely in
  Kubernetes API only.
- As a cluster admin, I don't want to provide Argo CD UI or Argo CD
  CLI to developers.

## Architecture

Because Argo CD is designed with a component based architecture in
mind, it is possible to have a more minimalist installation. In this
case fewer components are installed and yet the main gitops
functionality remains operational.

The diagram below shows the components that will be installed while
opting for Argo CD Core:

![Argo CD Core](../assets/argocd-core-components.png)

## Installing

Argo CD Core can be installed by applying a single manifest file that
contains all the required resources.

Example:

```
export ARGOCD_VERSION=<desired argo cd release version>
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/$ARGOCD_VERSION/manifests/core-install.yaml
```

## Using

Once Argo CD Core is installed, users will be able to interact with it
by relying on Gitops. The available Kubernetes resources will be the
Application and the ApplicationSet CRDs. By using those resources,
users will be able to deploy and manage applications in Kubernetes.

It is still possible to use Argo CD CLI even when running Argo CD
Core. In this case, the CLI will spawn a local API server process that
will be used to handle the CLI command. Once the command is concluded,
the local API Server process will also be terminated. This happens
transparently for the user with no additional command required. Note
that Argo CD Core will rely only on Kubernetes RBAC and the user (or
the process) invoking the CLI needs to have access to the Argo CD
namespace with the proper permission in the Application and
ApplicationSet resources for executing a given command.

In order to use Argo CD CLI in core mode it is necessary to pass a
special flag `--core` in the login command.

Example:

```bash
kubectl config set-context --current --namespace=argocd # change current kube context to argocd namespace
argocd login --core
```

Similarly, users can also run the Web UI locally if they prefer to
interact with Argo CD using this method. The Web UI can be started
locally by running the following command:

```
argocd admin dashboard -n argocd
```

Argo CD API will be available at `http://localhost:8080`

