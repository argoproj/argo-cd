# Git GnuPG signature verification

## Overview

Verify that commits in the source repository are correctly signed with one of the blessed GnuPG keys.

> [!NOTE]
> **A few words about trust**
>
> ArgoCD uses a very simple trust model for the keys you import: Once the key
> is imported, ArgoCD will trust it. ArgoCD does not support more complex
> trust models, and it is not necessary (nor possible) to sign the public keys
> you are going to import into ArgoCD.

> [!NOTE]
> **Compatibility notice**
>
> The GnuPG verification was first introduced in v1.7 as a project-wide constraint configured by `signatureKeys`.
> As of v**TODO**, it is supported as one of the methods for source integrity verification, but it is using a different declaration format.
> Keys configured in `signatureKeys` will continue to be supported, but they cannot be used together with `sourceIntegrity`.
> See below on how to convert the legacy `signatureKeys` configuration to `sourceIntegrity`.

Verification of GnuPG signatures is only supported with Git repositories. It is
not possible when using Helm or OCI repositories.

The GnuPG verification requires populating the Argo CD GnuPG keyring, and configuring source integrity policies for your repositories.

## Managing Argo CD GnuPG keyring

All the GnuPG keys Argo CD is going to trust must be introduced in its keyring first.

### Keyring RBAC rules

The appropriate resource notation for Argo CD's RBAC implementation to allow
the managing of GnuPG keys is `gpgkeys`.

To allow *listing* of keys for a role named `role:myrole`, use:

```
p, role:myrole, gpgkeys, get, *, allow
```

To allow *adding* keys for a role named `role:myrole`, use:

```
p, role:myrole, gpgkeys, create, *, allow
```

And finally, to allow *deletion* of keys for a role named `role:myrole`, use:

```
p, role:myrole, gpgkeys, delete, *, allow
```

### Keyring management

You can configure the GnuPG public keys that ArgoCD will use for verification
of commit signatures using either the CLI, the web UI or configuring it using
declarative setup.

> [!NOTE]
> After you have imported a GnuPG key, it may take a while until the key is
> propagated within the cluster, even if listed as configured. If you still
> cannot sync to commits signed by the already imported key, please see the
> troubleshooting section below.

#### Manage public keys using the CLI

To configure GnuPG public keys using the CLI, use the `argocd gpg` command.

##### Listing all configured keys

To list all configured keys known to ArgoCD, use the `argocd gpg list`
sub-command:

```bash
argocd gpg list
```

##### Show information about a certain key

To get information about a specific key, use the `argocd gpg get` sub-command:

```bash
argocd gpg get <key-id>
```

##### Importing a key

To import a new *public* key to ArgoCD, use the `argocd gpg add` sub-command:

```bash
argocd gpg add --from <path-to-key>
```

The key to be imported can be either in binary or ASCII-armored format.

##### Removing a key from the configuration

To remove a previously configured key from the configuration, use the
`argocd gpg rm` sub-command:

```bash
argocd gpg rm <key-id>
```

#### Manage public keys using the Web UI

Basic key management functionality for listing, importing and removing GnuPG
public keys is implemented in the Web UI. You can find the configuration
module from the **Settings** page in the **GnuPG keys** module.

Please note that when you configure keys using the Web UI, the key must be
imported in ASCII armored format for now.

#### Manage public keys in declarative setup

ArgoCD stores public keys internally in the `argocd-gpg-keys-cm` ConfigMap
resource, with the public GnuPG key's ID as its name and the ASCII armored
key data as string value, i.e. the entry for the GitHub's web-flow signing
key would look like follows:

```yaml
4AEE18F83AFDEB23: |
    -----BEGIN PGP PUBLIC KEY BLOCK-----

    mQENBFmUaEEBCACzXTDt6ZnyaVtueZASBzgnAmK13q9Urgch+sKYeIhdymjuMQta
    x15OklctmrZtqre5kwPUosG3/B2/ikuPYElcHgGPL4uL5Em6S5C/oozfkYzhwRrT
    SQzvYjsE4I34To4UdE9KA97wrQjGoz2Bx72WDLyWwctD3DKQtYeHXswXXtXwKfjQ
    7Fy4+Bf5IPh76dA8NJ6UtjjLIDlKqdxLW4atHe6xWFaJ+XdLUtsAroZcXBeWDCPa
    buXCDscJcLJRKZVc62gOZXXtPfoHqvUPp3nuLA4YjH9bphbrMWMf810Wxz9JTd3v
    yWgGqNY0zbBqeZoGv+TuExlRHT8ASGFS9SVDABEBAAG0NUdpdEh1YiAod2ViLWZs
    b3cgY29tbWl0IHNpZ25pbmcpIDxub3JlcGx5QGdpdGh1Yi5jb20+iQEiBBMBCAAW
    BQJZlGhBCRBK7hj4Ov3rIwIbAwIZAQAAmQEH/iATWFmi2oxlBh3wAsySNCNV4IPf
    DDMeh6j80WT7cgoX7V7xqJOxrfrqPEthQ3hgHIm7b5MPQlUr2q+UPL22t/I+ESF6
    9b0QWLFSMJbMSk+BXkvSjH9q8jAO0986/pShPV5DU2sMxnx4LfLfHNhTzjXKokws
    +8ptJ8uhMNIDXfXuzkZHIxoXk3rNcjDN5c5X+sK8UBRH092BIJWCOfaQt7v7wig5
    4Ra28pM9GbHKXVNxmdLpCFyzvyMuCmINYYADsC848QQFFwnd4EQnupo6QvhEVx1O
    j7wDwvuH5dCrLuLwtwXaQh0onG4583p0LGms2Mf5F+Ick6o/4peOlBoZz48=
    =Bvzs
    -----END PGP PUBLIC KEY BLOCK-----
```

