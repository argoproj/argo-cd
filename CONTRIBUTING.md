## Requirements
Make sure you have following tools installed [golang](https://golang.org/), [dep](https://github.com/golang/dep), [protobuf](https://developers.google.com/protocol-buffers/),
[kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/).

```
$ brew install go dep protobuf kubectl
$ go get -u github.com/golang/protobuf/protoc-gen-go
```

Nice to have [gometalinter](https://github.com/alecthomas/gometalinter) and [goreman](https://github.com/mattn/goreman):

```
$ go get -u gopkg.in/alecthomas/gometalinter.v2 github.com/mattn/goreman && gometalinter.v2 --install
```

## Building

```
$ go get -u github.com/argoproj/argo-cd
$ dep ensure
$ make
```

## Running locally

You need to have access to kubernetes cluster (including [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/) or [docker edge](https://docs.docker.com/docker-for-mac/install/) ) in order to run Argo CD on your laptop:

* install kubectl: `brew install kubectl`
* make sure `kubectl` is connected to your cluster (e.g. `kubectl get pods` should work).
* install application CRD using following command:

```
$ kubectl create -f install/manifests/01_application-crd.yaml
```

* start Argo CD services using [goreman](https://github.com/mattn/goreman):

```
$ goreman start
```
