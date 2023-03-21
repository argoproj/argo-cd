# Running Argo CD locally

## Run Argo CD outside of Kubernetes

During development, it might be viable to run Argo CD outside a Kubernetes cluster. This will greatly speed up development, as you don't have to constantly build, push and install new Argo CD Docker images with your latest changes.

You will still need a working Kubernetes cluster, as described in the [Toolchain Guide](toolchain-guide.md), where Argo CD will store all of its resources and configuration.

If you followed the [Toolchain Guide](toolchain-guide.md) in setting up your toolchain, you can run Argo CD locally with these simple steps:

### Install Argo CD resources to your cluster

First push the installation manifest into argocd namespace:

```shell
kubectl create namespace argocd
kubectl apply -n argocd --force -f manifests/install.yaml
```

### Scale down any Argo CD instance in your cluster

Make sure that Argo CD is not running in your development cluster by scaling down the deployments:

```shell
kubectl -n argocd scale statefulset/argocd-application-controller --replicas 0
kubectl -n argocd scale deployment/argocd-dex-server --replicas 0
kubectl -n argocd scale deployment/argocd-repo-server --replicas 0
kubectl -n argocd scale deployment/argocd-server --replicas 0
kubectl -n argocd scale deployment/argocd-redis --replicas 0
kubectl -n argocd scale deployment/argocd-applicationset-controller --replicas 0
kubectl -n argocd scale deployment/argocd-notifications-controller --replicas 0
```

### Start local services (virtualized toolchain inside Docker)

The started services assume you are running in the namespace where Argo CD is installed. You can set the current context default namespace as follows:

```bash
kubectl config set-context --current --namespace=argocd
```

When you use the virtualized toolchain, starting local services is as simple as running

```bash
make start
```

This will start all Argo CD services and the UI in a Docker container and expose the following ports to your host:

* The Argo CD API server on port 8080
* The Argo CD UI server on port 4000
* The Helm registry server on port 5000

You may get an error listening on port 5000 on macOS:

```text
docker: Error response from daemon: Ports are not available: exposing port TCP 0.0.0.0:5000 -> 0.0.0.0:0: listen tcp 0.0.0.0:5000: bind: address already in use.
```

In that case, you can disable "AirPlay Receiver" in macOS System Preferences.

You can now use either the web UI by pointing your browser to `http://localhost:4000` or use the CLI against the API at `http://localhost:8080`. Be sure to use the `--insecure` and `--plaintext` options to the CLI. Webpack will take a while to bundle resources initially, so the first page load can take several seconds or minutes.

As an alternative to using the above command line parameters each time you call `argocd` CLI, you can set the following environment variables:

```bash
export ARGOCD_SERVER=127.0.0.1:8080
export ARGOCD_OPTS="--plaintext --insecure"
```

### Start local services (running on local machine)

The `make start` command of the virtualized toolchain runs the build and programs inside a Docker container using the test tools image. That makes everything repeatable, but can slow down the development workflow. Particularly on macOS where Docker and the Linux kernel run inside a VM, you may want to try developing fully locally.

Docker should be installed already. Assuming you manage installed software using [Homebrew](https://brew.sh/), you can install other prerequisites like this:

```sh
# goreman is used to start all needed processes to get a working Argo CD development
# environment (defined in `Procfile`)
brew install goreman

# You can use `kind` to run Kubernetes inside Docker. But pointing to any other
# development cluster works fine as well as long as Argo CD can reach it.
brew install kind
```

To set up Kubernetes, you can use kind:

```sh
kind create cluster --kubeconfig ~/.kube/config-kind

# The started services assume you are running in the namespace where Argo CD is
# installed. Set the current context default namespace.
export KUBECONFIG=~/.kube/config-kind
kubectl config set-context --current --namespace=argocd
```

Follow the above sections "Install Argo CD resources to your cluster" and "Scale down any Argo CD instance in your cluster" to deploy all needed manifests such as config maps.

Start local services:

```sh
# Ensure you point to the correct Kubernetes cluster as shown above. For example:
export KUBECONFIG=~/.kube/config-kind

make start-local
```

This will start all Argo CD services and the UI in a Docker container and expose the following ports to your host:

* The Argo CD API server on port 8080
* The Argo CD UI server on port 4000
* The Helm registry server on port 5000

If you get firewall dialogs, for example on macOS, you can click "Deny", since no access from outside your computer is typically desired.

Check that all programs have started:

```text
$ goreman run status
*controller
*api-server
[...]
```

If not all critical processes run (marked with `*`), check logs to see why they terminated.

In case of an error like `gpg: key generation failed: Unknown elliptic curve` (a [gnupg bug](https://dev.gnupg.org/T5444)), disable GPG verification before running `make start-local`:

```sh
export ARGOCD_GPG_ENABLED=false
```

You may get an error listening on port 5000 on macOS:

```text
docker: Error response from daemon: Ports are not available: exposing port TCP 0.0.0.0:5000 -> 0.0.0.0:0: listen tcp 0.0.0.0:5000: bind: address already in use.
```

In that case, you can disable "AirPlay Receiver" in macOS System Preferences.

You can now use either the web UI by pointing your browser to `http://localhost:4000` or use the CLI against the API at `http://localhost:8080`. Be sure to use the `--insecure` and `--plaintext` options to the CLI. Webpack will take a while to bundle resources initially, so the first page load can take several seconds or minutes.

As an alternative to using the above command line parameters each time you call `argocd` CLI, you can set the following environment variables:

```bash
export ARGOCD_SERVER=127.0.0.1:8080
export ARGOCD_OPTS="--plaintext --insecure"
```

After making a code change, ensure to rebuild and restart the respective service:

```sh
# Example for working on the repo server Go code, see other service names in `Procfile`
goreman run restart repo-server
```

Clean up when you're done:

```sh
kind delete cluster; rm -f ~/.kube/config-kind
```

### Scale up Argo CD in your cluster

Once you have finished testing your changes locally and want to bring back Argo CD in your development cluster, simply scale the deployments up again:

```bash
kubectl -n argocd scale statefulset/argocd-application-controller --replicas 1
kubectl -n argocd scale deployment/argocd-dex-server --replicas 1
kubectl -n argocd scale deployment/argocd-repo-server --replicas 1
kubectl -n argocd scale deployment/argocd-server --replicas 1
kubectl -n argocd scale deployment/argocd-redis --replicas 1
```

## Run your own Argo CD images on your cluster

For your final tests, it might be necessary to build your own images and run them in your development cluster.

### Create Docker account and login

You might need to create an account on [Docker Hub](https://hub.docker.com) if you don't have one already. Once you created your account, login from your development environment:

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

With `IMAGE_NAMESPACE` and `IMAGE_TAG` still set, run:

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
