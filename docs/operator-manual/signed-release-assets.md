# Verification of Argo CD Artifacts

## Prerequisites
- cosign `v2.0.0` or higher [installation instructions](https://docs.sigstore.dev/cosign/installation)
- slsa-verifier [installation instructions](https://github.com/slsa-framework/slsa-verifier#installation)
- crane [installation instructions](https://github.com/google/go-containerregistry/blob/main/cmd/crane/README.md) (for container verification only)

***
## Release Assets
| Asset                    | Description                   |
|--------------------------|-------------------------------|
| argocd-darwin-amd64      | CLI Binary                    |
| argocd-darwin-arm64      | CLI Binary                    |
| argocd-linux_amd64       | CLI Binary                    |
| argocd-linux_arm64       | CLI Binary                    |
| argocd-linux_ppc64le     | CLI Binary                    |
| argocd-linux_s390x       | CLI Binary                    |
| argocd-windows_amd64     | CLI Binary                    |
| argocd-cli.intoto.jsonl  | Attestation of CLI binaries   |
| argocd-sbom.intoto.jsonl | Attestation of SBOM           |
| cli_checksums.txt        | Checksums of binaries         |
| sbom.tar.gz              | Sbom                          |
| sbom.tar.gz.pem          | Certificate used to sign sbom |
| sbom.tar.gz.sig          | Signature of sbom             |

***
## Verification of container images

Argo CD container images are signed by [cosign](https://github.com/sigstore/cosign) using identity-based ("keyless") signing and transparency. Executing the following command can be used to verify the signature of a container image:

```bash
cosign verify \
--certificate-identity-regexp https://github.com/argoproj/argo-cd/.github/workflows/image-reuse.yaml@refs/tags/v \
--certificate-oidc-issuer https://token.actions.githubusercontent.com \
--certificate-github-workflow-repository "argoproj/argo-cd" \
quay.io/argoproj/argocd:v2.11.3 | jq
```
The command should output the following if the container image was correctly verified:
```bash
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - Existence of the claims in the transparency log was verified offline
  - Any certificates were verified against the Fulcio roots.
[
  {
    "critical": {
      "identity": {
        "docker-reference": "quay.io/argoproj/argo-cd"
      },
      "image": {
        "docker-manifest-digest": "sha256:63dc60481b1b2abf271e1f2b866be8a92962b0e53aaa728902caa8ac8d235277"
      },
      "type": "cosign container image signature"
    },
    "optional": {
      "1.3.6.1.4.1.57264.1.1": "https://token.actions.githubusercontent.com",
      "1.3.6.1.4.1.57264.1.2": "push",
      "1.3.6.1.4.1.57264.1.3": "a6ec84da0eaa519cbd91a8f016cf4050c03323b2",
      "1.3.6.1.4.1.57264.1.4": "Publish ArgoCD Release",
      "1.3.6.1.4.1.57264.1.5": "argoproj/argo-cd",
      "1.3.6.1.4.1.57264.1.6": "refs/tags/<version>",
      ...
```

***
## Verification of container image with SLSA attestations

A [SLSA](https://slsa.dev/) Level 3 provenance is generated using [slsa-github-generator](https://github.com/slsa-framework/slsa-github-generator).

The following command will verify the signature of an attestation and how it was issued. It will contain the payloadType, payload, and signature.

Run the following command as per the [slsa-verifier documentation](https://github.com/slsa-framework/slsa-verifier/tree/main#containers):

```bash
# Get the immutable container image to prevent TOCTOU attacks https://github.com/slsa-framework/slsa-verifier#toctou-attacks
IMAGE=quay.io/argoproj/argocd:v2.7.0
IMAGE="${IMAGE}@"$(crane digest "${IMAGE}")
# Verify provenance, including the tag to prevent rollback attacks.
slsa-verifier verify-image "$IMAGE" \
    --source-uri github.com/argoproj/argo-cd \
    --source-tag v2.7.0
```

If you only want to verify up to the major or minor verion of the source repository tag (instead of the full tag), use the `--source-versioned-tag` which performs semantic versioning verification:

```shell
slsa-verifier verify-image "$IMAGE" \
    --source-uri github.com/argoproj/argo-cd \
    --source-versioned-tag v2 # Note: May use v2.7 for minor version verification.
```

The attestation payload contains a non-forgeable provenance which is base64 encoded and can be viewed by passing the `--print-provenance` option to the commands above:

```bash
slsa-verifier verify-image "$IMAGE" \
    --source-uri github.com/argoproj/argo-cd \
    --source-tag v2.7.0 \
    --print-provenance | jq
```

If you prefer using cosign, follow these [instructions](https://github.com/slsa-framework/slsa-github-generator/blob/main/internal/builders/container/README.md#cosign).

!!! tip
    `cosign` or `slsa-verifier` can both be used to verify image attestations.
    Check the documentation of each binary for detailed instructions.

***

## Verification of CLI artifacts with SLSA attestations

A single attestation (`argocd-cli.intoto.jsonl`) from each release is provided. This can be used with [slsa-verifier](https://github.com/slsa-framework/slsa-verifier#verification-for-github-builders) to verify that a CLI binary was generated using Argo CD workflows on GitHub and ensures it was cryptographically signed.

```bash
slsa-verifier verify-artifact argocd-linux-amd64 \
  --provenance-path argocd-cli.intoto.jsonl \
  --source-uri github.com/argoproj/argo-cd \
  --source-tag v2.7.0
```

If you only want to verify up to the major or minor verion of the source repository tag (instead of the full tag), use the `--source-versioned-tag` which performs semantic versioning verification:

```shell
slsa-verifier verify-artifact argocd-linux-amd64 \
  --provenance-path argocd-cli.intoto.jsonl \
  --source-uri github.com/argoproj/argo-cd \
  --source-versioned-tag v2 # Note: May use v2.7 for minor version verification.
```

The payload is a non-forgeable provenance which is base64 encoded and can be viewed by passing the `--print-provenance` option to the commands above:

```bash
slsa-verifier verify-artifact argocd-linux-amd64 \
  --provenance-path argocd-cli.intoto.jsonl \
  --source-uri github.com/argoproj/argo-cd \
  --source-tag v2.7.0 \
  --print-provenance | jq
```

## Verification of Sbom

A single attestation (`argocd-sbom.intoto.jsonl`) from each release is provided along with the sbom (`sbom.tar.gz`). This can be used with [slsa-verifier](https://github.com/slsa-framework/slsa-verifier#verification-for-github-builders) to verify that the SBOM was generated using Argo CD workflows on GitHub and ensures it was cryptographically signed.

```bash
slsa-verifier verify-artifact sbom.tar.gz \
  --provenance-path argocd-sbom.intoto.jsonl \
  --source-uri github.com/argoproj/argo-cd \
  --source-tag v2.7.0
```

***
## Verification on Kubernetes

### Policy controllers
!!! note
    We encourage all users to verify signatures and provenances with your admission/policy controller of choice. Doing so will verify that an image was built by us before it's deployed on your Kubernetes cluster.

Cosign signatures and SLSA provenances are compatible with several types of admission controllers. Please see the [cosign documentation](https://docs.sigstore.dev/cosign/overview/#kubernetes-integrations) and [slsa-github-generator](https://github.com/slsa-framework/slsa-github-generator/blob/main/internal/builders/container/README.md#verification) for supported controllers.
