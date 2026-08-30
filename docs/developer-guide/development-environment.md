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

> [!WARNING]
> **Windows users:** The local development workflow is designed primarily for Linux. If you are using Windows, run **all commands in this guide, including the commands that create or delete clusters, from inside WSL 2**. Do not create the Kind cluster from PowerShell or Command Prompt and then run the Argo CD commands from WSL 2.
>
> Docker Desktop must be running and configured to use the WSL 2 engine, with your WSL 2 distribution enabled under Docker Desktop's WSL integration settings. The following additional steps allow the Docker container used by Argo CD's virtualized toolchain to reach a Kind API server created in WSL 2.

#### Windows and WSL 2 networking setup

Run these commands from your WSL 2 shell, in the root of the Argo CD repository:

1. Create `kind-config.yaml` with the API server listening on all interfaces:

   ```yaml
   kind: Cluster
   apiVersion: kind.x-k8s.io/v1alpha4
   networking:
     apiServerAddress: "0.0.0.0"
   ```

2. Delete any existing Kind cluster and create a new one using this configuration:

   ```shell
   kind delete cluster
   kind create cluster --config kind-config.yaml
   ```

3. Find the WSL 2 gateway address. You will use this address in the next step:

   ```shell
   ip route show | grep default | awk '{print $3}'
   ```

4. Back up the WSL 2 kubeconfig and create a container-specific copy. Replace `<WSL_GATEWAY_IP>` with the address from the previous command:

   ```shell
   cp ~/.kube/config ~/.kube/config.local
   cp ~/.kube/config ~/.kube/config.container
   sed -i 's/0.0.0.0:6443/<WSL_GATEWAY_IP>:6443/g' ~/.kube/config.container
   sed -i '/certificate-authority-data/d' ~/.kube/config.container
   sed -i '/server:/a \    insecure-skip-tls-verify: true' ~/.kube/config.container
   cp ~/.kube/config.container ~/.kube/config
   ```

   The certificate authority data is removed because the API server is now reached through the WSL 2 gateway address rather than its original host name. The client certificate and key remain in the kubeconfig so Kubernetes can authenticate the request.

5. Verify that the virtualized toolchain can reach the cluster:

   ```shell
   make verify-kube-connect
   ```

6. Restore your normal kubeconfig immediately after verification:

   ```shell
   cp ~/.kube/config.local ~/.kube/config
   ```

   Repeat steps 4 through 6 whenever you need to run a Make target that accesses the Kind cluster from a Docker container. Keep `~/.kube/config.container` private because it disables TLS certificate verification.

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
kubectl apply -n argocd --server-side --force-conflicts -f https://raw.githubusercontent.com/argoproj/argo-cd/master/manifests/install.yaml
```

Set kubectl config to avoid specifying the namespace in every kubectl command.  

```shell
kubectl config set-context --current --namespace=argocd
```

