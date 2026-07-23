# Helm chart provenance verification

## Overview

Source integrity Helm policies verify Helm chart signatures for **traditional Helm repositories** — HTTP/HTTPS chart repos (`repoURL` + `chart`). This is [Helm chart provenance](https://helm.sh/docs/topics/provenance/) verification.

This verification is equivalent to running:

```bash
helm pull <repo>/mychart --version 1.0.0 --prov
gpg --verify mychart-1.0.0.tgz.prov
```

Argo CD automatically performs these steps during sync:

1. Fetches the chart and its `.prov` signature file from the Helm repository
2. Verifies the PGP signature using keys configured in `provenance.keys`
3. Ensures the chart contents match the cryptographically signed digest

For implementation details (mirror fallback, digest checks, sync errors), see [What Argo CD verifies](#what-argo-cd-verifies) below.

> [!NOTE]
> **OCI Helm charts**
>
> OCI Helm registries (`enableOCI: true` or host-style `repoURL` without `https://`) are **not** covered by `sourceIntegrity.helm` in this release. Chart integrity for OCI Helm is planned as part of broader OCI support (for example Sigstore/cosign), not Helm `.prov` layers on OCI artifacts.

For GnuPG verification of Git commit signatures, see [Git GnuPG verification](./source-integrity-git-gpg.md).

> [!WARNING]
> **Policies silently bypass if GnuPG is disabled**
>
> Provenance verification requires `ARGOCD_GPG_ENABLED=true` Environment Variable.
>
> **Critical:** If GnuPG is disabled, configured policies will **pass without verification** —
> there is no error or warning. Unsigned charts or charts with invalid signatures will
> deploy successfully.
>
> Verify GnuPG is enabled before relying on provenance policies:
>
> ```bash
> kubectl exec -it deploy/argocd-repo-server -- printenv | grep ARGOCD_GPG_ENABLED
> # Expected: ARGOCD_GPG_ENABLED=true
> ```

## Managing Argo CD GnuPG keyring

Helm provenance uses the same repo-server GnuPG keyring as [Git GnuPG verification](./source-integrity-git-gpg.md). The public key that signed the chart's `.prov` file must be imported into Argo CD and listed in `sourceIntegrity.helm.policies[].provenance.keys`.

### Keyring RBAC rules

RBAC for managing keys (`gpgkeys` resource) is identical to Git signature verification. See [Keyring RBAC rules](./source-integrity-git-gpg.md#keyring-rbac-rules) in the Git GnuPG guide.

### Keyring management

Import the chart maintainer's public key (the key that signs `.prov` files when you run `helm package --sign`):

```bash
argocd gpg add --from <path-to-key>
argocd gpg list
```

Then reference that key ID in your Helm provenance policy (`provenance.keys`).

> [!NOTE]
> After you import a key, it may take a short time to propagate to all `argocd-repo-server` pods.
> If sync fails right after import, wait briefly and retry, or restart the repo-server pods.

For Web UI import, declarative setup via `argocd-gpg-keys-cm`, and full keyring workflows, see [Keyring management](./source-integrity-git-gpg.md#keyring-management) in the Git GnuPG guide.

### Inspecting the keyring

Helm provenance uses the same keyring as Git. For background and sync behavior, see [Inspecting GnuPG key ring](./source-integrity-git-gpg.md#inspecting-gnupg-key-ring) in the Git GnuPG guide.

Quick check that a key ID from `provenance.keys` is on the repo-server:

```bash
kubectl exec -it <argocd-repo-server-pod> -- \
  sh -c 'GNUPGHOME=/app/config/gpg/keys gpg --list-keys'
```

## What Argo CD verifies

Once `.prov` and chart archive are loaded, Argo CD runs these steps (check name `HELM/PROVENANCE`):

| Step | What is checked |
|------|-----------------|
| 1. Provenance present | `.prov` content is non-empty |
| 2. PGP signature | `gpg --verify` on the cleartext message; signer key ID must be in `provenance.keys` and in the repo-server keyring |
| 3. Signed body parse | Extract the signed YAML plaintext from the PGP cleartext envelope |
| 4. Files digest | Find `files.<chart-filename>: sha256:...` in the signed body and compare to SHA256 of the chart archive |

If any step fails, sync is blocked with a `ResourceComparison` error.

## Policies for Helm provenance verification

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
            - url: "https://charts.example.com/*"
            - url: "!https://charts.example.com/internal/*"
          provenance:
            keys:
              - "4AEE18F83AFDEB23"
```

The `repos` field lists glob patterns matched against the application's Helm `repoURL` (same rules as Git policies: positive globs apply, negative globs starting with `!` exclude).

Only one Helm policy applies per source repository. Sources not matched by any policy are not verified for provenance.

Multi-source applications can combine Git and traditional Helm policies in the same `sourceIntegrity` block:

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
            - url: "https://charts.example.com/*"
          provenance:
            keys:
              - "4AEE18F83AFDEB23"
```

### The `provenance` policy

| Field | Meaning |
|-------|---------|
| `keys` | Allowed signer key IDs (short or long form). Keys must be present in the repo-server GnuPG keyring. |

When a matching policy includes `provenance`, Argo CD requires verification to succeed before sync: `.prov` must be present, the signature must verify, the signer must be listed in `keys`, and the chart digest must match.

When `keys` is empty, verification still runs but no signer is trusted. Sync fails if `.prov` is missing, or with `signed with unallowed key` if the chart is signed. Configure at least one trusted key ID to allow sync.

To skip Helm provenance checks for a project, do not configure `sourceIntegrity.helm` (or use a project without `sourceIntegrity`). Each Helm policy requires a `provenance` block in the CRD.

## Sync failures and check results

Failed verification surfaces as `ApplicationConditionComparisonError` with details under `sourceIntegrityResult` (check name `HELM/PROVENANCE`).

## Testing your setup

### Verify a chart locally

Before adding a project policy, confirm the maintainer key and `.prov` file are valid:

```bash
helm pull <repo>/mychart --version 1.0.0 --prov
gpg --verify mychart-1.0.0.tgz.prov
```

Import the signer key into Argo CD and confirm it is listed:

```bash
argocd gpg add --from <path-to-key>
argocd gpg list
```

### Test with a project policy

Create a test `AppProject` and application that point at a signed chart whose key is in `provenance.keys`, then sync and confirm `Application.status` has no comparison errors and `sourceIntegrityResult` shows a passing `HELM/PROVENANCE` check.

## Troubleshooting

### Key not trusted

Ensure the signer's public key is imported into the Argo CD keyring and listed in `provenance.keys`. Use `argocd gpg list` and [Inspecting GnuPG key ring](./source-integrity-git-gpg.md#inspecting-gnupg-key-ring).

### Local sync

As with Git source integrity, `argocd app sync --local` cannot enforce project source integrity policies.

### Common errors and solutions

#### `provenance file (.prov) is required but missing`

Cause: No `.prov` content was loaded for the chart version.

- Confirm `<chart-url>.prov` exists (for example `curl -I https://charts.example.com/mychart-1.0.0.tgz.prov`).
- If `index.yaml` lists multiple URLs, check whether a later mirror hosts the `.prov` file.

Fix: Use a signed chart release, or use a project without `sourceIntegrity.helm` if you intentionally want to skip verification for that repo.

#### `provenance signature verification failed`

Cause: `.prov` is present but PGP verification failed (bad signature, wrong format, corrupt file).

Diagnosis:

```bash
gpg --verify chart.tgz.prov
```

Fix: Obtain a valid `.prov` from the chart maintainer; ensure `ARGOCD_GPG_ENABLED` is `true` and the signer key is in the keyring.

#### `signed with unallowed key`

Cause: Signature is valid but the signer key ID is not listed in `provenance.keys`.

Diagnosis:

```bash
gpg --verify chart.tgz.prov
# Note the key ID in "Good signature from ..."
```

Fix: `argocd gpg add --from <path-to-key>` and add that key ID to `provenance.keys`.

#### `chart digest mismatch`

Cause: SHA256 of the chart `.tgz` does not match the digest in the signed `.prov` body.

Possible causes: chart changed after signing, wrong version fetched, or local/cache corruption.

Fix: `argocd app sync <app> --force` to refresh the cached chart; if the error persists, contact the chart maintainer.

#### `could not access chart for provenance verification`

Cause: Argo CD could not read the chart `.tgz` or fetch `.prov` (cache, network, or index layout).

Fix: Check repo-server logs, repository credentials, and that `helm pull` works outside Argo CD for the same chart version.

#### `multiple (N) Helm source integrity policies found for repo URL`

Cause: More than one `sourceIntegrity.helm` policy matches the application's `repoURL`.

Fix: Narrow `repos` globs so exactly one policy matches.
