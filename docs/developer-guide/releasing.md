# Releasing

Make sure you are logged into Docker Hub:

```bash
docker login
```

Export the upstream repository and branch name, e.g.:

```bash
REPO=upstream ;# or origin 
BRANCH=release-1.3
```

Set the `VERSION` environment variable:

```bash 
# release candidate
VERSION=v1.3.0-rc1
# GA release
VERSION=v1.3.1
```

Update `VERSION` and manifests with new version:

```bash
git checkout $BRANCH
echo ${VERSION:1} > VERSION
make dev-tools-image
make manifests IMAGE_TAG=$VERSION
git commit -am "Update manifests to $VERSION"
git tag $VERSION
```

Build, and push release to Docker Hub

```bash
git clean -fd
make release IMAGE_NAMESPACE=argoproj IMAGE_TAG=$VERSION DOCKER_PUSH=true
git push $REPO $BRANCH
git push $REPO $VERSION
```

If GA, update `stable` tag:

```bash
git tag stable --force && git push $REPO stable --force
```

Update [Github releases](https://github.com/argoproj/argo-cd/releases) with:

* Getting started (copy from previous release)
* Changelog
* Binaries (e.g. `dist/argocd-darwin-amd64`).

If GA, update Brew formula:

```bash
git clone git@github.com:argoproj/homebrew-tap.git
cd homebrew-tap
./update.sh argocd $VERSION
git commit -am "Update argocd to $VERSION"
git push
```

### Verify

Locally:

```bash
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/$VERSION/manifests/install.yaml
```

Follow the [Getting Started Guide](../getting_started/).

If GA:

```bash
brew upgrade argocd
/usr/local/bin/argocd version
```

Sync Argo CD in [https://cd.apps.argoproj.io/applications/argo-cd](https://cd.apps.argoproj.io/applications/argo-cd).

Deploy the [site](site.md).
