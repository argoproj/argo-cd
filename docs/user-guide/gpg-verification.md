# GnuPG signature verification

## Overview

As of v1.7 it is possible to configure ArgoCD to only sync against commits
that are signed in Git using GnuPG. Signature verification is configured on
project level.

If a project is configured to enforce signature verification, all applications
associated with this project must have the commits in the source repositories
signed with a GnuPG public key known to ArgoCD. ArgoCD will refuse to sync to
any revision that does not have a valid signature made by one of the configured
keys. The controller will emit a `ResourceComparison` error if it tries to sync
to a revision that is either not signed, or is signed by an unknown or not
allowed public key.

By default, signature verification is enabled but not enforced. If you wish to
completely disable the GnuPG functionality in ArgoCD, you have to set the
environment variable `ARGOCD_GPG_ENABLED` to `"false"` in the pod templates of
the `argocd-server`, `argocd-repo-server` and `argocd-application-controller`
deployment manifests.

Verification of GnuPG signatures is only supported with Git repositories. It is
not possible using Helm repositories.

!!!note "A few words about trust"
    ArgoCD uses a very simple trust model for the keys you import: Once the key
    is imported, ArgoCD will trust it. ArgoCD does not support more complex
    trust models, and it is not necessary (nor possible) to sign the public keys
    you are going to import into ArgoCD.

## Signature verification targets

If signature verification is enforced, ArgoCD will verify the signature using
following strategy:

* If `target revision` is a pointer to a commit object (i.e. a branch name, the
  name of a reference such as `HEAD` or a commit SHA), ArgoCD will perform the
  signature verification on the commit object the name points to, i.e. a commit.

* If `target revision` resolves to a tag and the tag is a lightweight tag, the
  behaviour is same as if `target revision` would be a pointer to a commit
  object. However, if the tag is annotated, the target revision will point to
  a *tag* object and thus, the signature verification is performed on the tag
  object, i.e. the tag itself must be signed (using `git tag -s`).

## Enforcing signature verification

To configure enforcing of signature verification, the following steps must be
performed:

* Import the GnuPG public key(s) used for signing commits in ArgoCD
* Configure a project to enforce signature verification for given keys

Once you have configured one or more keys to be required for verification for
a given project, enforcement is active for all applications associated with
this project.

!!!warning
    If signature verification is enforced, you will not be able to sync from
    local sources (i.e. `argocd app sync --local`) anymore.

## Importing GnuPG public keys

You can configure the GnuPG public keys that ArgoCD will use for verification
of commit signatures using either the CLI, the web UI or configuring it using
declarative setup.

!!!note
    After you have imported a GnuPG key, it may take a while until the key is
    propagated within the cluster, even if listed as configured. If you still
    cannot sync to commits signed by the already imported key, please see the
    troubleshooting section below.

Users wanting to manage the GnuPG public key configuration require the RBAC
permissions for `gpgkeys` resources.

### Manage public keys using the CLI

To configure GnuPG public keys using the CLI, use the `argocd gpg` command.

#### Listing all configured keys

To list all configured keys known to ArgoCD, use the `argocd gpg list`
sub-command:

```bash
argocd gpg list
```

#### Show information about a certain key

To get information about a specific key, use the `argocd gpg get` sub-command:

```bash
argocd gpg get <key-id>
```

#### Importing a key

To import a new key to ArgoCD, use the `argocd gpg add` sub-command:

```bash
argocd gpg add --from <path-to-key>
```

The key to be imported can be either in binary or ASCII-armored format.

#### Removing a key from configuration

To remove a previously configured key from the configuration, use the
`argocd gpg rm` sub-command:

```bash
argocd gpg rm <key-id>
```

### Manage public keys using the Web UI

Basic key management functionality for listing, importing and removing GnuPG
public keys is implemented in the Web UI. You can find the configuration
module from the **Settings** page in the **GnuPG keys** module.

