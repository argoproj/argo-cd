# Verification of Argo CD signatures

All Argo CD container images are signed by cosign. Checksums are created for the CLI binaries and then signed to ensure integrity.

## Prerequisites
- Cosign [installation instructions](https://docs.sigstore.dev/cosign/installation)
- Obtain or have a copy of ```argocd-cosign.pub```, which can be located in the assets section of the [release page](https://github.com/argoproj/argo-cd/releases)

Once you have installed cosign, you can use ```argocd-cosign.pub``` to verify the signed assets or container images.


## Verification of container images

```bash
cosign verify --key argocd-cosign.pub  quay.io/argoproj/argocd:<VERSION>

Verification for quay.io/argoproj/argocd:<VERSION> --
The following checks were performed on each of these signatures:
  * The cosign claims were validated
  * The signatures were verified against the specified public key
...
```
## Verification of signed assets

```bash
cosign verify-blob --key cosign.pub --signature $(cat argocd-<VERSION>-checksums.sig) argocd-$VERSION-checksums.txt
Verified OK
```
## Admission controllers

Cosign is compatible with several types of admission controllers.  Please see the [Cosign documentation](https://docs.sigstore.dev/cosign/overview/#kubernetes-integrations) for supported controllers
