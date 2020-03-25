# CI

!!!warning
    This documentation is out-of-date. Please bear with us while we work to
    update the documentation to reflect reality!

## Troubleshooting Builds

### "Check nothing has changed" step fails
 
If your PR fails the `codegen` CI step, you can either:

(1) Simple - download the `codgen.patch` file from CircleCI and apply it:

![download codegen patch file](../assets/download-codegen-patch-file.png)

```bash
git apply codegen.patch 
git commit -am "Applies codegen patch"
```

(2) Advanced - if you have the tools installed (see the contributing guide), run the following:

```bash
make pre-commit
git commit -am 'Ran pre-commit checks'
```

## Updating The Builder Image

Login to Docker Hub:

```bash
docker login
```

Build image:

```bash
make builder-image IMAGE_NAMESPACE=argoproj IMAGE_TAG=v1.0.0
```

## Public CD

Every commit to master is built and published to `docker.pkg.github.com/argoproj/argo-cd/argocd:<version>-<short-sha>`. The list of images is available at
https://github.com/argoproj/argo-cd/packages.

!!! note
    Github docker registry [requires](https://github.community/t5/GitHub-Actions/docker-pull-from-public-GitHub-Package-Registry-fail-with-quot/m-p/32888#M1294) authentication to read
    even publicly available packages. Follow the steps from Kubernetes [documentation](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry)
    to configure image pull secret if you want to use `docker.pkg.github.com/argoproj/argo-cd/argocd` image.

The image is automatically deployed to the dev Argo CD instance: [https://cd.apps.argoproj.io/](https://cd.apps.argoproj.io/)
