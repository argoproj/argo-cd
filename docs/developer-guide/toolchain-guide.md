# Development Toolchain

## Prerequisites
[Development Environment](development-environment.md)

## Local vs Virtualized toolchain

Argo CD provides a fully virtualized development and testing toolchain using Docker images. Those images provide the same runtime environment as the final product, and it is much easier to keep up-to-date with changes to the toolchain and dependencies. The virtualized toolchain runs the build and programs inside a Docker container using the test tools image. That makes everything repeatable. The dynamic nature of requirements is another reason to choose this toolchain. This setup may also require configuring the default K8s API URL that comes with installing a local K8s cluster.

The local toolchain results in a faster development and testing cycle. Particularly on macOS where Docker and the Linux kernel run inside a VM, you may want to try developing fully locally. Local toolchain also requires installing additional tools on your machine. This toolchain is a good choice for working with an IDE debugger. 

Most relevant targets for the build & test cycles in the `Makefile` provide two variants, one of them suffixed with `-local`. For example, `make test` will run unit tests in the Docker container (virtualized toolchain), `make test-local` (local toolchain) will run it natively on your local system.

### Setting up a virtualized toolchain

If you are going to use the virtualized toolchain, please bear in mind the following things:

* Your Kubernetes API server must listen on the interface of your local machine or VM, and not on `127.0.0.1` or  `localhost` only.
* Your Kubernetes client configuration (`~/.kube/config`) must not use an API URL that points to `localhost`, `127.0.0.1` or `0.0.0.0`

The Docker container for the virtualized toolchain will use the following local mounts from your workstation, and possibly modify its contents:

* `~/go/src` - Your Go workspace's source directory (modifications expected)
* `~/.cache/go-build` - Your Go build cache (modifications expected)
* `~/.kube` - Your Kubernetes client configuration (no modifications)

#### Known issues on macOS
[Known issues](mac-users.md)

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

#### Using Podman
In order to use podman for running and testing Argo CD locally, set the `DOCKER` environment variable to `podman` before you run `make`, e.g:

```
DOCKER=podman make start
```
If you have podman installed, you can leverage its rootless mode.

#### Build the required Docker image

Build the required Docker image by running `make test-tools-image`. This image offers the environment of the virtualized toolchain.

The `Dockerfile` used to build these images can be found at `test/container/Dockerfile`.

#### Configure your K8s cluster for connection from the build container
##### K3d
K3d is a minimal Kubernetes distribution, in docker. Because it's running in a docker container, you're dealing with docker's internal networking rules when using k3d. A typical Kubernetes cluster running on your local machine is part of the same network that you're on, so you can access it using **kubectl**. However, a Kubernetes cluster running within a docker container (in this case, the one launched by make) cannot access 0.0.0.0 from inside the container itself, when 0.0.0.0 is a network resource outside the container itself (and/or the container's network). This is the cost of a fully self-contained, disposable Kubernetes cluster.

The configuration you will need for Argo CD virtualized toolchain:

1. For most users, the following command works to find the host IP address.

    * If you have perl

       ```pl
       perl -e '
       use strict;
       use Socket;

       my $target = sockaddr_in(53, inet_aton("8.8.8.8"));
       socket(my $s, AF_INET, SOCK_DGRAM, getprotobyname("udp")) or die $!;
       connect($s, $target) or die $!;
       my $local_addr = getsockname($s) or die $!;
       my (undef, $ip) = sockaddr_in($local_addr);
       print "IP: ", inet_ntoa($ip), "\n";
       '
       ```

    * If you don't

      * Try `ip route get 8.8.8.8` on Linux
      * Try `ifconfig`/`ipconfig` (and pick the ip address that feels right -- look for `192.168.x.x` or `10.x.x.x` addresses)

    Note that `8.8.8.8` is Google's Public DNS server, in most places it's likely to be accessible and thus is a good proxy for "which outbound address would my computer use", but you can replace it with a different IP address if necessary.

    Keep in mind that this IP is dynamically assigned by the router so if your router restarts for any reason, your IP might change.

2. Edit your ~/.kube/config and replace 0.0.0.0 with the above IP address, delete the cluster cert and add `insecure-skip-tls-verify: true`

3. Execute a `kubectl version` to make sure you can still connect to the Kubernetes API server via this new IP. 

##### Minikube

By default, minikube will create Kubernetes client configuration that uses authentication data from files. This is incompatible with the virtualized toolchain. So if you intend to use the virtualized toolchain, you have to embed this authentication data into the client configuration. To do so, start minikube using `minikube start --embed-certs`. Please also note that minikube using the Docker driver is currently not supported with the virtualized toolchain, because the Docker driver exposes the API server on 127.0.0.1 hard-coded.

#### Test connection from the build container to your K8s cluster

You can test whether the virtualized toolchain has access to your Kubernetes cluster by running `make verify-kube-connect` which will run `kubectl version` inside the Docker container used for running all tests.


If you receive an error similar to the following:

```
The connection to the server 127.0.0.1:6443 was refused - did you specify the right host or port?
make: *** [Makefile:386: verify-kube-connect] Error 1
```

you should edit your `~/.kube/config` and modify the `server` option to point to your correct K8s API (as described above).

### Setting up a local toolchain

#### Install `node`

<https://nodejs.org/en/download>

#### Install `yarn`

<https://classic.yarnpkg.com/lang/en/docs/install/>

#### Install `goreman`

<https://github.com/mattn/goreman#getting-started>

Goreman is used to start all needed processes to get a working Argo CD development environment (defined in `Procfile`)

#### Install required dependencies and build-tools

> [!NOTE]
> The installations instructions are valid for Linux hosts only. Mac instructions will follow shortly.

For installing the tools required to build and test Argo CD on your local system, we provide convenient installer scripts. By default, they will install binaries to `/usr/local/bin` on your system, which might require `root` privileges.

You can change the target location by setting the `BIN` environment before running the installer scripts. For example, you can install the binaries into `~/go/bin` (which should then be the first component in your `PATH` environment, i.e. `export PATH=~/go/bin:$PATH`):

```shell
BIN=~/go/bin make install-tools-local
```

Additionally, you have to install at least the following tools via your OS's package manager (this list might not be always up-to-date):

* Git LFS plugin
* GnuPG version 2
