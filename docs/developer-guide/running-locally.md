# Running ArgoCD locally

## Run ArgoCD outside of Kubernetes

During development, it might be viable to run ArgoCD outside of a Kubernetes cluster. This will greatly speed up development, as you don't have to constantly build, push and install new ArgoCD Docker images with your latest changes.

You will still need a working Kubernetes cluster, as described in the [Contribution Guide](contributing.md), where ArgoCD will store all of its resources.

If you followed the [Contribution Guide](contributing.md) in setting up your toolchain, you can run ArgoCD locally with these simple steps:

### Install ArgoCD resources to your cluster

First push the installation manifest into argocd namespace:

```shell
kubectl create namespace argocd
kubectl apply -n argocd --force -f manifests/install.yaml
```

### Scale down any ArgoCD instance in your cluster

Make sure that ArgoCD is not running in your development cluster by scaling down the deployments:

```shell
kubectl -n argocd scale statefulset/argocd-application-controller --replicas 0
kubectl -n argocd scale deployment/argocd-dex-server --replicas 0
kubectl -n argocd scale deployment/argocd-repo-server --replicas 0
kubectl -n argocd scale deployment/argocd-server --replicas 0
kubectl -n argocd scale deployment/argocd-redis --replicas 0
```

### Start local services

Before starting local services, make sure you are present in `argocd` namespace. When you use the virtualized toolchain, starting local services is as simple as running

```bash
make start
```

This will start all ArgoCD services and the UI in a Docker container and expose the following ports to your host:

* The ArgoCD API server on port 8080
* The ArgoCD UI server on port 4000

You can now use either the web UI by pointing your browser to `http://localhost:4000` or use the CLI against the API at `http://localhost:8080`. Be sure to use the `--insecure` and `--plaintext` options to the CLI.

As an alternative to using the above command line parameters each time you call `argocd` CLI, you can set the following environment variables:

```bash
export ARGOCD_SERVER=127.0.0.1:8080
export ARGOCD_OPTS="--plaintext --insecure"
```

### Scale up ArgoCD in your cluster

Once you have finished testing your changes locally and want to bring back ArgoCD in your development cluster, simply scale the deployments up again:

```bash
kubectl -n argocd scale statefulset/argocd-application-controller --replicas 1
kubectl -n argocd scale deployment/argocd-dex-server --replicas 1
kubectl -n argocd scale deployment/argocd-repo-server --replicas 1
kubectl -n argocd scale deployment/argocd-server --replicas 1
kubectl -n argocd scale deployment/argocd-redis --replicas 1
```

## Run your own ArgoCD images on your cluster

For your final tests, it might be necessary to build your own images and run them in your development cluster.

### Create Docker account and login

You might need to create a account on [Docker Hub](https://hub.docker.com) if you don't have one already. Once you created your account, login from your development environment:

```bash
docker login
```

### Create and push Docker images

You will need to push the built images to your own Docker namespace:

```bash
export IMAGE_NAMESPACE=youraccount
```

If you don't set `IMAGE_TAG` in your environment, the default of `:latest` will be used. To change the tag, export the variable in the environment:

```bash
export IMAGE_TAG=1.5.0-myrc
```

Then you can build & push the image in one step:

```bash
DOCKER_PUSH=true make image
```

### Configure manifests for your image

With `IMAGE_NAMESPACE` and `IMAGE_TAG` still set, run

```bash
make manifests
```

to build a new set of installation manifests which include your specific image reference.

!!!note
    Do not commit these manifests to your repository. If you want to revert the changes, the easiest way is to unset `IMAGE_NAMESPACE` and `IMAGE_TAG` from your environment and run `make manifests` again. This will re-create the default manifests.

### Configure your cluster with custom manifests

The final step is to push the manifests to your cluster, so it will pull and run your image:

```bash
kubectl apply -n argocd --force -f manifests/install.yaml
```
