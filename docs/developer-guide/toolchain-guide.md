# Development toolchain

## Before you start


!!! note
    **Attention minikube users**: By default, minikube will create Kubernetes client configuration that uses authentication data from files. This is incompatible with the virtualized toolchain. So if you intend to use the virtualized toolchain, you have to embed this authentication data into the client configuration. To do so, start minikube using `minikube start --embed-certs`. Please also note that minikube using the Docker driver is currently not supported with the virtualized toolchain, because the Docker driver exposes the API server on 127.0.0.1 hard-coded. If in doubt, run `make verify-kube-connect` to find out.


## Local vs Virtualized toolchain

Argo CD provides a fully virtualized development and testing toolchain using Docker images. Those images provide the same runtime environment as the final product, and it is much easier to keep up-to-date with changes to the toolchain and dependencies. They also require configuring the default K8s API URL that comes with installing a local K8s cluster.

The local toolchain results in a faster development and testing cycle. It also requires installing additional tools on your machine. 

Most relevant targets for the build & test cycles in the `Makefile` provide two variants, one of them suffixed with `-local`. For example, `make test` will run unit tests in the Docker container (virtualized toolchain), `make test-local` (local toolchain) will run it natively on your local system.

### Using Virtualized toolchain

If you are going to use the virtualized toolchain, please bear in mind the following things:

* Your Kubernetes API server must listen on the interface of your local machine or VM, and not on `127.0.0.1` or  `localhost` only.
* Your Kubernetes client configuration (`~/.kube/config`) must not use an API URL that points to `localhost`, `127.0.0.1` or `0.0.0.0`

The Docker container for the virtualized toolchain will use the following local mounts from your workstation, and possibly modify its contents:

* `~/go/src` - Your Go workspace's source directory (modifications expected)
* `~/.cache/go-build` - Your Go build cache (modifications expected)
* `~/.kube` - Your Kubernetes client configuration (no modifications)
* `/tmp` - Your system's temp directory (modifications expected)

#### Docker privileges

If you opt in to use the virtualized toolchain, you will need to have the appropriate privileges to interact with the Docker daemon. It is not recommended to work as the root user, and if your user does not have the permissions to talk to the Docker user, but you have `sudo` setup on your system, you can set the environment variable `SUDO` to `sudo` in order to have the build scripts make any calls to the `docker` CLI using sudo, without affecting the other parts of the build scripts (which should be executed with your normal user privileges).

You can either set this before calling `make`, like so for example:

```
SUDO=sudo make sometarget
```

Or you can opt to export this permanently to your environment, for example
```
export SUDO=sudo
```

If you have podman installed, you can also leverage its rootless mode. In order to use podman for running and testing Argo CD locally, set the `DOCKER` environment variable to `podman` before you run `make`, e.g:

```
DOCKER=podman make start
```

#### Build the required Docker image

Build the required Docker image by running `make test-tools-image`. This image offers the environment of the virtualized toolchain.

The `Dockerfile` used to build these images can be found at `test/container/Dockerfile`.

#### Test connection from build container to your K8s cluster

You can test whether the virtualized toolchain has access to your Kubernetes cluster by running `make verify-kube-connect` which will run `kubectl version` inside the Docker container used for running all tests.


If you receive an error similar to the following:

```
The connection to the server 127.0.0.1:6443 was refused - did you specify the right host or port?
make: *** [Makefile:386: verify-kube-connect] Error 1
```

you should edit your `~/.kube/config` and modify the `server` option to point to your correct K8s API (as described above).

### Using k3d

[k3d](https://github.com/rancher/k3d) is a lightweight wrapper to run [k3s](https://github.com/rancher/k3s), a minimal Kubernetes distribution, in docker. Because it's running in a docker container, you're dealing with docker's internal networking rules when using k3d. A typical Kubernetes cluster running on your local machine is part of the same network that you're on, so you can access it using **kubectl**. However, a Kubernetes cluster running within a docker container (in this case, the one launched by make) cannot access 0.0.0.0 from inside the container itself, when 0.0.0.0 is a network resource outside the container itself (and/or the container's network). This is the cost of a fully self-contained, disposable Kubernetes cluster. The following steps should help with a successful `make verify-kube-connect` execution.

1. Find your host IP by executing `ifconfig` on Mac/Linux and `ipconfig` on Windows. For most users, the following command works to find the IP address.

    * For Mac:

    ```
    IP=`ifconfig en0 | grep inet | grep -v inet6 | awk '{print $2}'`
    echo $IP
    ```

    * For Linux:

    ```
    IP=`ifconfig eth0 | grep inet | grep -v inet6 | awk '{print $2}'`
    echo $IP
    ```

    Keep in mind that this IP is dynamically assigned by the router so if your router restarts for any reason, your IP might change.

2. Edit your ~/.kube/config and replace 0.0.0.0 with the above IP address.

3. Execute a `kubectl version` to make sure you can still connect to the Kubernetes API server via this new IP. Run `make verify-kube-connect` and check if it works.

4. Finally, so that you don't have to keep updating your kube-config whenever you spin up a new k3d cluster, add `--api-port $IP:6550` to your **k3d cluster create** command, where $IP is the value from step 1. An example command is provided here:

```
k3d cluster create my-cluster --wait --k3s-arg '--disable=traefik@server:*' --api-port $IP:6550 -p 443:443@loadbalancer
```

!!!note
    For k3d versions less than v5.0.0, the example command flags `--k3s-arg` and `'--disable=traefik@server:*'` should change to `--k3s-server-arg` and `'--disable=traefik'`, respectively.


## Setting up a local toolchain

For development, you can either use the fully virtualized toolchain provided as Docker images, or you can set up the toolchain on your local development machine. Due to the dynamic nature of requirements, you might want to stay with the virtualized environment.

### Install required dependencies and build-tools

!!!note
    The installations instructions are valid for Linux hosts only. Mac instructions will follow shortly.

For installing the tools required to build and test Argo CD on your local system, we provide convenient installer scripts. By default, they will install binaries to `/usr/local/bin` on your system, which might require `root` privileges.

You can change the target location by setting the `BIN` environment before running the installer scripts. For example, you can install the binaries into `~/go/bin` (which should then be the first component in your `PATH` environment, i.e. `export PATH=~/go/bin:$PATH`):

```shell
BIN=~/go/bin make install-tools-local
```

Additionally, you have to install at least the following tools via your OS's package manager (this list might not be always up-to-date):

* Git LFS plugin
* GnuPG version 2
