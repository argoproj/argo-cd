## Mac

### Homebrew

Both Intel and Apple Silicon Macs can use Homebrew:

```bash
brew install argocd
```

### Download With Curl

#### Mac Intel (x86_64)

You can view the latest version of Argo CD at the link above or run the following command to grab the version:

```bash
VERSION=$(curl --silent "https://api.github.com/repos/argoproj/argo-cd/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
```

Replace `VERSION` in the command below with the version of Argo CD you would like to download:

```bash
curl -sSL -o argocd https://github.com/argoproj/argo-cd/releases/download/$VERSION/argocd-darwin-amd64
```

Install the Argo CD CLI binary:

```bash
sudo install -m 555 argocd /usr/local/bin/argocd
rm argocd
```

#### Mac Apple Silicon (M1/M2/M3)

You can view the latest version of Argo CD at the link above or run the following command to grab the version:

```bash
VERSION=$(curl --silent "https://api.github.com/repos/argoproj/argo-cd/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
```

Replace `VERSION` in the command below with the version of Argo CD you would like to download:

```bash
curl -sSL -o argocd https://github.com/argoproj/argo-cd/releases/download/$VERSION/argocd-darwin-arm64
```

Install the Argo CD CLI binary:

```bash
sudo install -m 555 argocd /usr/local/bin/argocd
rm argocd
```

After finishing either of the instructions above, you should now be able to run `argocd` commands.
