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
brew tap argoproj/tap
brew install argoproj/tap/argocd
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
