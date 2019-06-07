# Private Repositories

## Credentials

If application manifests are located in private repository then repository credentials have to be configured. Argo CD supports both HTTP and SSH Git credentials.

### HTTPS Username And Password Credential

Private repositories that require a username and password typically have a URL that start with "https://" rather than "git@" or "ssh://". 

Credentials can be configured using Argo CD CLI:

```bash
argocd repo add https://github.com/argoproj/argocd-example-apps --username <username> --password <password>
```

or UI:

1. Navigate to `Settings/Repositories`
1. Click `Connect Repo` button and enter HTTP credentials

![connect repo](../assets/connect_repo.png)

#### Access Token

Instead of using username and password you might use access token. Following instructions of your Git hosting service to generate the token:

* [Github](https://help.github.com/en/articles/creating-a-personal-access-token-for-the-command-line)
* [Gitlab](https://docs.gitlab.com/ee/user/project/deploy_tokens/)
* [Bitbucket](https://confluence.atlassian.com/bitbucketserver/personal-access-tokens-939515499.html)

Then, connect the repository using an empty string as a username and access token value as a password.

### SSH Private Key Credential

Private repositories that require an SSH private key have a URL that typically start with "git@" or "ssh://" rather than "https://".  

The Argo CD UI don't support configuring SSH credentials. The SSH credentials can only be configured using the Argo CD CLI:

```
argocd repo add git@github.com:argoproj/argocd-example-apps.git --ssh-private-key-path ~/.ssh/id_rsa
```

## HTTPS Self-signed Certificate

We do not currently have first-class support for this. See [#1513](https://github.com/argoproj/argo-cd/issues/1513).

As a work-around, you can customize your Argo CD image.

## Unknown SSH Hosts

If you are using a self-hosted Git service over SSH then you need to disable host verification for that Git host.

Following options are available:

(1) Add repository using Argo CD CLI and `--insecure-ignore-host-key` flag:

```bash
argocd repo add git@github.com:argoproj/argocd-example-apps.git --ssh-private-key-path ~/.ssh/id_rsa --insecure-ignore-host-key 
```

!!! warning "This does not work for Kustomize remote bases or custom plugins"
    This does not yet work for them. For Kustomize support, see [#827](https://github.com/argoproj/argo-cd/issues/827).
 
 You can add Git service hostname to the `/etc/ssh/ssh_known_hosts` in each Argo CD deployment and disables cert validation for Git SSL URLs. For more information see 

The flag disables certificate validation only for specified repository.

(2) You can add the Git service's hostname to the `/etc/ssh/ssh_known_hosts` in each Argo CD deployment and disables cert validation for Git SSL URLs. For more information see 
[example](https://github.com/argoproj/argo-cd/tree/master/examples/known-hosts) which demonstrates how `/etc/ssh/ssh_known_hosts` can be customized.

!!! note
    The `/etc/ssh/ssh_known_hosts` should include Git host on each Argo CD deployment as well as on a computer where `argocd repo add` is executed. After resolving issue
    [#1514](https://github.com/argoproj/argo-cd/issues/1514) only `argocd-repo-server` deployment has to be customized.

## Declarative Configuration

See [declarative setup](../operator-manual/declarative-setup#Repositories)

