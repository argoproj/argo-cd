# Managing configured repositories

## Overview

You can manage configured repositories for use with ArgoCD in three ways:

* Using the CLI's `repo` sub-command
* Using the web UI repository configuration, to be found at the `Repositories`
  module in the `Settings` sections
* Using declarative setup. For further information, please refer to the
  appropriate chapter in the
  [Operator Manual]().

With each of the methods above, you can add, edit and remove custom repositories
and their configuration.

## Using the CLI

### Listing all configured repositories

You can list all currently configured repositories using the CLI:

```shell
argocd repo list
```

If you prefer to use the web UI, you find the list of configured repositories
at the `Settings` -> `Repositories` page.

### Adding a repository configuration

Connecting a repository via HTTPS (TLS) is supported for both repository
types, `git` and `helm`. The URL for a Git repository connected using HTTPS
must be fully-qualified and prefixed with the protocol, i.e. `https://`. The
URL may have an optional port modifier if the repository is served from a non
default port, i.e. `https://example.com:9443`.

!!! note "A few words on HTTP redirects"
    ArgoCD does not follow HTTP redirects when handling repositories. Some Git
    providers, notably GitLab and possibly also self-hosted GitLab, will send
    you a HTTP redirect if your repository URL is not suffixed with `.git`. If
    you receive a HTTP redirect on connecting the repository, try appending
    the `.git` suffix to your URL. For example, if you use the URL
    `https://gitlab.com/you/repo` and GitLab sends you a HTTP 301, try to use
    `https://gitlab.com/you/repo.git` as the URL to your repository.

To add a configuration for a Git repository to be connected using HTTPS, you
can use the `argocd repo add` command, specifying a repository URL starting
with `https://`. 

In its most simple form, the command

```bash
argocd repo add https://example.com/your/repo
```

will add the Git repository at `https://example.com/your/repo` to the ArgoCD
configuration. This simple form however is not different from using an
unconfigured repository, except it will give you the perks from selecting
the repository as an application's source in the UI from a dropdown list.

You can add custom configuration for the repository by using the following set
of command line switches to the `repo add` command:

|Switch|Argument|Description|
|-|-|-|
|`--insecure-skip-server-verification`|None|Disables verification of the server's TLS certificate or SSH known host signature, depending on the connection method. You do not want use this switch for production environments.|
|`--username`|`username`|Use `username` for authenticating at the server (only valid for HTTPS repositories and in combination with `--password`)|
|`--password`|`password`|Use `password` for authenticating at the server (only valid for HTTPS repositories and in combination with `--username`)|
|`--ssh-private-key-path`|`path`|Use SSH private key from `path` to authenticate at the remote repository. Only valid and also mandatory for SSH repositories. The private key will be stored in a secret on the cluster ArgoCD runs on.|
|`--type`|`type`|Specify that repository is of type `repotype`. Current possible values are `helm` and `git` (defaults to `git`)|
|`--name`|`name`|Specify the name of the repository to be `name`. This is mandatory when adding Helm repositories and optional when adding Git repositories.|
|`--tls-client-cert-path`|`path`|Specifies to read the TLS client certificate used for authentication from `path` on the local machine. The certificate will be stored in a secret on the cluster ArgoCD is running on.|
|`--tls-client-cert-key-path`|`path`|Specifies to read the key for TLS client certificate used for authentication from `path` on the local machine. The key will be stored in a secret on the cluster ArgoCD is running on.|
|`--enable-lfs`|None|Enables the support for Git Large File Storage (LFS) on the repository. Only valid for Git repositories.|

### Examples: Adding repositories via CLI

The following command adds a Git repository from `https://github.com/foo/repo`,
using `foo` as the username and `bar` as the password for authentication:

```bash
argocd repo add --username foo --password bar https://github.com/foo/repo
```

The following command uses a TLS client certificate in addition to the 
username/password combination to connect the repository. The cert is read
from `~/mycert.crt`, the corresponding key from `~/mycert.key`:

```bash
argocd repo add --username foo --password \
  --tls-client-cert-path ~/mycert.key \
  --tls-client-cert-key-path ~/mykey.key \
  https://secure.example.com/repos/myrepo
```

