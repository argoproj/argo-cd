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
brew install go git-lfs kubectl kubectx dep ksonnet/tap/ks kubernetes-helm kustomize
```

Check the versions:

```bash
go version ;# must be v1.12.x
helm version ;# must be v2.13.x
kustomize version ;# must be v3.1.x
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
dep ensure
make dev-tools-image
make install-lint-tools
go get github.com/mattn/goreman
go get github.com/jstemmer/go-junit-report
```

Common make targets:

* `make codegen` - Run code generation
* `make lint` - Lint code
* `make test` - Run unit tests
* `make cli` - Make the `argocd` CLI tool

Check out the following [documentation](https://github.com/argoproj/argo-cd/blob/master/docs/developer-guide/test-e2e.md) for instructions on running the e2e tests.

## Running Locally

It is much easier to run and debug if you run ArgoCD on your local machine than in the Kubernetes cluster.

You should scale the deployments to zero:

```bash
kubectl -n argocd scale deployment/argocd-application-controller --replicas 0
kubectl -n argocd scale deployment/argocd-dex-server --replicas 0
kubectl -n argocd scale deployment/argocd-repo-server --replicas 0
kubectl -n argocd scale deployment/argocd-server --replicas 0
kubectl -n argocd scale deployment/argocd-redis --replicas 0
```

Download Yarn dependencies and Compile:

```bash
~/go/src/github.com/argoproj/argo-cd/ui
yarn install
yarn build
```

Then start the services:

```bash
cd ~/go/src/github.com/argoproj/argo-cd
make start
```

You can now execute `argocd` command against your locally running ArgoCD by appending `--server localhost:8080 --plaintext --insecure`, e.g.:

```bash
argocd app create guestbook --path guestbook --repo https://github.com/argoproj/argocd-example-apps.git --dest-server https://kubernetes.default.svc  --dest-namespace default --server localhost:8080 --plaintext --insecure
```

You can open the UI: [http://localhost:4000](http://localhost:4000)

As an alternative to using the above command line parameters each time you call `argocd` CLI, you can set the following environment variables:

```bash
export ARGOCD_SERVER=127.0.0.1:8080
export ARGOCD_OPTS="--plaintext --insecure"
```

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

If you don't want to use `latest` as the image's tag (the default), you can set it from the environment too:

```bash
export IMAGE_TAG=yourtag
```

Build the image:

```bash
DOCKER_PUSH=true make image
```

Update the manifests (be sure to do that from a shell that has above environment variables set)

```bash
make manifests
```

Install the manifests:

```bash
kubectl -n argocd apply --force -f manifests/install.yaml
```

Scale your deployments up:

```bash
kubectl -n argocd scale deployment/argocd-application-controller --replicas 1
kubectl -n argocd scale deployment/argocd-dex-server --replicas 1
kubectl -n argocd scale deployment/argocd-repo-server --replicas 1
kubectl -n argocd scale deployment/argocd-server --replicas 1
kubectl -n argocd scale deployment/argocd-redis --replicas 1
```

Now you can set-up the port-forwarding and open the UI or CLI.