Please note that when you configure keys using the Web UI, the key must be
imported in ASCII armored format for now.

### Manage public keys in declarative setup

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

## Configuring a project to enforce signature verification

Once you have imported the GnuPG keys to ArgoCD, you must now configure the
project to enforce the verification of commit signatures with the imported
keys.

### Configuring using the CLI

#### Adding a key ID to list of allowed keys

To add a key ID to the list of allowed GnuPG keys for a project, you can use
the `argocd proj add-signature-key` command, i.e. the following command would
add the key ID `4AEE18F83AFDEB23` to the project named `myproj`:

```bash
argocd proj add-signature-key myproj 4AEE18F83AFDEB23
```

#### Removing a key ID from the list of allowed keys

Similarily, you can remove a key ID from the list of allowed GnuPG keys for a
project using the `argocd proj remove-signature-key` command, i.e. to remove
the key added above from project `myproj`, use the command:

```bash
argocd proj remove-signature-key myproj 4AEE18F83AFDEB23
```

#### Showing allowed key IDs for a project

To see which key IDs are allowed for a given project, you can inspect the
output of the `argocd proj get` command, i.e for a project named `gpg`:

```bash
$ argocd proj get gpg
Name:                             gpg
Description:                      GnuPG verification
Destinations:                     *,*
Repositories:                     *
Whitelisted Cluster Resources:    */*
Blacklisted Namespaced Resources: <none>
Signature keys:                   4AEE18F83AFDEB23, 07E34825A909B250
Orphaned Resources:               disabled
```

#### Override list of key IDs

You can also explicitly set the currently allowed keys with one or more new keys
using the `argocd proj set` command in combination with the `--signature-keys`
flag, which you can use to specify a comma separated list of allowed key IDs:

```bash
argocd proj set myproj --signature-keys 4AEE18F83AFDEB23,07E34825A909B250
```

The `--signature-keys` flag can also be used on project creation, i.e. the
`argocd proj create` command.

### Configure using the Web UI

You can configure the GnuPG key IDs required for signature verification using
the web UI, in the Project configuration. Navigate to the **Settings** page
and select the **Projects** module, then click on the project you want to
configure.

From the project's details page, click **Edit** and find the
**Required signature keys** section, where you can add or remove the key IDs
for signature verification. After you have modified your project, click
**Update** to save the changes.

### Configure using declarative setup

You can specify the key IDs required for signature verification in the project
manifest within the `signatureKeys` section, i.e:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: gpg
  namespace: argocd
spec:
  clusterResourceWhitelist:
  - group: '*'
    kind: '*'
  description: GnuPG verification
  destinations:
  - namespace: '*'
    server: '*'
  namespaceResourceWhitelist:
  - group: '*'
    kind: '*'
  signatureKeys:
  - keyID: 4AEE18F83AFDEB23
  sourceRepos:
  - '*'
```

`signatureKeys` is an array of `SignatureKey` objects, whose only property is
`keyID` at the moment.

## Troubleshooting

### Disabling the feature

The GnuPG feature can be completely disabled if desired. In order to disable it,
set the environment variable `ARGOCD_GPG_ENABLED` to `false` for the pod
templates of the `argocd-server`, `argocd-repo-server` and
 `argocd-application-controller` deployments.

After the pods have been restarted, the GnuPG feature is disabled.

### GnuPG key ring

The GnuPG key ring used for signature verification is maintained within the
pods of `argocd-repo-server`. The keys in the keyring are synchronized to the
configuration stored in the `argocd-gpg-keys-cm` ConfigMap resource, which is
volume-mounted to the `argocd-repo-server` pods.

!!!note
    The GnuPG key ring in the pods is transient and gets recreated from the
    configuration on each restart of the pods. You should never add or remove
    keys manually to the key ring, because your changes will be lost. Also,
    any of the private keys found in the key ring are transient and will be
    regenerated upon each restart. The private key is only used to build the
    trust DB for the running pod.

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
