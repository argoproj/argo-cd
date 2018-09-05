# ArgoCD Release Instructions

1. Tag and build argo-cd-ui
```bash
cd argo-cd-ui
git tag vX.Y.Z
git push upstream vX.Y.Z
IMAGE_NAMESPACE=argoproj IMAGE_TAG=vX.Y.Z DOCKER_PUSH=true yarn docker
```

2. Edit CHANGELOG.md, getting_started.md, install manifests with new version
```bash
make install-manifest IMAGE_NAMESPACE=argoproj IMAGE_TAG=vX.Y.Z
git commit -a -m "Update manifests to vX.Y.Z"
git push upstream master
```

3. Tag, build the release, and push to docker hub
```bash
git tag vX.Y.Z
make release IMAGE_NAMESPACE=argoproj IMAGE_TAG=vX.Y.Z DOCKER_PUSH=true
git push upstream vX.Y.Z
```

4. Create release-X.Y branch (if necessary)
```bash
git checkout -b release-X.Y
git push upstream release-X.Y
```

5. Update argocd brew formula
```bash
git clone https://github.com/argoproj/homebrew-tap
cd homebrew-tap
shasum -a 256 ~/go/src/github.com/argoproj/argo-cd/dist/argocd-darwin-amd64
# edit argocd.rb with version and checksum
git commit -a -m "Update argocd to vX.Y.Z"
git push
```
