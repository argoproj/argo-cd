# Maintaining Internal Argo CD Forks

Most Argo CD contributors don't need this section to contribute to Argo CD. In most cases, the [Regular Developer Guide](index.md) is sufficient.

This section will help companies that need to publish custom Argo CD images or publish custom Argo CD releases from their forks.
Such companies need the below documentation in addition to the [Regular Developer Guide](index.md).
It will also help Argo CD maintainers to test the release process in a test environment.

## Understanding where and which upstream images are published

Official upstream release images are published to `quay.io/argoproj/argocd`.   
Upstream images, built from upstream master branch with the latest tag, are also published to `quay.io/argoproj/argocd`.
Official upstream attestations and upstream images, built from upstream master branch tagged with commit sha, are published to `ghcr.io/argoproj/argo-cd/argocd`.

## Publishing custom images upon master builds on forks

In order to publish custom images on Argo CD forks, the following needs to be performed:

### Configuring custom image repository GitHub Actions variables on forks
All or some of the above variables may need to be configured, dependending on you desired setup.   

- `IMAGE_REGISTRY` - `quay.io` by default   
- `IMAGE_NAMESPACE` - `argoproj` by default (overriding this one is mandatory to enable fork image builds)   
- `IMAGE_REPOSITORY` - `argocd` by default    
- `GHCR_REGISTRY` - `ghcr.io` by default (usually does not need to be changed)   
- `GHCR_NAMESPACE` - `argoproj/argo-cd` by default (has to be changed, usually to YOUR_GITHUB_USERNAME/YOUR_FORK_REPO_NAME)   
- `GHCR_REPOSITORY` - `argocd` by default

The custom images names will then be constructed as following:
$IMAGE_REGISTRY/$IMAGE_NAMESPACE/$IMAGE_REPOSITORY
$GHCR_REGISTRY/$GHCR_NAMESPACE/$GHCR_REPOSITORY

For example:   

Let's assume your GitHub username is `my-user`, and you have forked argo-cd repo to a repo named `my-argo-cd-fork` under this user. Let's also assume that you want to publish the images to `quay.io/my-quay-user/argocd`, and that you will be using the ghcr packages repo under your GitHub username.

For the above, you would be setting the following on your fork's GitHub Actions vars:    
- `IMAGE_NAMESPACE`: `my-quay-user`   
- `GHCR_NAMESPACE`: `my-user/my-argo-cd-fork`   

### Configuring custom image repository GitHub Actions secrets on forks
You will also need to configure the following secrets on your fork's GitHub Actions with the credentials for your image repository:   
- `RELEASE_QUAY_TOKEN`   
- `RELEASE_QUAY_USERNAME`

## Enabling fork releases
It is possible to create custom Argo CD releases on forks. For this, the GitHub Actions variable `ENABLE_FORK_RELEASES` has to be set to `true` on the fork.   
In order to create a custom release, the upstream repo must be forked with all the tags, as the existing tags are needed to create the release.   
The regular [Release Process](releasing.md) can then be followed, with one important change:

> [!WARNING]

> Upon tagging the release branch, the `hack/trigger-release.sh` script must be run on `origin` and 
> NOT on ~~upstream~~, in order for the release tag to be pushed to the fork and not attempted to be pushed to upstream. 
> For example: 

> `./hack/trigger-release.sh v2.7.2 origin`