## Policies for GnuPG signature verification

The GnuPG commit signature verification is configured through one or multiple Git `gpg` policies.

The policies are configured as illustrated:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
spec:
  sourceIntegrity:
    git:
      policies:
        - repos:
            - url: "https://github.com/my-group/*"
            - url: "!https://github.com/my-group/ignored.git"
          gpg:
            mode: "none|head|strict"
            keys:
              - "D56C4FCA57A46444"
```

The `repos ` key contains a list of glob-style patterns matched against the URL of the source to verify.
Given strategy will be used when matched some of the positive globs, while not matched by any of the negative ones (starting with `!`).

Only one policy is applied per source repository, and sources not matched by any policy will not have its integrity verified.

Note that a multi-source application can have each of its source repositories validated against a different policy.

### The `gpg` verification policy

The Git commit signature verification is an alternative to calling `git verify-commit`/`git verify-tag` with configured keyring and making sure the key ID used for the signatures is among the configured Key IDs in the source integrity policy.
If the target revision points to a commit or tags that do not satisfy those criteria, it will not be synced.

The `keys` key lists the set of key IDs to trust for signed commits.
If a commit in the repository is signed by an ID not specified in the list of trusted signers, the verification will fail.

The `mode` defines how thorough the GnuPG verification is:

##### Verification mode `none`

Verification is not performed for this strategy, and no following strategies are tried.

Note this accepts unsigned commits as well as commits with a signature that is invalid in some sense (expired, unverifiable, etc.).

##### Verification mode `head`

Verify only the commit/tag pointed to by the target revision of the source.
If the revision is an annotated tag, it is the tag's signature that is verified, not the commit's signature (i.e. the tag itself must be signed using `git tag -s`).
Otherwise, if target revision is a branch name, reference name (such as `HEAD`), or a commit SHA Argo CD verifies the commit's GnuPG signature.

##### Verification mode `strict`

Verify target revision and all its ancestors.
This makes sure there is no unsigned change in the history as well.
If the revision is an annotated tag, the tag's signature is verified together with the commit history, including the commit it points to.

There are situations where verifying the entire history is not practical - typically in case the history contains unsigned commits, or commits signed with keys that are no longer trusted.
This happens when GnuPG verification is introduced later to the git repository, or when formerly accepted keys get removed, revoked, or rotated.
While this can be addressed by re-signing with git rebase, there is a better way that does not require rewriting the Git history.

###### Commit seal-signing with `strict` mode

A sealing commit is a GnuPG signed commit that works as a "seal of approval" attesting that all its ancestor commits were either signed by a trusted key, or reviewed and trusted by the author of the sealing commit.
Argo CD verifying GnuPG signatures would then progres only as far back in the history as the most recent "seal" commits in each individual ancestral branch.

In practice, a commiter first *reviews* all commits that are not signed or signed with untrusted keys from the previous "seal commit" and creates a new, possibly empty commit with a custom Git trailer in its message.
Such commits can have the following organization-level semantics:

- "From now on, we are going to GnuPG sign all commits in this repository. There is no point in verifying the unsigned ones from before."
- "I merge these changes from untrusted external contributor, and I approve of them."
- "I am removing the GnuPG key of Bob. All his previous commits are trusted, but no new ones will be. Happy retirement, Bob!"
- "I am replacing my old key with a new one. Trust my commit signed with the old one before this commit, trust my new one from now on."

To create a seal commit, run `git commit --signoff --gpg-sign --trailer="Argocd-gpg-seal: <justification>"` and push to branch pulled by Argo CD.
Using seal commits is preferable to rewriting git history as it eliminates the room for eventual rebasing mistakes that would jeopardize either source integrity or correctness of the repository data.

## Upgrade to Source Integrity Verification

To migrate from the legacy declaration to the new source verification policies, remove `.spec.signatureKeys` and then define desired policies in `.spec.sourceIntegrity.git.policies`.

To achieve the legacy Argo CD verification behavior in a project, use the following config:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
spec:
  sourceIntegrity:
    git:
      policies:
        - repos:
          - url: "*" # For any repository in the project
          gpg:
            mode: "head" # Verify only the HEAD of the target revision
            keys:
              - "..." # Keys from .spec.signatureKeys
```

When `.spec.sourceIntegrity` is not defined but `.spec.signatureKeys` is, Argo CD will do similar conversion behind the scenes.
Though it is advised to perform the migration as source integrity config allows for greater flexibility, and `.spec.signatureKeys` will be a subject of removal in future releases.

