# Helm chart provenance verification

Argo CD can verify [Helm chart provenance](https://helm.sh/docs/topics/provenance/) for charts from HTTP/HTTPS Helm repositories, Helm OCI repositories (`enableOCI: true`), and OCI repositories (`type: oci`, `oci://` URLs). Before generating manifests, the repo-server:

1. Pulls the chart with `helm pull --prov`, which also downloads its `.prov` file. In an HTTP/HTTPS repository the `.prov` is published next to the chart archive (`<chart-url>.prov`). In an OCI registry it is the provenance layer `helm push` uploads when a `.prov` file sits next to the pushed archive. For `oci://` sources, the provenance layer is read from the pulled artifact instead.
2. Verifies the `.prov` PGP signature against the Argo CD GnuPG keyring, and checks that the signer is listed in the policy's `keys`.
3. Checks that the SHA-256 digest of the chart archive matches the digest in the signed `.prov` body, and that the chart name and version signed in it are the ones requested. For `oci://` sources the digest is that of the chart layer in the image manifest, the name is the last segment of `repoURL`, and the version is checked only when `targetRevision` is an exact version.

Together, these steps are equivalent to:

```bash
helm pull <repo>/mychart --version 1.0.0 --prov
helm verify mychart-1.0.0.tgz
```

If any step fails, the application gets a `ComparisonError` condition and cannot sync. The check is reported as `HELM/PROVENANCE`.

For GnuPG verification of Git commits, see [Git GnuPG verification](./source-integrity-git-gpg.md).

> [!WARNING]
> Verification requires GnuPG. When the repo-server runs with `ARGOCD_GPG_ENABLED=false`, Helm policies are skipped and only a warning is logged.

## Keyring

Helm provenance uses the same repo-server GnuPG keyring as [Git GnuPG verification](./source-integrity-git-gpg.md). Import the public key that signed the chart (the key used by `helm package --sign`):

```bash
argocd gpg add --from <path-to-key>
argocd gpg list
```

For RBAC, Web UI import, and declarative setup through `argocd-gpg-keys-cm`, see [Keyring management](./source-integrity-git-gpg.md#keyring-management).

## Policies

Policies are declared on the `AppProject` under `spec.sourceIntegrity.helm`:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: myproject
spec:
  sourceIntegrity:
    helm:
      policies:
        - repos:
            - url: "https://charts.example.com"
            - url: "https://charts.example.com/*"
            - url: "!https://charts.example.com/internal/*"
          provenance:
            keys:
              - "4AEE18F83AFDEB23"
```

`repos` are globs matched against the application's Helm `repoURL`, with the same rules as Git policies: a leading `!` excludes. A glob ending in `/*` does not match the bare URL, so list `https://charts.example.com` as well if applications use it as `repoURL`. For Helm OCI repositories, match the host-style `repoURL` (for example `registry.example.com/charts*`), without an `oci://` prefix. For `oci://` sources, match the full URL, including `oci://`. At most one Helm policy may match a repository; if several match, verification fails. Sources no policy matches are not verified.

> [!NOTE]
> Helm policies are currently the only source integrity check for `oci://` sources. A Helm policy that matches an `oci://` source whose artifact is not a Helm chart fails verification, so scope `repos` to your chart repositories.

`provenance.keys` lists the allowed signer key IDs. A signature made by a subkey is accepted when its primary key is listed. An empty list rejects every chart.

Git and Helm policies can be combined, which is useful for multi-source applications:

```yaml
spec:
  sourceIntegrity:
    git:
      policies:
        - repos:
            - url: "https://github.com/my-org/*"
          gpg:
            mode: head
            keys:
              - "D56C4FCA57A46444"
    helm:
      policies:
        - repos:
            - url: "https://charts.example.com"
            - url: "https://charts.example.com/*"
          provenance:
            keys:
              - "4AEE18F83AFDEB23"
```

As with Git source integrity, `argocd app sync --local` is rejected for applications with a matching policy.

## Troubleshooting

| Error | Cause and fix |
|-------|---------------|
| `could not access chart for provenance verification: chart has no provenance file (.prov)` | The chart was published without a `.prov`, or downloading it failed. Publish a signed chart, or narrow the policy's `repos`. The chart is pulled again on the next refresh until its `.prov` is downloaded. |
| `could not access chart for provenance verification: provenance unavailable: ... is not a Helm chart` | A Helm policy matches an `oci://` source whose artifact is not a Helm chart. Narrow the policy's `repos`. |
| `could not access chart for provenance verification: provenance unavailable: ... has no provenance layer` | The `oci://` chart was pushed without a `.prov`. Push it again with `helm push` from a directory that also holds the `.prov`. |
| `provenance signature verification failed: signed with key not in keyring` | Import the signer's public key with `argocd gpg add`. |
| `provenance signature verification failed: bad signature` | The `.prov` file is corrupt or was modified after signing. |
| `signed with unallowed key (key_id=...)` | The signature is valid, but neither the signing key nor its primary key is in `provenance.keys`. |
| `chart digest mismatch` | The chart archive differs from the one that was signed. Run `argocd app get <app> --hard-refresh`; if it persists, contact the chart maintainer. |
| `provenance signs chart "..."` or `provenance signs version "..." of the chart` | The signed `.prov` belongs to a different chart or version than the one requested, for example an older release served in its place. Contact the chart maintainer. |
| `multiple (N) Helm source integrity policies found` | Narrow the `repos` globs so one policy matches. |

To check a chart outside Argo CD, run `helm verify --keyring <public-keyring> mychart-1.0.0.tgz`, which checks both the signature and the archive digest. `gpg --verify mychart-1.0.0.tgz.prov` only shows the signing key ID to compare with `provenance.keys`; it does not check the archive.
