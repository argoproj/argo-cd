# Installing Argo CD CLI

Argo CD provides a CLI (command line interface) tool for interaction through its
API. The CLI is currently available for the following platforms:

* Linux on amd64 architecture,
* Mac (darwin) on amd64 architecture,
* Windows on amd64 architecture

Ports for other architectures, such as arm32 and arm64, are not yet officially
available but are planned.

Installing and/or using the CLI is completely optional, but recommended. The
CLI provides a convinient way to interact with Argo CD through its API.

## Install on Linux

We are not aware of official Argo CD CLI packages for Linux distributions, so
the easiest way to retrieve and install the CLI on your Linux machine is to
download the appropriate binary from GitHub using the shell and `curl`:

### Manual download and install Linux CLI

First, retrieve the version of the current release (or set the `ARGOCD_VERSION`
environment variable manually):

```bash
ARGOCD_VERSION=$(curl --silent "https://api.github.com/repos/argoproj/argo-cd/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
```

Then, retrieve the binary from GitHub to a temporary location:

```bash
curl -sSL -o /tmp/argocd-${ARGOCD_VERSION} https://github.com/argoproj/argo-cd/releases/download/${ARGOCD_VERSION}/argocd-linux-amd64
```

Finally, make the binary executable and move it to a location within your
`$PATH`, in this example `/usr/local/bin`:

```bash
chmod +x /tmp/argocd-${VERSION}
sudo mv /tmp/argocd-${VERSION} /usr/local/bin/argocd 
```

Verify that your CLI is working properly:

```bash
argocd version --client
```

This should give an output similar to the following (details may differ across
versions and platform):

```bash
argocd: v1.8.1+c2547dc
  BuildDate: 2020-12-10T02:57:57Z
  GitCommit: c2547dca95437fdbb4d1e984b0592e6b9110d37f
  GitTreeState: clean
  GoVersion: go1.14.12
  Compiler: gc
  Platform: linux/amd64
```

## Install on MacOS (Darwin)

You can install the MacOS CLI either using Homebrew, or manually by downloading
the CLI from GitHub.

### Installing using Homebrew

This is as simple as running

```bash
brew install argocd
```

### Manual download and install MacOS CLI

First, retrieve the version of the current release (or set the `ARGOCD_VERSION`
environment variable manually):

```bash
ARGOCD_VERSION=$(curl --silent "https://api.github.com/repos/argoproj/argo-cd/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
```

Then, retrieve the binary from GitHub to a temporary location:

```bash
curl -sSL -o /tmp/argocd-${ARGOCD_VERSION} https://github.com/argoproj/argo-cd/releases/download/${ARGOCD_VERSION}/argocd-darwin-amd64
```

Finally, make the binary executable and move it to a location within your
`$PATH`, in this example `/usr/local/bin`:

```bash
chmod +x /tmp/argocd-${VERSION}
sudo mv /tmp/argocd-${VERSION} /usr/local/bin/argocd 
```

Verify that your CLI is working properly:

```bash
argocd version --client
```

This should give an output similar to the following (details may differ across
versions and platform):

```bash
argocd: v1.8.1+c2547dc
  BuildDate: 2020-12-10T02:57:57Z
  GitCommit: c2547dca95437fdbb4d1e984b0592e6b9110d37f
  GitTreeState: clean
  GoVersion: go1.14.12
  Compiler: gc
  Platform: darwin/amd64
```

## Install on Windows

You can also use the Argo CD CLI from a Windows machine.

### Installation via WSL

The easiest way to use the CLI from Windows is via the [Windows Subsystem for Linux](https://docs.microsoft.com/en-us/windows/wsl/about).

First you need [to install WSL 1 or 2](https://docs.microsoft.com/en-us/windows/wsl/install) on your Windows machine. Then you can install the Argo CD CLI by following the instructions at the [linux section above](#install-on-linux).

### Manual installation

You can install the native Windows CLI to any folder of your choosing. Open a terminal by typing `cmd` in your start menu and enter the following:

```shell
cd c:\
md myapps
cd myapps
curl -sSL -o argocd.exe https://github.com/argoproj/argo-cd/releases/latest/download/argocd-windows-amd64.exe
```

Then type `argocd version --client` to verify your CLI. Finally add the `myapps` folder to your Windows PATH variable.

