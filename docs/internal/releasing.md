# ArgoCD Release Instructions

1. Tag, build, and push argo-cd-ui
```bash
cd argo-cd-ui
git tag vX.Y.Z
git push upstream vX.Y.Z
IMAGE_NAMESPACE=argoproj IMAGE_TAG=vX.Y.Z DOCKER_PUSH=true yarn docker
```

2. Create release-X.Y branch (if creating initial X.Y release)
```bash
git checkout -b release-X.Y
git push upstream release-X.Y
```

3. Update manifests with new version
```bash
vi manifests/base/kustomization.yaml # update with new image tags
make manifests
git commit -a -m "Update manifests to vX.Y.Z"
git push upstream master
```

4. Tag, build, and push release to docker hub
```bash
git tag vX.Y.Z
make release IMAGE_NAMESPACE=argoproj IMAGE_TAG=vX.Y.Z DOCKER_PUSH=true
git push upstream vX.Y.Z
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

6. Update documentation:
* Edit CHANGELOG.md with release notes
* Update getting_started.md with new version
* Create GitHub release from new tag and upload binaries (e.g. dist/argocd-darwin-amd64)