## Downgrade from Source Integrity Verification

At downgrade time, reintroduce `.spec.signatureKeys` in any AppProject and populate it will all the keys from `.spec.sourceIntegrity.git.policies` at the same time.
Mind the legacy functionality lacks many of the new features—it will be all "head" mode for all project repositories.

As an alternative to downgrade, consult the troubleshooting section here.

## Legacy signature key management (DEPRECATED)

The project-wide signature keys can be managed through UI and CLI.
Note they are being replaced by the source integrity policies, so users are advised to migrate away from these.

### Configuring using the CLI (DEPRECATED)

#### Adding a key ID to the list of allowed keys

To add a key ID to the list of allowed GnuPG keys for a project, you can use
the `argocd proj add-signature-key` command, i.e. the following command would
add the key ID `4AEE18F83AFDEB23` to the project named `myproj`:

```bash
# DEPRECATED
argocd proj add-signature-key myproj 4AEE18F83AFDEB23
```

#### Removing a key ID from the list of allowed keys

Similarly, you can remove a key ID from the list of allowed GnuPG keys for a
project using the `argocd proj remove-signature-key` command, i.e. to remove
the key added above from project `myproj`, use the command:

```bash
# DEPRECATED
argocd proj remove-signature-key myproj 4AEE18F83AFDEB23
```

#### Showing allowed key IDs for a project

To see which key IDs are allowed for a given project, you can inspect the
output of the `argocd proj get` command, i.e. for a project named `gpg`:

```bash
# DEPRECATED
$ argocd proj get gpg
Name:                        gpg
Description:                 GnuPG verification
Destinations:                *,*
Repositories:                *
Allowed Cluster Resources:   */*
Denied Namespaced Resources: <none>
Signature keys:              4AEE18F83AFDEB23, 07E34825A909B250
Orphaned Resources:          disabled
```

#### Override list of key IDs

You can also explicitly set the currently allowed keys with one or more new keys
using the `argocd proj set` command in combination with the `--signature-keys`
flag, which you can use to specify a comma separated list of allowed key IDs:

```bash
# DEPRECATED
argocd proj set myproj --signature-keys 4AEE18F83AFDEB23,07E34825A909B250
```

The `--signature-keys` flag can also be used on project creation, i.e. the
`argocd proj create` command.

### Configure using the Web UI (DEPRECATED)

You can configure the GnuPG key IDs required for signature verification using
the web UI, in the Project configuration. Navigate to the **Settings** page
and select the **Projects** module, then click on the project you want to
configure.

From the project's details page, click **Edit** and find the
**Required signature keys** section, where you can add or remove the key IDs
for signature verification. After you have modified your project, click
**Update** to save the changes.

## Troubleshooting

### Disabling the feature

The GnuPG feature can be completely disabled if desired. In order to disable it,
set the environment variable `ARGOCD_GPG_ENABLED` to `false` for the pod
templates of the `argocd-server`, `argocd-repo-server`, `argocd-application-controller`
and `argocd-applicationset-controller` deployment manifests.

After the pods have been restarted, the GnuPG feature is disabled.

### Inspecting GnuPG key ring

The GnuPG key ring used for signature verification is maintained within the
pods of `argocd-repo-server`. The keys in the keyring are synchronized to the
configuration stored in the `argocd-gpg-keys-cm` ConfigMap resource, which is
volume-mounted to the `argocd-repo-server` pods.

> [!NOTE]
> The GnuPG key ring in the pods is transient and gets recreated from the
> configuration on each restart of the pods. You should never add or remove
> keys manually to the key ring in the pod, because your changes will be lost. Also,
> any of the private keys found in the key ring are transient and will be
> regenerated upon each restart. The private key is only used to build the
> trust DB for the running pod.

To check whether the keys are actually in sync, you can `kubectl exec` into the
repository server's pods and inspect the key ring, which is located at path
`/app/config/gpg/keys`

```bash
$ kubectl exec -it argocd-repo-server-7d6bdfdf6d-hzqkg bash
argocd@argocd-repo-server-7d6bdfdf6d-hzqkg:~$ GNUPGHOME=/app/config/gpg/keys gpg --list-keys
/app/config/gpg/keys/pubring.kbx
--------------------------------
pub   rsa2048 2020-06-15 [SC] [expires: 2020-12-12]
      D48F075D818A813C436914BC9324F0D2144753B1
uid           [ultimate] Anon Ymous (ArgoCD key signing key) <noreply@argoproj.io>

pub   rsa2048 2017-08-16 [SC]
      5DE3E0509C47EA3CF04A42D34AEE18F83AFDEB23
uid           [ultimate] GitHub (web-flow commit signing) <noreply@github.com>

argocd@argocd-repo-server-7d6bdfdf6d-hzqkg:~$
```

If the key ring stays out of sync with your configuration after you have added
or removed keys for a longer period of time, you might want to restart your
`argocd-repo-server` pods. If such a problem persists, please consider raising
a bug report.
