# Working with repositories

## Introduction

Since ArgoCD is a GitOps centric tool, the repositories containing your
application manifest(s) play a very vital role in the configuration of
ArgoCD.

ArgoCD supports pulling the manifests from two distinct types of repositories:

* Git repositories, such as GitHub, GitLab or privately hostes ones
* Helm repositories, such as Helm's stable charts, Harbor or Chart museum

Git repositories can hold any kind of manifests or sources that ArgoCD
supports. You can even store Helm charts in your Git repositories. Git
repositories can be connected using either HTTPS or SSH protocols.

Helm repositories, however, can only hold Helm charts by definition. Helm
repositories can only be connected using HTTPS.

!!! note
    Each application defined in ArgoCD is mapped to exactly one repository.
    It is not possible to map two or more repositories to a single
    application. If you need resources from more than one repository to define
    your application, you can look at the advanced repository topics below.

## Unconfigured vs. Configured repositories

ArgoCD differentiates between *unconfigured* and *configured* repositories.
Unconfigured repositories are those that you can access without any further
configuration, while a configured repository is required when you need to
authenticate to the repository (and don't use credential templates as
described below), or when you need additional custom settings.

Configured repositories were previously known as *private* repositories, but
have now evolved to be named *configured* repositories - because they don't
necessarily need to be private.

