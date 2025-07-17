# Workflows

| Workflow           | Description                                                    |
|--------------------|----------------------------------------------------------------|
| ci-build.yaml      | Build, lint, test, codegen, build-ui, analyze, e2e-test        |
| codeql.yaml        | CodeQL analysis                                                |
| image-reuse.yaml   | Build, push, and Sign container images                         |
| image.yaml         | Build container image for PR's & publish for push events       |
| init-release.yaml  | Build manifests and version then create a PR for release branch|
| pr-title-check.yaml| Lint PR for semantic information                               |
| release.yaml       | Build images, cli-binaries, provenances, and post actions      |
| scorecard.yaml     | Generate scorecard for supply-chain security                   |
| update-snyk.yaml   | Scheduled snyk reports                                         |

# Reusable workflows

## image-reuse.yaml

- The resuable workflow can be used to publish or build images with multiple container registries(Quay,GHCR, dockerhub), and then sign them with cosign when an image is published.
- A GO version `must` be specified e.g. 1.21
- The image name for each registry *must* contain the tag. Note: multiple tags are allowed for each registry using a CSV type.
- Multiple platforms can be specified e.g. linux/amd64,linux/arm64
- Images are not published by default. A boolean value must be set to `true` to push images.
- An optional target can be specified.

| Inputs            | Description                         | Type        | Required | Defaults        |
|-------------------|-------------------------------------|-------------|----------|-----------------|
| go-version        | Version of Go to be used            | string      | true     | none            |
| quay_image_name   | Full image name and tag             | CSV, string | false    | none            |
| ghcr_image_name   | Full image name and tag             | CSV, string | false    | none            |
| docker_image_name | Full image name and tag             | CSV, string | false    | none            |
| platforms         | Platforms to build (linux/amd64)    | CSV, string | false    | linux/amd64     |
| push              | Whether to push image/s to registry | boolean     | false    | false           |
| target            | Target build stage                  | string      | false    | none            |

| Outputs     | Description                              | Type  |
|-------------|------------------------------------------|-------|
|image-digest | Image digest of image container created  | string|

