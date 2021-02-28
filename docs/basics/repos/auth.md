# Repository authentication

## Available authentication methods

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

## Personal Access Token (PAT)

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

## Credential templates

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
