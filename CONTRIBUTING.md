## Requirements

You must install and run the ArgoCD eusing miniubke first.

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
* [minikube](https://kubernetes.io/docs/setup/minikube/)

```
$ brew tap go-swagger/go-swagger
$ brew install go dep protobuf kubectl ksonnet/tap/ks kubernetes-helm jq go-swagger 
```

Set up environment variables (e.g. is `~/.bashrc`):

```
export GOPATH=~/go
export PATH=$PATH:$GOPATH/bin
```

Install go dependencies:

```
$ go get -u github.com/golang/protobuf/protoc-gen-go
$ go get -u github.com/go-swagger/go-swagger/cmd/swagger
$ go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway
$ go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger
$ go get -u gopkg.in/alecthomas/gometalinter.v2 
$ go get -u github.com/mattn/goreman 

$ gometalinter.v2 --install
```

## Building

```
$ go get -u github.com/argoproj/argo-cd
$ dep ensure
$ make
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

You should scale the deployemnts to zero via the Kubernetes dashboard, then start the services:

```
$ goreman start
```

You can now execute `argocd` command against your locally running ArgoCD by appending `--server localhost:8080 --plaintext --insecure`, e.g.:

```
app set guestbook --path guestbook --repo https://github.com/argoproj/argocd-example-apps.git --dest-server https://192.168.99.102:8443  --dest-namespace default --server localhost:8080 --plaintext --insecure
```

## Pre-commit Checks

Before you commit, make sure you've formatted and linted your code:

```
STAGED_GO_FILES=$(git diff --cached --name-only | grep ".go$")

gofmt -w $STAGED_GO_FILES

make codgen
make precommit ;# lint and test
```
