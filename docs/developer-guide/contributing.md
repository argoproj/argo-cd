# Contribution guide

## Preface

We want to make contributing to ArgoCD as simple and smooth as possible.

This guide shall help you in setting up your build & test environment, so that you can start developing and testing bug fixes and feature enhancements without having to make too much effort in setting up a local toolchain.

If you want to submit a PR, please read this document carefully, as it contains important information guiding you through our PR quality gates.

As is the case with the development process, this document is under constant change. If you notice any error, or if you think this document is out-of-date, or if you think it is missing something: Feel free to submit a PR or submit a bug to our GitHub issue tracker.

If you need guidance with submitting a PR, or have any other questions regarding development of ArgoCD, do not hesitate to [join our Slack](https://argoproj.github.io/community/join-slack) and get in touch with us in the `#argo-dev` channel!

## Before you start

You will need at least the following things in your toolchain in order to develop and test ArgoCD locally:

* A Kubernetes cluster. You won't need a fully blown multi-master, multi-node cluster, but you will need something like K3S, Minikube or microk8s. You will also need a working Kubernetes client (`kubectl`) configuration in your development environment. The configuration must reside in `~/.kube/config` and the API server URL must point to the IP address of your local machine (or VM), and **not** to `localhost` or `127.0.0.1` if you are using the virtualized development toolchain (see below)

* You will also need a working Docker runtime environment, to be able to build and run images.
The Docker version must be fairly recent, and support multi-stage builds. You should not work as root. Make your local user a member of the `docker` group to be able to control the Docker service on your machine.

* Obviously, you will need a `git` client for pulling source code and pushing back your changes.

* Last but not least, you will need a Go SDK and related tools (such as GNU `make`) installed and working on your development environment. The minimum required Go version for building ArgoCD is **v1.14.0**.

* We will assume that your Go workspace is at `~/go`

!!! note
    **Attention minikube users**: By default, minikube will create Kubernetes client configuration that uses authentication data from files. This is incompatible with the virtualized toolchain. So if you intend to use the virtualized toolchain, you have to embed this authentication data into the client configuration. To do so, issue `minikube config set embed-certs true` and restart your minikube. Please also note that minikube using the Docker driver is currently not supported with the virtualized toolchain, because the Docker driver exposes the API server on 127.0.0.1 hard-coded. If in doubt, run `make verify-kube-connect` to find out.

## Submitting PRs

When you submit a PR against ArgoCD's GitHub repository, a couple of CI checks will be run automatically to ensure your changes will build fine and meet certain quality standards. Your contribution needs to pass those checks in order to be merged into the repository.

In general, it might be beneficial to only submit a PR for an existing issue. Especially for larger changes, an Enhancement Proposal should exist before.

!!!note

    Please make sure that you always create PRs from a branch that is up-to-date with the latest changes from ArgoCD's master branch. Depending on how long it takes for the maintainers to review and merge your PR, it might be necessary to pull in latest changes into your branch again.

Please understand that we, as an Open Source project, have limited capacities for reviewing and merging PRs to ArgoCD. We will do our best to review your PR and give you feedback as soon as possible, but please bear with us if it takes a little longer as expected.

The following read will help you to submit a PR that meets the standards of our CI tests:

### Title of the PR

Please use a meaningful and concise title for your PR. This will help us to pick PRs for review quickly, and the PR title will also end up in the Changelog.

We use the [Semantic PR title checker](https://github.com/zeke/semantic-pull-requests) to categorize your PR into one of the following categories:

* `fix` - Your PR contains one or more code bug fixes
* `feat` - Your PR contains a new feature
* `docs` - Your PR improves the documentation
* `chore` - Your PR improves any internals of ArgoCD, such as the build process, unit tests, etc

Please prefix the title of your PR with one of the valid categories. For example, if you chose the title your PR `Add documentation for GitHub SSO integration`, please use `docs: Add documentation for GitHub SSO integration` instead.

### Contributor License Agreement

Every contributor to ArgoCD must have signed the current Contributor License Agreement (CLA). You only have to sign the CLA when you are a first time contributor, or when the agreement has changed since your last time signing it. The main purpose of the CLA is to ensure that you hold the required rights for your contribution. The CLA signing is an automated process.

You can read the current version of the CLA [here](https://cla-assistant.io/argoproj/argo-cd).

### PR template checklist

Upon opening a PR, the details will contain a checklist from a template. Please read the checklist, and tick those marks that apply to you.

### Automated builds & tests

After you have submitted your PR, and whenever you push new commits to that branch, GitHub will run a number of Continuous Integration checks against your code. It will execute the following actions, and each of them has to pass:

* Build the Go code (`make build`)
* Generate API glue code and manifests (`make codegen`)
* Run a Go linter on the code (`make lint`)
* Run the unit tests (`make test`)
* Run the End-to-End tests (`make test-e2e`)
* Build and lint the UI code (`make lint-ui`)
* Build the `argocd` CLI (`make cli`)

If any of these tests in the CI pipeline fail, it means that some of your contribution is considered faulty (or a test might be flaky, see below).

### Code test coverage

We use [CodeCov](https://codecov.io) in our CI pipeline to check for test coverage, and once you submit your PR, it will run and report on the coverage difference as a comment within your PR. If the difference is too high in the negative, i.e. your submission introduced a significant drop in code coverage, the CI check will fail.

Whenever you develop a new feature or submit a bug fix, please also write appropriate unit tests for it. If you write a completely new module, please aim for at least 80% of coverage.
If you want to see how much coverage just a specific module (i.e. your new one) has, you can set the `TEST_MODULE` to the (fully qualified) name of that module with `make test`, i.e.

```bash
 make test TEST_MODULE=github.com/argoproj/argo-cd/server/cache
...
ok      github.com/argoproj/argo-cd/server/cache        0.029s  coverage: 89.3% of statements
```

## Local vs Virtualized toolchain

ArgoCD provides a fully virtualized development and testing toolchain using Docker images. It is recommended to use those images, as they provide the same runtime environment as the final product and it is much easier to keep up-to-date with changes to the toolchain and dependencies. But as using Docker comes with a slight performance penalty, you might want to setup a local toolchain.

Most relevant targets for the build & test cycles in the `Makefile` provide two variants, one of them suffixed with `-local`. For example, `make test` will run unit tests in the Docker container, `make test-local` will run it natively on your local system.

If you are going to use the virtualized toolchain, please bear in mind the following things:

* Your Kubernetes API server must listen on the interface of your local machine or VM, and not on `127.0.0.1` only.
* Your Kubernetes client configuration (`~/.kube/config`) must not use an API URL that points to `localhost` or `127.0.0.1`.

You can test whether the virtualized toolchain has access to your Kubernetes cluster by running `make verify-kube-connect` (*after* you have setup your development environment, as described below), which will run `kubectl version` inside the Docker container used for running all tests.

The Docker container for the virtualized toolchain will use the following local mounts from your workstation, and possibly modify its contents:

* `~/go/src` - Your Go workspace's source directory (modifications expected)
* `~/.cache/go-build` - Your Go build cache (modifications expected)
* `~/.kube` - Your Kubernetes client configuration (no modifications)
* `/tmp` - Your system's temp directory (modifications expected)

## Setting up your development environment

The following steps are required no matter whether you chose to use a virtualized or a local toolchain.

### Clone the ArgoCD repository from your personal fork on GitHub

* `mkdir -p ~/go/src/github.com/argoproj`
* `cd ~/go/src/github.com/argoproj`
* `git clone https://github.com/yourghuser/argo-cd`
* `cd argo-cd`

### Optional: Setup an additional Git remote

While everyone has their own Git workflow, the author of this document recommends to create a remote called `upstream` in your local copy pointing to the original ArgoCD repository. This way, you can easily keep your local branches up-to-date by merging in latest changes from the ArgoCD repository, i.e. by doing a `git pull upstream master` in your locally checked out branch. To create the remote, run `git remote add upstream https://github.com/argoproj/argo-cd`

### Install the must-have requirements

Make sure you fulfill the pre-requisites above and run some preliminary tests. Neither of them should report an error.

* Run `kubectl version`
* Run `docker version`
* Run `go version`

### Build (or pull) the required Docker image

Build the required Docker image by running `make test-tools-image` or pull the latest version by issuing `docker pull argoproj/argocd-test-tools`.

The `Dockerfile` used to build these images can be found at `test/container/Dockerfile`.

### Test connection from build container to your K8s cluster

Run `make verify-kube-connect`, it should execute without error.

If you receive an error similar to the following:

```
The connection to the server 127.0.0.1:6443 was refused - did you specify the right host or port?
make: *** [Makefile:386: verify-kube-connect] Error 1
```

you should edit your `~/.kube/config` and modify the `server` option to point to your correct K8s API (as described above).

### Using k3d

k3d is a lightweight wrapper to run [k3s](https://github.com/rancher/k3s), a minimal Kubernetes distribution, in docker. Because it's running in a docker container, you're dealing with docker's internal networking rules when using k3d. A typical Kubernetes cluster running on your local machine is part of the same network that you're on so you can access it using **kubectl**. However, a Kubernetes cluster running within a docker container (in this case, the one launched by make) cannot access 0.0.0.0 from inside the container itself, when 0.0.0.0 is a network resource outside the container itself (and/or the container's network). This is the cost of a fully self-contained, disposable Kubernetes cluster. The following steps should help with a successful `make verify-kube-connect` execution.

1. Find your host IP by executing `ifconfig` on Mac/Linux and `ipconfig` on Windows. For most users, the following command works to find the IP address.

For Mac:
```
IP=`ifconfig en0 | grep inet | grep -v inet6 | awk '{print $2}'`
echo $IP
```

For Linux:
```
IP=`ifconfig eth0 | grep inet | grep -v inet6 | awk '{print $2}'`
echo $IP
```

Keep in mind that this IP is dynamically assigned by the router so if your router restarts for any reason, your IP might change.

2. Edit your ~/.kube/config and replace 0.0.0.0 with the above IP address.

3. Execute a `kubectl version` to make sure you can still connect to the Kubernetes API server via this new IP. Run `make verify-kube-connect` and check if it works.

4. Finally, so that you don't have to keep updating your kube-config whenever you spin up a new k3d cluster, add `--api-port $IP:6550` to your **k3d cluster create** command, where $IP is the value from step 1. An example command is provided here.

```
k3d cluster create my-cluster --wait --k3s-server-arg '--disable=traefik' --api-port $IP:6550 -p 443:443@loadbalancer
```

## The development cycle

When you have developed and possibly manually tested the code you want to contribute, you should ensure that everything will build correctly. Commit your changes to the local copy of your Git branch and perform the following steps:

### Pull in all build dependencies

As build dependencies change over time, you have to synchronize your development environment with the current specification. In order to pull in all required dependencies, issue:

* `make dep-ui`

ArgoCD recently migrated to Go modules. Usually, dependencies will be downloaded on build time, but the Makefile provides two targets to download and vendor all dependencies:

* `make mod-download` will download all required Go modules and
* `make mod-vendor` will vendor those dependencies into the ArgoCD source tree

### Generate API glue code and other assets

ArgoCD relies on Google's [Protocol Buffers](https://developers.google.com/protocol-buffers) for its API, and this makes heavy use of auto-generated glue code and stubs. Whenever you touched parts of the API code, you must re-generate the auto generated code.

* Run `make codegen`, this might take a while
* Check if something has changed by running `git status` or `git diff`
* Commit any possible changes to your local Git branch, an appropriate commit message would be `Changes from codegen`, for example.

!!!note
    There are a few non-obvious assets that are auto-generated. You should not change the autogenerated assets, as they will be overwritten by a subsequent run of `make codegen`. Instead, change their source files. Prominent examples of non-obvious auto-generated code are `swagger.json` or the installation manifest YAMLs.

### Build your code and run unit tests

After the code glue has been generated, your code should build and the unit tests should run without any errors. Execute the following statements:

* `make build`
* `make test`

These steps are non-modifying, so there's no need to check for changes afterwards.

### Lint your code base

In order to keep a consistent code style in our source tree, your code must be well-formed in accordance to some widely accepted rules, which are applied by a Linter.

The Linter might make some automatic changes to your code, such as indentation fixes. Some other errors reported by the Linter have to be fixed manually.

* Run `make lint` and observe any errors reported by the Linter
* Fix any of the errors reported and commit to your local branch
* Finally, after the Linter reports no errors anymore, run `git status` or `git diff` to check for any changes made automatically by Lint
* If there were automatic changes, commit them to your local branch

If you touched UI code, you should also run the Yarn linter on it:

* Run `make lint-ui`
* Fix any of the errors reported by it

## Setting up a local toolchain

For development, you can either use the fully virtualized toolchain provided as Docker images, or you can set up the toolchain on your local development machine. Due to the dynamic nature of requirements, you might want to stay with the virtualized environment.

### Install required dependencies and build-tools

!!!note
    The installations instructions are valid for Linux hosts only. Mac instructions will follow shortly.

For installing the tools required to build and test ArgoCD on your local system, we provide convenient installer scripts. By default, they will install binaries to `/usr/local/bin` on your system, which might require `root` privileges.

You can change the target location by setting the `BIN` environment before running the installer scripts. For example, you can install the binaries into `~/go/bin` (which should then be the first component in your `PATH` environment, i.e. `export PATH=~/go/bin:$PATH`):

```shell
make BIN=~/go/bin install-tools-local
```

Additionally, you have to install at least the following tools via your OS's package manager (this list might not be always up-to-date):

* Git LFS plugin
* GnuPG version 2

### Install Go dependencies

You need to pull in all required Go dependencies. To do so, run

* `make mod-download-local`
* `make mod-vendor-local`

### Test your build toolchain

The first thing you can do whether your build toolchain is setup correctly is by generating the glue code for the API and after that, run a normal build:

* `make codegen-local`
* `make build-local`

This should return without any error.

### Run unit-tests

The next thing is to make sure that unit tests are running correctly on your system. These will require that all dependencies, such as Helm, Kustomize, Git, GnuPG, etc are correctly installed and fully functioning:

* `make test-local`

### Run end-to-end tests

The final step is running the End-to-End testsuite, which makes sure that your Kubernetes dependencies are working properly. This will involve starting all of the ArgoCD components locally on your computer. The end-to-end tests consists of two parts: a server component, and a client component.

* First, start the End-to-End server: `make start-e2e-local`. This will spawn a number of processes and services on your system.
* When all components have started, run `make test-e2e-local` to run the end-to-end tests against your local services.

For more information about End-to-End tests, refer to the [End-to-End test documentation](test-e2e.md).