You don't have to configure a repository in ArgoCD in order to use it as a
manifest source for your application - you can simply specify the URL of the
repository when creating an application, as long as the repository is allowed
as a source in the 
[project's configuration](projects/#sources) and is publicly accesible or matches one of
the configured credential templates. Using an unconfigured repository as source
for your application is as simple as specifying its URL using the `--repo`
parameter to the `argocd app create` command.

!!! note
    Only Git repositories accessed using HTTPS are currently supported to be
    connected without further configuration. Git repositories connected using
    SSH must always be configured in ArgoCD as a repository or have a matching
    credential template. Helm repositories must always have an explicit
    configuration before they can be used.

Using a repository that requires further configuration as the source for an
Application requires the repository to be configured, or *connected* first.
For further information on how to connect a repository, please see below.

It is suggested that you configure each repository that you will use as an
application's source is configured in ArgoCD first.

## Repository authentication

### Available authentication methods

If your repository needs authentication to be accessed, the following methods
are currently supported:

||Basic Auth|TLS client certs|SSH private keys|
|-|-|-|-|
|Git via https|X|v1.3+|-|
|Git via ssh|-|-|X|
|Helm via https|v1.3+|v1.3+|-|

Other authentication methods, such as AWS IAM or Google ServiceAccounts, are
not (yet) supported by ArgoCD.

!!! note "Optional vs mandatory authentication"
    Authentication is optional for Git and Helm repositories connected using the
    HTTPS protocol. For Git repositories connected using SSH, authentication is
    mandatory and you need to supply a private key for these connections.

### Personal Access Token (PAT)

Some Git providers require you to use a personal access token (PAT) instead of
username/password combination when accessing the repositories hosted there
via HTTPS.

Providers known to enforce the use of PATs are:

* GitHub
* GitLab
* BitBucket

You can specify the PAT simply as the password (see below) when connecting
the custom repository to ArgoCD, using any or the empty string as the username.
The value for the username (any, empty or your actual username) varies from
provider to provider.

## Specifying repository URLs

Repository URLs should always be specified in a fully-qualified manner, that
is they should contain the protocol modifier (i.e. `https://` or `ssh://`) as
a prefix. Specifying custom ports for the connection to the repository server
is possible using the `:port` modifier in the `hostname` portion of the URL.
If a port is not specified, the default ports for the requested protocol
will be used:

* Port 443 for HTTPS connections
* Port 22 for SSH connections

Generally, URLs for repositories take the following form

```bash
protocol://[username@]hostname[:port]/path/to/repo
```

The `username` URL modifier is only valid (and mandatory!) for connecting Git
repositories using SSH. Likewise, the `--username` parameter for the appropriate
CLI commands is only valid for connecting Git or Helm repositories via HTTPS.

!!! note "Usernames for SSH repositories"
    When using SSH to connect to the repository, you *must* specify the remote
    username in the URL, i.e. using `ssh://user@example.com/your/repo`. Most
    Git providers use `git` as remote username, further information should be
    taken from the provider's documentation.

There is an exception when specifying repository URLs for repositories that
are to be connected using SSH. These URLs can also be of `scp` style syntax
in the following form:

```bash
username@hostname:path/to/repo
```

!!! warning "Remote port in SSH URLs"
    Please note that with the `scp` style syntax, it is not possible to specify
    a custom SSH server port in the URL, because the colon denominates the
    beginning of the path, and the path will be relative to the SSH server's
    working directory. If you need to connect via SSH to a non-standard port,
    you **must** use `ssh://` style URLs to specify the repository to use.

The following are some examples for valid repository URLs

* `https://example.com/yourorg/repo` - specifies repository `/yourorg/repo` on
  remote server `example.com`, connected via HTTPS on standard port.
* `https://example.com:9443/yourorg/repo` - specifies repository `/yourorg/repo`
  on remote server `example.com`, connected via HTTPS on non-standard port
  `9443`.
* `ssh://git@example.com/yourorg/repo` - specifies repository `/yourorg/repo`
  on remote server `example.com`, connected via SSH on standard port and using
  the remote username `git`.
* `git@example.com:yourorg/repo` - same as above, but denoted using an `scp`
  URL.
* `ssh://git@example.com:2222/yourorg/repo` - specifies repository
  `/yourorg/repo` on remote server `example.com`, connected via SSH on the
  non-standard port `2222` and using `git` as the remote username.

A common pitfall is the following `scp` style URL:

* `git@example.com:2222/yourorg/repo` - This would **not** specify a repository
  `/yourorg/repo` on remote server `example.com` with a non-standard port of
  `2222`, but rather the repository `2222/yourorg/repo` on the remote server
  `example.com` with the default SSH port `22`.

## Managing configured repositories

You can manage configured repositories for use with ArgoCD in three ways:

* Using the CLI's `repo` sub-command
* Using the web UI repository configuration, to be found at the `Repositories`
  module in the `Settings` sections
* Using declarative setup. For further information, please refer to the
  appropriate chapter in the
  [Operator Manual]().

With each of the methods above, you can add, edit and remove custom repositories
and their configuration.

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

#### Configuration using the CLI

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

**Some examples:**

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

#### Configuration using the web UI

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

## Managing credential templates

Credential templates are a convinient method for accessing multiple repositories
with the same set of credentials, so you don't have to configure (and possibly
change regulary) credentials for every repository that you might want to access
from ArgoCD. Instead, you set up the credentials once using the template and all
repositories whose URL matches the templated one will re-use these credentials,
as long as they don't have credentials set up specifically.

For example, you have a bunch of private repositories in the GitHub organisation
`yourorg`, all accessible using the same SSH key, you can set up a credential
template for accessing the repositories via SSH like follows:

```bash
argocd repocreds add git@github.com:yourorg/ --ssh-private-key-path yourorg.key
```

Since the URL here is a pattern, no validation of the credentials supplied will
be performed at all during creation of the template.

### Matching templates against repository URLs

Pattern matching will be done on a *best match* basis, so you can have more than
one matching pattern for any given URL. The pattern that matches best (i.e. is
the more specific) will win.

Consider you have templates for the following two patterns:

* `https://github.com/yourorg`

* `https://github.com/yourorg/special-`

Now, for the repository `https://github.com/yourorg/yourrepo`, the first pattern
would match while for the repository `https://github.com/yourorg/special-repo`
both pattern will match, but the second one will win because it is more specific.

The syntax for the `argocd repocreds` command is similar to that of the
`argocd repo` command, however it does not support any repository specific
configuration such as LFS support.

## Self-signed TLS certificates, custom CAs and SSH Known Hosts

## Advanced repository topics

### Git LFS

### Git submodules

### Separating Helm values and Helm charts