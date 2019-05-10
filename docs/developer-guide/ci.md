# CI

## Updating The Builder Image

Login to Docker Hub:

```bash
docker login
```

Build image:

```bash
make builder-image IMAGE_NAMESPACE=argoproj IMAGE_TAG=v1.0.0
```
