## Requirements
Make sure you have following tools installed
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

```
$ brew tap go-swagger/go-swagger
$ brew install go dep protobuf kubectl ksonnet/tap/ks kubernetes-helm jq go-swagger
$ go get -u github.com/golang/protobuf/protoc-gen-go
$ go get -u github.com/go-swagger/go-swagger/cmd/swagger
$ go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway
$ go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger
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
NOTE: The make command can take a while, and we recommend building the specific component you are working on
* `make cli` - Make the argocd CLI tool
* `make server` - Make the API/repo/controller server
* `make codegen` - Builds protobuf and swagger files
* `make argocd-util` - Make the administrator's utility, used for certain tasks such as import/export

## Generating ArgoCD manifests for a specific image repository/tag

During development, the `update-manifests.sh` script, can be used to conveniently regenerate the
ArgoCD installation manifests with a customized image namespace and tag. This enables developers
to easily apply manifests which are using the images that they pushed into their personal container
repository.

```
$ IMAGE_NAMESPACE=jessesuen IMAGE_TAG=latest ./hack/update-manifests.sh
$ kubectl apply -n argocd -f ./manifests/install.yaml
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

## Troubleshooting
* Ensure argocd is installed: ./dist/argocd install
* Ensure you're logged in: ./dist/argocd login --username admin --password <whatever password you set at install> localhost:8080
* Ensure that roles are configured: kubectl create -f install/manifests/02c_argocd-rbac-cm.yaml
* Ensure minikube is running: minikube stop && minikube start
* Ensure Argo CD is aware of minikube: ./dist/argocd cluster add minikube
