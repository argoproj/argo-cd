# Contributing
## Before You Start

You must install and run the ArgoCD using a local Kubernetes (e.g. Docker for Desktop or Minikube) first. This will help you understand the application, but also get your local environment set-up.

Then, to get a good grounding in Go, try out [the tutorial](https://tour.golang.org/).

## Pre-requisites

Install:

* [docker](https://docs.docker.com/install/#supported-platforms)
* [git](https://git-scm.com/) and [git-lfs](https://git-lfs.github.com/)
* [golang](https://golang.org/)
* [dep](https://github.com/golang/dep)
* [ksonnet](https://github.com/ksonnet/ksonnet#install)
* [helm](https://github.com/helm/helm/releases)
* [kustomize](https://github.com/kubernetes-sigs/kustomize/releases)
* [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
* [kubectx](https://kubectx.dev)
* [minikube](https://kubernetes.io/docs/setup/minikube/) or Docker for Desktop

Brew users can quickly install the lot:
    
```bash
brew install git-lfs go dep kubectl kubectx ksonnet/tap/ks kubernetes-helm kustomize 
```

Set up environment variables (e.g. is `~/.bashrc`):

```bash
export GOPATH=~/go
export PATH=$PATH:$GOPATH/bin
```

Checkout the code:

```bash
go get -u github.com/argoproj/argo-cd
cd ~/go/src/github.com/argoproj/argo-cd
```

## Building

Ensure dependencies are up to date first:

```shell
make dep
```

Build `cli`, `image`, and `argocd-util` as default targets by running make:

```bash
make
```

The make command can take a while, and we recommend building the specific component you are working on

* `make codegen` - Builds protobuf and swagger files.

Note: `make codegen` is slow because it uses docker + volume mounts. To improve performance you might install binaries from `./hack/Dockerfile.dev-tools`
and use `make codegen-local`. It is still recommended to run `make codegen` once before sending PR to make sure correct version of codegen tools is used.   

* `make cli` - Make the argocd CLI tool
* `make server` - Make the API/repo/controller server
* `make argocd-util` - Make the administrator's utility, used for certain tasks such as import/export

## Running Tests

To run unit tests:

```bash
make test
```

Check out the following [documentation](https://github.com/argoproj/argo-cd/blob/master/docs/developer-guide/test-e2e.md) for instructions on running the e2e tests.

## Running Locally

It is much easier to run and debug if you run ArgoCD on your local machine than in the Kubernetes cluster.

You should scale the deployments to zero:

```bash
kubectl -n argocd scale deployment.extensions/argocd-application-controller --replicas 0
kubectl -n argocd scale deployment.extensions/argocd-dex-server --replicas 0
kubectl -n argocd scale deployment.extensions/argocd-repo-server --replicas 0
kubectl -n argocd scale deployment.extensions/argocd-server --replicas 0
kubectl -n argocd scale deployment.extensions/argocd-redis --replicas 0
```

Note: you'll need to use the https://localhost:6443 cluster now.

Then start the services:

```bash
cd ~/go/src/github.com/argoproj/argo-cd
make start
```

You can now execute `argocd` command against your locally running ArgoCD by appending `--server localhost:8080 --plaintext --insecure`, e.g.:

```bash
argocd app set guestbook --path guestbook --repo https://github.com/argoproj/argocd-example-apps.git --dest-server https://localhost:6443  --dest-namespace default --server localhost:8080 --plaintext --insecure
```

You can open the UI: http://localhost:8080

Note: you'll need to use the https://kubernetes.default.svc cluster now.

## Running Local Containers

You may need to run containers locally, so here's how:

Create login to Docker Hub, then login.

```bash
docker login
```

Add your username as the environment variable, e.g. to your `~/.bash_profile`:

```bash
export IMAGE_NAMESPACE=alexcollinsintuit
```

If you have not built the UI image (see [the UI README](https://github.com/argoproj/argo-cd/blob/master/ui/README.md)), then do the following:

```bash
docker pull argoproj/argocd-ui:latest
docker tag argoproj/argocd-ui:latest $IMAGE_NAMESPACE/argocd-ui:latest
docker push $IMAGE_NAMESPACE/argocd-ui:latest
```

Build the images:

```bash
DOCKER_PUSH=true make image
```

Update the manifests:

```bash
make manifests
```

Install the manifests:

```bash
kubectl -n argocd apply --force -f manifests/install.yaml
```

Scale your deployments up:

```bash
kubectl -n argocd scale deployment.extensions/argocd-application-controller --replicas 1
kubectl -n argocd scale deployment.extensions/argocd-dex-server --replicas 1
kubectl -n argocd scale deployment.extensions/argocd-repo-server --replicas 1
kubectl -n argocd scale deployment.extensions/argocd-server --replicas 1
kubectl -n argocd scale deployment.extensions/argocd-redis --replicas 1
```

Now you can set-up the port-forwarding and open the UI or CLI.
