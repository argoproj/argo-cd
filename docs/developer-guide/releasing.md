# Releasing

Ensure the changelog is up to date. 

Export the branch name, e.g.:

```bash
BRANCH=release-1.0
```

Set the `VERSION` environment variable:

```bash 
# release candidate
VERSION=v1.0.0-rc1
# GA release
VERSION=v1.0.0
```

If not already created, create UI release branch:

```bash
cd argo-cd-ui
git checkout -b $BRANCH
```

Tag UI:

```bash
git tag $VERSION
git push origin $BRANCH --tags
IMAGE_NAMESPACE=argoproj IMAGE_TAG=$VERSION DOCKER_PUSH=true yarn docker
```

If not already created, create release branch:

```bash
cd argo-cd
git checkout -b $BRANCH
git push origin $BRANCH
```

Update `VERSION` and manifests with new version:

```bash
echo ${VERSION:} > VERSION
make manifests IMAGE_TAG=$VERSION
git commit -am "Update manifests to $VERSION"
git push origin $BRANCH
```

Tag, build, and push release to Docker Hub

```bash
git tag $VERSION
make release IMAGE_NAMESPACE=argoproj IMAGE_TAG=$VERSION DOCKER_PUSH=true
git push origin $VERSION
```

Update [Github releases](https://github.com/argoproj/argo-cd/releases) with:

* Getting started (copy from previous release)
* Changelog

## Stable Release

Update Brew formula:

```bash
git clone https://github.com/argoproj/homebrew-tap
cd homebrew-tap
./update.sh ~/go/src/github.com/argoproj/argo-cd/dist/argocd-darwin-amd64
git commit -a -m "Update argocd to $VERSION"
git push
```

Deploy the [site](site.md).

Update `stable` tag:

```
git tag stable --force && git push origin stable --force
```

Create GitHub release from new tag and upload binaries (e.g. dist/argocd-darwin-amd64).
