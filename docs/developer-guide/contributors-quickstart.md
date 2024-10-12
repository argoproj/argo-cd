# Contributors Quick-Start 

This guide is a starting point for first-time contributors running Argo CD locally for the first time.

It skips advanced topics such as codegen, which are covered in the [running locally guide](running-locally.md)
and the [toolchain guide](toolchain-guide.md).

## Getting Started

### Install Go

<https://go.dev/doc/install/>

Install Go with a version equal to or greater than the version listed in `go.mod` (verify go version with `go version`). 

### Clone the Argo CD repo

```shell
mkdir -p $GOPATH/src/github.com/argoproj/ &&
cd $GOPATH/src/github.com/argoproj &&
git clone https://github.com/argoproj/argo-cd.git
```

### Install Docker

<https://docs.docker.com/engine/install/>

### Install or Upgrade a Tool for Running Local Clusters (e.g. kind or minikube)

#### Installation guide for kind:

<https://kind.sigs.k8s.io/docs/user/quick-start/>

#### Installation guide for minikube:

<https://minikube.sigs.k8s.io/docs/start/>

### Start Your Local Cluster

For example, if you are using kind:
```shell
kind create cluster
```

Or, if you are using minikube:

```shell
minikube start
```

### Install Argo CD

```shell
kubectl create namespace argocd &&
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/master/manifests/install.yaml
```

Set kubectl config to avoid specifying the namespace in every kubectl command.  
All following commands in this guide assume the namespace is already set.

```shell
kubectl config set-context --current --namespace=argocd
```

### Install `yarn`

<https://classic.yarnpkg.com/lang/en/docs/install/>

### Install `goreman`

<https://github.com/mattn/goreman#getting-started>

### Run Argo CD

```shell
cd argo-cd
make start-local ARGOCD_GPG_ENABLED=false
```

- Navigate to [localhost:4000](http://localhost:4000) in your browser to load the Argo CD UI
- It may take a few minutes for the UI to be responsive

!!! note
    If the UI is not working, check the logs from `make start-local`. The logs are `DEBUG` level by default. If the logs are
    too noisy to find the problem, try editing log levels for the commands in the `Procfile` in the root of the Argo CD repo.

## Making Changes

### UI Changes

Modifying the User-Interface (by editing .tsx or .scss files) auto-reloads the changes on port 4000.

### Backend Changes

Modifying the API server, repo server, or a controller requires restarting the current `make start-local` session to reflect the changes.

### CLI Changes

Modifying the CLI requires restarting the current `make start-local` session to reflect the changes.

To test most CLI commands, you will need to log in.

First, get the auto-generated secret:

```shell
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d; echo
```

Then log in using that password and username `admin`:

```shell
dist/argocd login localhost:8080
```

---
Congrats on making it to the end of this runbook! 🚀

For more on Argo CD, find us in Slack - <https://slack.cncf.io/> [#argo-contributors](https://cloud-native.slack.com/archives/C020XM04CUW)
