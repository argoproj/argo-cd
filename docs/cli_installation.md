# Installation

You can download the latest Argo CD version from [the latest release page of this repository](https://github.com/argoproj/argo-cd/releases/latest), which will include the `argocd` CLI.

## Linux

You can view the latest version of Argo CD at the link above or run the following command to grab the version:

```bash
VERSION=$(curl --silent "https://api.github.com/repos/argoproj/argo-cd/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
```

Replace `VERSION` in the command below with the version of Argo CD you would like to download:

```bash
curl -sSL -o /usr/local/bin/argocd https://github.com/argoproj/argo-cd/releases/download/$VERSION/argocd-linux-amd64
```

Make the `argocd` CLI executable:

```bash
chmod +x /usr/local/bin/argocd
```

You should now be able to run `argocd` commands.

## Mac

### Homebrew

```bash
brew install argocd
```

### Download With Curl

You can view the latest version of Argo CD at the link above or run the following command to grab the version:

```bash
VERSION=$(curl --silent "https://api.github.com/repos/argoproj/argo-cd/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
```

Replace `VERSION` in the command below with the version of Argo CD you would like to download:

```bash
curl -sSL -o /usr/local/bin/argocd https://github.com/argoproj/argo-cd/releases/download/$VERSION/argocd-darwin-amd64
```

Make the `argocd` CLI executable:

```bash
chmod +x /usr/local/bin/argocd
```

After finishing either of the instructions above, you should now be able to run `argocd` commands.


## Windows

### Download With Powershell: Invoke-WebRequest

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
