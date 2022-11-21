# Installation

You can download the latest Argo CD version from [the latest release page of this repository](https://github.com/argoproj/argo-cd/releases/latest), which will include the `argocd` CLI.

## Linux and WSL

### ArchLinux

```bash
pacman -S argocd
```

### Homebrew

```bash
brew install argocd
```

### Download With Curl

#### Download latest version

```bash
curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64
```

#### Download concrete version

Set `VERSION` replacing `<TAG>` in the command below with the version of Argo CD you would like to download:

```bash
VERSION=<TAG> # Select desired TAG from https://github.com/argoproj/argo-cd/releases
curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/${VERSION}/download/argocd-linux-amd64
```

#### Installation steps

```bash
mkdir -p /usr/local/bin/
sudo install -m 555 argocd-linux-amd64 /usr/local/bin/argocd
rm argocd-linux-amd64
```

Update `$PATH` environment variable if necessary (https://linuxconfig.org/linux-path-environment-variable).

You should now be able to run `argocd` commands.

## Mac

### Homebrew

```bash
brew install argocd
```

### Download With Curl

See [Linux curl instructions](#download-with-curl).

After finishing either of the instructions above, you should now be able to run `argocd` commands.


## Windows

### Download With PowerShell: Invoke-WebRequest

You can view the latest version of Argo CD at the link above or run the following command to grab the version:

```powershell
$version = (Invoke-RestMethod https://api.github.com/repos/argoproj/argo-cd/releases/latest).tag_name
```

Replace `$version` in the command below with the version of Argo CD you would like to download:

```powershell
$url = "https://github.com/argoproj/argo-cd/releases/download/" + $version + "/argocd-windows-amd64.exe"
$output = "argocd.exe"

Invoke-WebRequest -Uri $url -OutFile $output
```
Also please note you will probably need to move the file into your PATH.


After finishing the instructions above, you should now be able to run `argocd` commands.
