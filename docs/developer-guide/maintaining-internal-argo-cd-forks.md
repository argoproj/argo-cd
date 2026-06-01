# Maintaining Internal Argo CD Forks

Most Argo CD contributors don't need this section to contribute to Argo CD. In most cases, the [Regular Developer Guide](index.md) is sufficient.

This section will help companies that need to publish custom Argo CD images or publish custom Argo CD releases from their forks.
Such companies need the below documentation in addition to the [Regular Developer Guide](index.md).
This section will also help Argo CD maintainers to test the release process in a test environment.

## Understanding where and which upstream images are published

Official upstream release tags (`vX.Y.Z*`) publish their multi-platform images and the corresponding provenance attestations—to `quay.io/argoproj/argocd` (or whatever registry a fork configures via `IMAGE_*` variables).  
Upstream master builds continue to refresh the `latest` tag in the same primary registry, while also pushing commit-tagged images (and their provenance) to `ghcr.io/argoproj/argo-cd/argocd` so `cd.apps.argoproj.io` can pin exact SHAs.  
Forks inherit the same behavior but target their customized registries/namespaces and do not deploy to `cd.apps.argoproj.io`.

## Publishing custom images from forked master branches

Fork builds can publish their own containers once workflow variables point at your registry/namespace instead of `argoproj`.

### Configuring GitHub Actions variables
Adjust the variables below to match your setup (overriding `IMAGE_NAMESPACE` is required, because it flips the workflows out of “upstream” mode):

- `IMAGE_NAMESPACE` – defaults to `argoproj` (overriding required)
- `IMAGE_REPOSITORY` – defaults to `argocd` (may need overriding)
- `GHCR_NAMESPACE` – defaults to `${{ github.repository }}`, which translates to `<YOUR_GITHUB_USERNAME>/<YOUR_FORK_REPO>`, rarely needs overriding)
- `GHCR_REPOSITORY` – defaults to `argocd` (may need overriding)

These values produce the final image names:

- `quay.io/$IMAGE_NAMESPACE/$IMAGE_REPOSITORY`
- `ghcr.io/$GHCR_NAMESPACE/$GHCR_REPOSITORY`

Example: if your GitHub account is `my-user`, your fork is `my-argo-cd-fork`, and you want to push release images to `quay.io/my-quay-user/argocd`, configure:

- `IMAGE_NAMESPACE = my-quay-user`
Your master build images will then be published to `quay.io/my-quay-user/argocd:latest`, and the commit tagged images along with the attestations will be published under the Packages (GHCR) of your GitHub fork repo. 

### Configuring GitHub Actions secrets
Supply credentials for your primary registry so the workflow can push:

- `RELEASE_QUAY_USERNAME`
- `RELEASE_QUAY_TOKEN`

## Enabling fork releases

Forks can run the full release workflow by setting `ENABLE_FORK_RELEASES: true`, ensuring all upstream tags are fetched (the release tooling needs previous tags for changelog diffs), and reusing the same image variables/secrets listed above so release images push to your custom registry. After that, follow the standard [Release Process](releasing.md) with one critical adjustment:

> [!WARNING]
> When invoking `hack/trigger-release.sh`, point it at your fork remote (usually `origin`) rather than ~~upstream~~, otherwise the script may try to push official tags.  
> Example: `./hack/trigger-release.sh v2.7.2 origin`
