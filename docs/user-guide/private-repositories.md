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

### TLS Client Certificates for HTTPS repositories

> v1.3 and later

If your repository server requires you to use TLS client certificates for authentication, you can configure ArgoCD repositories to make use of them. For this purpose, `--tls-client-cert-path` and `--tls-client-cert-key-path` switches to the `argocd repo add` command can be used to specify the files on your local system containing client certificate and the corresponding key, respectively:

```
argocd repo add https://repo.example.com/repo.git --tls-client-cert-path ~/mycert.crt --tls-client-cert-key-path ~/mycert.key
```

Of course, you can also use this in combination with the `--username` and `--password` switches, if your repository server should require this. The options `--tls-client-cert-path` and `--tls-client-cert-key-path` must always be specified together.

!!! note
    Your client certificate and key data must be in PEM format, other formats (such as PKCS12) are not understood. Also make sure that your certificate's key is not password protected, otherwise it cannot be used by ArgoCD.

### SSH Private Key Credential

Private repositories that require an SSH private key have a URL that typically start with "git@" or "ssh://" rather than "https://".  

The Argo CD UI don't support configuring SSH credentials. The SSH credentials can only be configured using the Argo CD CLI:

```
argocd repo add git@github.com:argoproj/argocd-example-apps.git --ssh-private-key-path ~/.ssh/id_rsa
```

## Self-signed & Untrusted TLS Certificates

> v1.2 or higher

If you are connecting a repository on a HTTPS server using a self-signed certificate, or a certificate signed by a custom Certificate Authority (CA) which are not known to ArgoCD, the repository will not be added due to security reasons. This is indicated by an error message such as `x509: certificate signed by unknown authority`.

1. You can let ArgoCD connect the repository in an insecure way, without verifying the server's certificate at all. This can be accomplished by using the `--insecure-skip-server-verification` flag when adding the repository with the `argocd` CLI utility. However, this should be done only for non-production setups, as it imposes a serious security issue through possible man-in-the-middle attacks.

2. You can let ArgoCD use a custom certificate for the verification of the server's certificate using the `cert add-tls` command of the `argocd` CLI utility. This is the recommended method and suitable for production use. In order to do so, you will need the server's certificate, or the certificate of the CA used to sign the server's certificate, in PEM format.

!!! note
    For invalid server certificates, such as those without matching server name, or those that are expired, adding a CA certificate will not help. In this case, your only option will be to use the `--insecure-skip-server-verification` flag to connect the repository. You are strongly urged to use a valid certificate on the repository server, or to urge the server's administrator to replace the faulty certificate with a valid one.

Example for adding a HTTPS repository to ArgoCD without verifying the server's certificate (**Caution:** This is **not** recommended for production use):

```bash
argocd repo add --insecure-skip-server-verification https://git.example.com/test-repo
```

Example for adding a CA certificate contained in file `~/myca-cert.pem` to properly verify the repository server:

```bash
argocd cert add-tls git.example.com --from ~/myca-cert.pem
argocd repo add https://git.example.com/test-repo
```

You can also add more than one PEM for a server by concatenating them into the input stream. This might be useful if the repository server is about to replace the server certificate, possibly with one signed by a different CA. This way, you can have the old (current) as well as the new (future) certificate co-existing. If you already have the old certificate configured, use the `--upsert` flag and add the old and the new one in a single run:

```bash
cat cert1.pem cert2.pem | argocd cert add-tls git.example.com --upsert
```

!!! note
    To replace an existing certificate for a server, use the `--upsert` flag to the `cert add-tls` CLI command. 

!!! note
    TLS certificates are configured on a per-server, not on a per-repository basis. If you connect multiple repositories from the same server, you only have to configure the certificates once for this server.

!!! note
    It can take up to a couple of minutes until the changes performed by the `argocd cert` command are propagated across your cluster, depending on your Kubernetes setup.

You can also manage TLS certificates in a declarative, self-managed ArgoCD setup. All TLS certificates are stored in the ConfigMap object `argocd-tls-cert-cm`.

Managing TLS certificates via the web UI is currently not possible, but will be introduced with **v1.3**

> Before v1.2

We do not currently have first-class support for this. See [#1513](https://github.com/argoproj/argo-cd/issues/1513).

As a work-around, you can customize your Argo CD image. See [#1344](https://github.com/argoproj/argo-cd/issues/1344#issuecomment-479811810)

## Unknown SSH Hosts

If you are using a privately hosted Git service over SSH, then you have the following  options:

> v1.2 or later

1. You can let ArgoCD connect the repository in an insecure way, without verifying the server's SSH host key at all. This can be accomplished by using the `--insecure-skip-server-verification` flag when adding the repository with the `argocd` CLI utility. However, this should be done only for non-production setups, as it imposes a serious security issue through possible man-in-the-middle attacks.

2. You can make the server's SSH public key known to ArgoCD by using the `cert add-ssh` command of the `argocd` CLI utility. This is the recommended method and suitable for production use. In order to do so, you will need the server's SSH public host key, in the `known_hosts` format understood by `ssh`. You can get the server's public SSH host key e.g. by using the `ssh-keyscan` utility.

Example for adding all available SSH public host keys for a server to ArgoCD:

```bash
ssh-keyscan server.example.com | argocd cert add-ssh --batch 

```

Example for importing an existing `known_hosts` file to ArgoCD:

```bash
argocd cert add-ssh --batch --from /etc/ssh/ssh_known_hosts
```

!!! note
    It can take up to a couple of minutes until the changes performed by the `argocd cert` command are propagated across your cluster, depending on your Kubernetes setup.

You can also manage SSH known hosts entries in a declarative, self-managed ArgoCD setup. All SSH public host keys are stored in the ConfigMap object `argocd-ssh-known-hosts-cm`.

Managing SSH public host keys via the web UI is currently not possible, but will be introduced with **v1.3**

> Before v1.2

 
(1) You can customize the Argo CD Docker image by adding the host's SSH public key to `/etc/ssh/ssh_known_hosts`. Additional entries to this file can be generated using the `ssh-keyscan` utility (e.g. `ssh-keyscan your-private-git-server.com`. For more information see [example](https://github.com/argoproj/argo-cd/tree/master/examples/known-hosts) which demonstrates how `/etc/ssh/ssh_known_hosts` can be customized.

!!! note
    The `/etc/ssh/ssh_known_hosts` should include Git host on each Argo CD deployment as well as on a computer where `argocd repo add` is executed. After resolving issue
    [#1514](https://github.com/argoproj/argo-cd/issues/1514) only `argocd-repo-server` deployment has to be customized.

(1) Add repository using Argo CD CLI and `--insecure-ignore-host-key` flag:

```bash
argocd repo add git@github.com:argoproj/argocd-example-apps.git --ssh-private-key-path ~/.ssh/id_rsa --insecure-ignore-host-key 
```

!!! warning "Don't use in production"
    The `--insecure-ignore-host-key` should not be used in production as this is subject to man-in-the-middle attacks. 

!!! warning "This does not work for Kustomize remote bases or custom plugins"
    For Kustomize support, see [#827](https://github.com/argoproj/argo-cd/issues/827).

## Declarative Configuration

See [declarative setup](../../operator-manual/declarative-setup#Repositories)

