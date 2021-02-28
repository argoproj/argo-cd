# Repositories

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
