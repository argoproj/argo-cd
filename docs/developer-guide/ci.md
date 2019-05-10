# CI

## Updating The Builder Image

Login to Docker Hub:

```bash
docker login
```

Run:

```bash
make builder-image IMAGE_NAMESPACE=argoproj
```

Choose a version:

```bash
export VERSION=v1.0.0
```

Tag and push:

```bash
docker tag argoproj/argo-cd-ci-builder:latest argoproj/argo-cd-ci-builder:$VERSION
docker push argoproj/argo-cd-ci-builder:$VERSION
```