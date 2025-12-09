# Setting Up the Development Environment

## Required Tools Overview

You will need to install the following tools with the specified minimum versions:

* Git (v2.0.0+)
* Go (version specified in `go.mod` - check with `go version`)
* Docker (v20.10.0+) Or Podman (v3.0.0+)
* Kind (v0.11.0+) Or Minikube (v1.23.0+) Or K3d (v5.7.3+)



## Install Required Tools

### Install Git

Obviously, you will need a `git` client for pulling source code and pushing back your changes.

<https://github.com/git-guides/install-git>


### Install Go

You will need a Go SDK and related tools (such as GNU `make`) installed and working on your development environment.

<https://go.dev/doc/install/>

Install Go with a version equal to or greater than the version listed in `go.mod` (verify go version with `go version`).  
We will assume that your Go workspace is at `~/go`.

Verify: run `go version`

### Install Docker or Podman

#### Installation guide for docker

<https://docs.docker.com/engine/install/>

You will need a working Docker runtime environment, to be able to build and run images. Argo CD is using multi-stage builds. 

Verify: run `docker version`

#### Installation guide for podman

<https://podman.io/docs/installation>

### Install a Local K8s Cluster

You won't need a fully blown multi-master, multi-node cluster, but you will need something like K3S, K3d, Minikube, Kind or microk8s. You will also need a working Kubernetes client (`kubectl`) configuration in your development environment. The configuration must reside in `~/.kube/config`.

#### Kind

##### [Installation guide](https://kind.sigs.k8s.io/docs/user/quick-start)

You can use `kind` to run Kubernetes inside Docker. But pointing to any other development cluster works fine as well as long as Argo CD can reach it.

##### Start the Cluster
```shell
kind create cluster
```

#### Minikube

##### [Installation guide](https://minikube.sigs.k8s.io/docs/start)

##### Start the Cluster
```shell
minikube start
```

Or, if you are using minikube with podman driver:

```shell
minikube start --driver=podman
```

#### K3d

##### [Installation guide](https://k3d.io/stable/#quick-start)

### Verify cluster installation

* Run `kubectl version` 

## Fork and Clone the Repository
1. Fork the Argo CD repository to your personal GitHub Account
2. Clone the forked repository:
```shell
git clone https://github.com/YOUR-USERNAME/argo-cd.git
```
   Please note that the local build process uses GOPATH and that path should not be used, unless the Argo CD repository was directly cloned in it.

3. While everyone has their own Git workflow, the author of this document recommends to create a remote called `upstream` in your local copy pointing to the original Argo CD repository. This way, you can easily keep your local branches up-to-date by merging in latest changes from the Argo CD repository, i.e. by doing a `git pull upstream master` in your locally checked out branch.
   To create the remote, run:
   ```shell
   cd argo-cd
   git remote add upstream https://github.com/argoproj/argo-cd.git
   ```

## Install Additional Required Development Tools

```shell
make install-go-tools-local
make install-codegen-tools-local
```

## Install Latest Argo CD on Your Local Cluster

```shell
kubectl create namespace argocd &&
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/master/manifests/install.yaml
```

Set kubectl config to avoid specifying the namespace in every kubectl command.  

```shell
kubectl config set-context --current --namespace=argocd
```

