## Before You Start

You must install and run the ArgoCD using a local Kubernetes (e.g. Docker for Desktop or Minikube) first. This will help you understand the application, but also get your local environment set-up.

Then, to get a good grounding in Go, try out [the tutorial](https://tour.golang.org/).

## Pre-requisites  

Install:

* [docker](https://docs.docker.com/install/#supported-platforms)
* [golang](https://golang.org/)
* [dep](https://github.com/golang/dep)
* [protobuf](https://developers.google.com/protocol-buffers/)
* [ksonnet](https://github.com/ksonnet/ksonnet#install)
* [helm](https://github.com/helm/helm/releases)
* [kustomize](https://github.com/kubernetes-sigs/kustomize/releases)
* [go-swagger](https://github.com/go-swagger/go-swagger/blob/master/docs/install.md)
* [jq](https://stedolan.github.io/jq/)
* [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/).
* [minikube](https://kubernetes.io/docs/setup/minikube/) or Docker for Desktop

```
brew tap go-swagger/go-swagger
brew install go dep protobuf kubectl ksonnet/tap/ks kubernetes-helm jq go-swagger 
```

Set up environment variables (e.g. is `~/.bashrc`):

```
export GOPATH=~/go
export PATH=$PATH:$GOPATH/bin
```

Install go dependencies:

```
go get -u github.com/golang/protobuf/protoc-gen-go
go get -u github.com/go-swagger/go-swagger/cmd/swagger
go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway
go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger
go get -u gopkg.in/alecthomas/gometalinter.v2 
go get -u github.com/mattn/goreman 

gometalinter.v2 --install
```

## Building

```
go get -u github.com/argoproj/argo-cd
dep ensure
make
```

The make command can take a while, and we recommend building the specific component you are working on

* `make codegen` - Builds protobuf and swagger files
* `make cli` - Make the argocd CLI tool
* `make server` - Make the API/repo/controller server
* `make argocd-util` - Make the administrator's utility, used for certain tasks such as import/export

## Running Tests

To run unit tests:

```
make test
```

To run e2e tests:

```
make test-e2e
```

## Running Locally

It is much easier to run and debug if you run ArgoCD on your local machine than in the Kubernetes cluster.

You should scale the deployemnts to zero:

```
kubectl -n argocd scale deployment.extensions/argocd-application-controller --replicas 0
kubectl -n argocd scale deployment.extensions/argocd-dex-server --replicas 0
kubectl -n argocd scale deployment.extensions/argocd-repo-server --replicas 0
kubectl -n argocd scale deployment.extensions/argocd-server --replicas 0
kubectl -n argocd scale deployment.extensions/argocd-redis --replicas 0
```

Then checkout and build the UI next to your code

```
cd ~/go/src/github.com/argoproj
git clone git@github.com:argoproj/argo-cd-ui.git
```

Follow the UI's [README](https://github.com/argoproj/argo-cd-ui/blob/master/README.md) to build it.

Note: you'll need to use the https://localhost:6443 cluster now.

Then start the services:

```
cd ~/go/src/github.com/argoproj/argo-cd
goreman start
```

You can now execute `argocd` command against your locally running ArgoCD by appending `--server localhost:8080 --plaintext --insecure`, e.g.:

```
argocd app set guestbook --path guestbook --repo https://github.com/argoproj/argocd-example-apps.git --dest-server https://localhost:6443  --dest-namespace default --server localhost:8080 --plaintext --insecure
```

You can open the UI: http://localhost:8080

Note: you'll need to use the https://kubernetes.default.svc cluster now.

## Running Local Containers

You may need to run containers locally, so here's how:

Create login to Docker Hub, then login.

```
docker login
```

Add your username as the environment variable, e.g. to your `~/.bash_profile`:

```
export IMAGE_NAMESPACE=alexcollinsintuit
```

If you have not built the UI image (see [the UI README](https://github.com/argoproj/argo-cd-ui/blob/master/README.md)), then do the following:

```
docker pull argoproj/argocd-ui:latest
docker tag argoproj/argocd-ui:latest $IMAGE_NAMESPACE/argocd-ui:latest
docker push $IMAGE_NAMESPACE/argocd-ui:latest
```

Build the images:

```
DOCKER_PUSH=true make image
```

Update the manifests:

```
make manifests
```

Install the manifests:

```
kubectl -n argocd apply --force -f manifests/install.yaml
```

Scale your deployments up:

```
kubectl -n argocd scale deployment.extensions/argocd-application-controller --replicas 1
kubectl -n argocd scale deployment.extensions/argocd-dex-server --replicas 1
kubectl -n argocd scale deployment.extensions/argocd-repo-server --replicas 1
kubectl -n argocd scale deployment.extensions/argocd-server --replicas 1
kubectl -n argocd scale deployment.extensions/argocd-redis --replicas 0
```

Now you can set-up the port-forwarding (see [README](README.md)) and open the UI or CLI.

## Pre-commit Checks

Before you commit, make sure you've formatted and linted your code, or your PR will fail CI:

```
STAGED_GO_FILES=$(git diff --cached --name-only | grep ".go$")

gofmt -w $STAGED_GO_FILES

make codgen
make precommit ;# lint and test
```