The following command adds the repository without any authentication, but will
ignore the TLS certificate presented by the server. Needless to say, this should
only be used for testing purposes in non-prod environments. Instead of using
this insecure option, you should consider adding the appropriate TLS certificate
or CA certificate to ArgoCD so it will be able to correctly verify the server's
certificate:

```bash
argocd repo add --insecure-skip-server-verification \
  https://self-hosted.example.com/repos/myrepo
```

Finally, the following command adds a repository using the SSH protocol, the
private SSH key from your local path `~/.ssh/id_rsa` for authentication and
`git` as the remote username:

```bash
argocd repo add --ssh-private-key-path ~/.ssh/id_rsa \
  ssh://git@example.com/yourorg/repo
```

## Using the web UI

Repositories can also be configured using the web UI. The configuration module
can be found by clicking on `Settings` and then `Repositories`.

You first need to chose what type of connection your repository should use, and
then click on the appropriate button:

![Choose repo type](/assets/repo-mgmt-ui-add.png)

The following will walk you through the dialogues for connecting the repository,
depending on which method you chose:

**SSH:**

![Connect repo using SSH](/assets/repo-mgmt-ui-add-ssh.png)

1. The name of the repository. This is optional for Git repositories.

1. The URL to the repository. This must be either a `ssh://` or `scp` style
  URL (see discussions about URLs above)

1. Paste the SSH private key to use. This must be a valid SSH private key,
  including the start and end denominators.

1. If you want to skip the server's SSH host key signature verification, tick
  this box. You should **not** use this in production environments.

1. If you require Git LFS, tick this box.

1. Click on "Connect" to connect the repository to ArgoCD.

!!! note "Note about SSH private keys"
    You should make sure that the SSH private key you are pasting does not
    contain any unintentional line breaks. If using a terminal, you should
    use `cat ~/yourkey`, mark everything including the
    `-----BEGIN OPENSSH PRIVATE KEY-----` and
    `-----END OPENSSH PRIVATE KEY-----` markers, copy the selection to your
    clipboard and paste it into the UI's field.

**HTTPS:**

![Add repository using HTTPS](/assets/repo-mgmt-ui-add-https.png)

1. The type of the repository. This can either be `git` or `helm`. Please note
  that when `helm` is selected, another input field for `Repository name` will
  appear, which you need to fill out as well.

1. The URL to the repository. This must be a `https://` URL.

1. The username to use for authenticating at the repository (optional)

1. The password to use for authenticating at the repository (optional)

1. An optional TLS client certificate to use for authentication. This should
  be a paste of the full Base64-encoded TLS certificate, including the
  `-----BEGIN CERTIFICATE-----` and `-----END CERTIFICATE-----` markers.
  The certificate will be stored in a secret on the cluster ArgoCD is running
  on.

1. If you have specified a TLS client certificate, you must provide the
  corresponding private key as well. This should be a paste of the full
  Base64-encoded private key, including the
  `-----BEGIN PRIVATE KEY-----` and `-----END PRIVATE KEY-----` markers. The
  private key will be stored in a secret on the cluster ArgoCD is running on.

1. To skip verification of the repository server's certificate, tick this box.
  Using this setting in production environments is not recommended.

1. If you require Git LFS support with this repository, tick this box.

1. Click "Connect" to validate configuration and add the repository to ArgoCD
  configuration.

### Removing a repository configuration

!!! warning
    If you remove a repository configuration that is in active use by any of
    your applications, ArgoCD will not prevent you to do so. All applications
    that use the repository whose configuration has been removed as source,
    will now access the repository as if it would be unconfigured - this could
    lead to breakage due to unaccessible manifests.

#### Remove using the CLI

To remove a repository configuration from ArgoCD using the CLI, simply issue
the following command:

```bash
argocd repo rm https://example.com/your/repo
```

#### Using the web UI

Navigate to the repositories configuration at `Settings` -> `Repositories` and
find the repository you want to unconfigure in the list of configured
repositories. Then click on the three vertical dots next to the entry and select
`Disconnect` from the dropdown, as shown on the following screenshot:

![Remove repository](/assets/repo-mgmt-ui-remove.png)

The UI will ask for your final confirmation before removing the repository from
the configuration.

