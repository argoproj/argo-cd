# Verification of Argo CD signatures

All Argo CD container images are signed by cosign. Checksums are created for the CLI binaries and then signed to ensure integrity.

## Prerequisites
- Cosign [installation instructions](https://docs.sigstore.dev/cosign/installation)
- Obtain or have a copy of the [public key](https://github.com/argoproj/argo-cd/blob/master/argocd-cosign.pub) ```argocd-cosign.pub```

Once you have installed cosign, you can use [argocd-cosign.pub](https://github.com/argoproj/argo-cd/blob/master/argocd-cosign.pub) to verify the signed assets or container images.
```
-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEesHEB7vX5Y2RxXypjMy1nI1z7iRG
JI9/gt/sYqzpsa65aaNP4npM43DDxoIy/MQBo9s/mxGxmA+8UXeDpVC9vw==
-----END PUBLIC KEY-----
```
## Verification of container images

```bash
cosign verify --key argocd-cosign.pub  quay.io/argoproj/argocd:latest

Verification for quay.io/argoproj/argocd:latest --
The following checks were performed on each of these signatures:
  * The cosign claims were validated
  * The signatures were verified against the specified public key
...
```
## Verification of signed assets

```bash
cosign verify-blob --key cosign.pub --signature $(cat argocd-$VERSION-checksums.sig) argocd-$VERSION-checksums.txt
Verified OK
```
