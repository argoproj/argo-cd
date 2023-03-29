# Verification of Argo CD Artifacts

## Prerequisites
- cosign `v2.0.0` or higher [installation instructions](https://docs.sigstore.dev/cosign/installation)
- slsa-verifier [installation instructions](https://github.com/slsa-framework/slsa-verifier#installation)

***
## Release Assets
| Asset                   | Description                   |
|-------------------------|-------------------------------|
| argocd-darwin-amd64     | CLI Binary                    |
| argocd-darwin-arm64     | CLI Binary                    |
| argocd-linux_amd64      | CLI Binary                    |
| argocd-linux_arm64      | CLI Binary                    |
| argocd-linux_ppc64le    | CLI Binary                    |
| argocd-linux_s390x      | CLI Binary                    |
| argocd-windows_amd64    | CLI Binary                    |
| argocd-cli.intoto.jsonl | Attestation of CLI binaries   |
| cli_checksums.txt       | Checksums of binaries         |
| sbom.tar.gz             | Sbom                          |
| sbom.tar.gz.pem         | Certificate used to sign sbom |
| sbom.tar.gz.sig         | Signature of sbom                |

***
## Verification of container images

Argo CD container images are signed by [cosign](https://github.com/sigstore/cosign) using identity-based ("keyless") signing and transparency. Executing the following command can be used to verify the signature of a container image:

```bash
cosign verify \
--certificate-identity-regexp https://github.com/argoproj/argo-cd/.github/workflows/image-reuse.yaml@refs/tags/v \
--certificate-oidc-issuer https://token.actions.githubusercontent.com \
quay.io/argoproj/argocd:v2.7.0 | jq
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
## Verification of container image attestations

A [SLSA](https://slsa.dev/) Level 3 provenance is generated using [slsa-github-generator](https://github.com/slsa-framework/slsa-github-generator).

The following command will verify the signature of an attestation and how it was issued. It will contain the payloadType, payload, and signature.
```bash
cosign verify-attestation --type slsaprovenance \
--certificate-identity-regexp https://github.com/slsa-framework/slsa-github-generator/.github/workflows/generator_container_slsa3.yml@refs/tags/v \
--certificate-oidc-issuer https://token.actions.githubusercontent.com \
quay.io/argoproj/argocd:v2.7.0 | jq
```
The payload is a non-falsifiable provenance which is base64 encoded and can be viewed by using the command below:
```bash
cosign verify-attestation --type slsaprovenance \
--certificate-identity-regexp https://github.com/slsa-framework/slsa-github-generator/.github/workflows/generator_container_slsa3.yml@refs/tags/v \
--certificate-oidc-issuer https://token.actions.githubusercontent.com \
quay.io/argoproj/argocd:v2.7.0 | jq -r .payload | base64 -d | jq
```
!!! tip
    `cosign` or `slsa-verifier` can both be used to verify image attestations.
    Check the documentation of each binary for detailed instructions.

***
## Verification of CLI artifacts with attestations

A single attestation (`argocd-cli.intoto.jsonl`) from each release is provided. This can be used with [slsa-verifier](https://github.com/slsa-framework/slsa-verifier#verification-for-github-builders) to verify that a CLI binary was generated using Argo CD workflows on GitHub and ensures it was cryptographically signed.
```bash
slsa-verifier verify-artifact argocd-linux-amd64 --provenance-path argocd-cli.intoto.jsonl  --source-uri github.com/argoproj/argo-cd
```
## Verifying an artifact and output the provenance

```bash
slsa-verifier verify-artifact argocd-linux-amd64 --provenance-path argocd-cli.intoto.jsonl  --source-uri github.com/argoproj/argo-cd --print-provenance | jq
```
## Verification of Sbom

```bash
cosign verify-blob --signature sbom.tar.gz.sig --certificate sbom.tar.gz.pem \
--certificate-identity-regexp ^https://github.com/argoproj/argo-cd/.github/workflows/release.yaml@refs/tags/v \
--certificate-oidc-issuer https://token.actions.githubusercontent.com  \
 ~/Downloads/sbom.tar.gz | jq
```

***
## Verification on Kubernetes

### Policy controllers
!!! note
    We encourage all users to verify signatures and provenances with your admission/policy controller of choice. Doing so will verify that an image was built by us before it's deployed on your Kubernetes cluster.

Cosign signatures and SLSA provenances are compatible with several types of admission controllers. Please see the [cosign documentation](https://docs.sigstore.dev/cosign/overview/#kubernetes-integrations) and [slsa-github-generator](https://github.com/slsa-framework/slsa-github-generator/blob/main/internal/builders/container/README.md#verification) for supported controllers.
