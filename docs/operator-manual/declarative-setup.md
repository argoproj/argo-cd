# Declarative Setup

Argo CD applications, projects and settings can be defined declaratively using Kubernetes manifests. These can be updated using `kubectl apply`, without needing to touch the `argocd` command-line tool.

## Quick Reference

All resources, including `Application` and `AppProject` specs, have to be installed in the Argo CD namespace (by default `argocd`).

### Atomic configuration

| Sample File | Resource Name | Kind | Description |
|-------------|---------------|------|-------------|
| [`argocd-cm.yaml`](argocd-cm.yaml) | argocd-cm | ConfigMap | General Argo CD configuration |
| [`argocd-repositories.yaml`](argocd-repositories.yaml) | my-private-repo / istio-helm-repo / private-helm-repo / private-repo | Secrets | Sample repository connection details |
| [`argocd-repo-creds.yaml`](argocd-repo-creds.yaml) | argoproj-https-creds / argoproj-ssh-creds / github-creds / github-enterprise-creds | Secrets | Sample repository credential templates |
| [`argocd-cmd-params-cm.yaml`](argocd-cmd-params-cm.yaml) | argocd-cmd-params-cm | ConfigMap | Argo CD env variables configuration |
| [`argocd-secret.yaml`](argocd-secret.yaml) | argocd-secret | Secret | User Passwords, Certificates (deprecated), Signing Key, Dex secrets, Webhook secrets |
| [`argocd-rbac-cm.yaml`](argocd-rbac-cm.yaml) | argocd-rbac-cm | ConfigMap | RBAC Configuration |
| [`argocd-tls-certs-cm.yaml`](argocd-tls-certs-cm.yaml) | argocd-tls-certs-cm | ConfigMap | Custom TLS certificates for connecting Git repositories via HTTPS (v1.2 and later) |
| [`argocd-ssh-known-hosts-cm.yaml`](argocd-ssh-known-hosts-cm.yaml) | argocd-ssh-known-hosts-cm | ConfigMap | SSH known hosts data for connecting Git repositories via SSH (v1.2 and later) |

For each specific kind of ConfigMap and Secret resource, there is only a single supported resource name (as listed in the above table) - if you need to merge things you need to do it before creating them.

!!!warning "A note about ConfigMap resources"
    Be sure to annotate your ConfigMap resources using the label `app.kubernetes.io/part-of: argocd`, otherwise Argo CD will not be able to use them.

### Multiple configuration objects

| Sample File | Kind | Description |
|-------------|------|-------------|
| [`application.yaml`](application.yaml) | Application | Example application spec |
| [`project.yaml`](project.yaml) | AppProject | Example project spec |
| - | Secret | Repository credentials |

For `Application` and `AppProject` resources, the name of the resource equals the name of the application or project within Argo CD. This also means that application and project names are unique within a given Argo CD installation - you cannot have the same application name for two different applications.

## Applications

The Application CRD is the Kubernetes resource object representing a deployed application instance
in an environment. It is defined by two key pieces of information:

* `source` reference to the desired state in Git (repository, revision, path, environment)
* `destination` reference to the target cluster and namespace. For the cluster one of server or name can be used, but not both (which will result in an error). Under the hood when the server is missing, it is calculated based on the name and used for any operations.

A minimal Application spec is as follows:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    path: guestbook
  destination:
    server: https://kubernetes.default.svc
    namespace: guestbook
```

See [application.yaml](application.yaml) for additional fields. As long as you have completed the first step of [Getting Started](../getting_started.md#1-install-argo-cd), you can apply this with `kubectl apply -n argocd -f application.yaml` and Argo CD will start deploying the guestbook application.

!!! note
    The namespace must match the namespace of your Argo CD instance - typically this is `argocd`.

!!! note
    When creating an application from a Helm repository, the `chart` attribute must be specified instead of the `path` attribute within `spec.source`.

```yaml
spec:
  source:
    repoURL: https://argoproj.github.io/argo-helm
    chart: argo
```

!!! warning
    By default, deleting an application will not perform a cascade delete, which would delete its resources. You must add the finalizer if you want this behaviour - which you may well not want.

```yaml
metadata:
  finalizers:
    - resources-finalizer.argocd.argoproj.io
```

### App of Apps

You can create an app that creates other apps, which in turn can create other apps.
This allows you to declaratively manage a group of apps that can be deployed and configured in concert.

See [cluster bootstrapping](cluster-bootstrapping.md).

## Projects

The AppProject CRD is the Kubernetes resource object representing a logical grouping of applications.
It is defined by the following key pieces of information:

* `sourceRepos` reference to the repositories that applications within the project can pull manifests from.
* `destinations` reference to clusters and namespaces that applications within the project can deploy into (don't use the `name` field, only the `server` field is matched).
* `roles` list of entities with definitions of their access to resources within the project.

An example spec is as follows:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: my-project
  namespace: argocd
  # Finalizer that ensures that project is not deleted until it is not referenced by any application
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  description: Example Project
  # Allow manifests to deploy from any Git repos
  sourceRepos:
  - '*'
  # Only permit applications to deploy to the guestbook namespace in the same cluster
  destinations:
  - namespace: guestbook
    server: https://kubernetes.default.svc
  # Deny all cluster-scoped resources from being created, except for Namespace
  clusterResourceWhitelist:
  - group: ''
    kind: Namespace
  # Allow all namespaced-scoped resources to be created, except for ResourceQuota, LimitRange, NetworkPolicy
  namespaceResourceBlacklist:
  - group: ''
    kind: ResourceQuota
  - group: ''
    kind: LimitRange
  - group: ''
    kind: NetworkPolicy
  # Deny all namespaced-scoped resources from being created, except for Deployment and StatefulSet
  namespaceResourceWhitelist:
  - group: 'apps'
    kind: Deployment
  - group: 'apps'
    kind: StatefulSet
  roles:
  # A role which provides read-only access to all applications in the project
  - name: read-only
    description: Read-only privileges to my-project
    policies:
    - p, proj:my-project:read-only, applications, get, my-project/*, allow
    groups:
    - my-oidc-group
  # A role which provides sync privileges to only the guestbook-dev application, e.g. to provide
  # sync privileges to a CI system
  - name: ci-role
    description: Sync privileges for guestbook-dev
    policies:
    - p, proj:my-project:ci-role, applications, sync, my-project/guestbook-dev, allow
    # NOTE: JWT tokens can only be generated by the API server and the token is not persisted
    # anywhere by Argo CD. It can be prematurely revoked by removing the entry from this list.
    jwtTokens:
    - iat: 1535390316
```

## Repositories

!!!note
    Some Git hosters - notably GitLab and possibly on-premise GitLab instances as well - require you to
    specify the `.git` suffix in the repository URL, otherwise they will send a HTTP 301 redirect to the
    repository URL suffixed with `.git`. Argo CD will **not** follow these redirects, so you have to
    adjust your repository URL to be suffixed with `.git`.

Repository details are stored in secrets. To configure a repo, create a secret which contains repository details.
Consider using [bitnami-labs/sealed-secrets](https://github.com/bitnami-labs/sealed-secrets) to store an encrypted secret definition as a Kubernetes manifest.
Each repository must have a `url` field and, depending on whether you connect using HTTPS, SSH, or GitHub App, `username` and `password` (for HTTPS), `sshPrivateKey` (for SSH), or `githubAppPrivateKey` (for GitHub App).

!!!warning
    When using [bitnami-labs/sealed-secrets](https://github.com/bitnami-labs/sealed-secrets) the labels will be removed and have to be readded as descibed here: https://github.com/bitnami-labs/sealed-secrets#sealedsecrets-as-templates-for-secrets
    
Example for HTTPS:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: private-repo
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: git
  url: https://github.com/argoproj/private-repo
  password: my-password
  username: my-username
```

Example for SSH:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: private-repo
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: git
  url: git@github.com:argoproj/my-private-repository
  sshPrivateKey: |
    -----BEGIN OPENSSH PRIVATE KEY-----
    ...
    -----END OPENSSH PRIVATE KEY-----
```

Example for GitHub App:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: github-repo
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: git
  repo: https://github.com/argoproj/my-private-repository
  githubAppID: 1
  githubAppInstallationID: 2
  githubAppPrivateKey: |
    -----BEGIN OPENSSH PRIVATE KEY-----
    ...
    -----END OPENSSH PRIVATE KEY-----
---
apiVersion: v1
kind: Secret
metadata:
  name: github-enterprise-repo
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: git
  repo: https://ghe.example.com/argoproj/my-private-repository
  githubAppID: 1
  githubAppInstallationID: 2
  githubAppEnterpriseBaseUrl: https://ghe.example.com/api/v3
  githubAppPrivateKeySecret: |
    -----BEGIN OPENSSH PRIVATE KEY-----
    ...
    -----END OPENSSH PRIVATE KEY-----
```

!!! tip
    The Kubernetes documentation has [instructions for creating a secret containing a private key](https://kubernetes.io/docs/concepts/configuration/secret/#use-case-pod-with-ssh-keys).

### Repository Credentials

If you want to use the same credentials for multiple repositories, you can configure credential templates. Credential templates can carry the same credentials information as repositories.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: first-repo
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: git
  url: https://github.com/argoproj/private-repo
---
apiVersion: v1
kind: Secret
metadata:
  name: second-repo
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: git
  url: https://github.com/argoproj/other-private-repo
---
apiVersion: v1
kind: Secret
metadata:
  name: private-repo-creds
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repo-creds
stringData:
  type: git
  url: https://github.com/argoproj
  password: my-password
  username: my-username
```

In the above example, every repository accessed via HTTPS whose URL is prefixed with `https://github.com/argoproj` would use a username stored in the key `username` and a password stored in the key `password` of the secret `private-repo-creds` for connecting to Git.

In order for Argo CD to use a credential template for any given repository, the following conditions must be met:

* The repository must either not be configured at all, or if configured, must not contain any credential information (i.e. contain none of `sshPrivateKey`, `username`, `password` )
* The URL configured for a credential template (e.g. `https://github.com/argoproj`) must match as prefix for the repository URL (e.g. `https://github.com/argoproj/argocd-example-apps`). 

!!! note
    Matching credential template URL prefixes is done on a _best match_ effort, so the longest (best) match will take precedence. The order of definition is not important, as opposed to pre v1.4 configuration.

The following keys are valid to refer to credential secrets:

#### SSH repositories

* `sshPrivateKey` refers to the SSH private key for accessing the repositories

#### HTTPS repositories

* `username` and `password` refer to the username and/or password for accessing the repositories
* `tlsClientCertData` and `tlsClientCertKey` refer to secrets where a TLS client certificate (`tlsClientCertData`) and the corresponding private key `tlsClientCertKey` are stored for accessing the repositories

#### GitHub App repositories

* `githubAppPrivateKey` refers to the GitHub App private key for accessing the repositories
* `githubAppID` refers to the GitHub Application ID for the application you created.
* `githubAppInstallationID` refers to the Installation ID of the GitHub app you created and installed.
* `githubAppEnterpriseBaseUrl` refers to the base api URL for GitHub Enterprise (e.g. `https://ghe.example.com/api/v3`)
* `tlsClientCertData` and `tlsClientCertKey` refer to secrets where a TLS client certificate (`tlsClientCertData`) and the corresponding private key `tlsClientCertKey` are stored for accessing GitHub Enterprise if custom certificates are used.

### Repositories using self-signed TLS certificates (or are signed by custom CA)

> v1.2 or later

You can manage the TLS certificates used to verify the authenticity of your repository servers in a ConfigMap object named `argocd-tls-certs-cm`. The data section should contain a map, with the repository server's hostname part (not the complete URL) as key, and the certificate(s) in PEM format as data. So, if you connect to a repository with the URL `https://server.example.com/repos/my-repo`, you should use `server.example.com` as key. The certificate data should be either the server's certificate (in case of self-signed certificate) or the certificate of the CA that was used to sign the server's certificate. You can configure multiple certificates for each server, e.g. if you are having a certificate roll-over planned.

If there are no dedicated certificates configured for a repository server, the system's default trust store is used for validating the server's repository. This should be good enough for most (if not all) public Git repository services such as GitLab, GitHub and Bitbucket as well as most privately hosted sites which use certificates from well-known CAs, including Let's Encrypt certificates.

An example ConfigMap object:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-tls-certs-cm
  namespace: argocd
  labels:
    app.kubernetes.io/name: argocd-cm
    app.kubernetes.io/part-of: argocd
data:
  server.example.com: |
    -----BEGIN CERTIFICATE-----
    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
    XWyb96wrUlv+E8I=
    -----END CERTIFICATE-----

```

!!! note
    The `argocd-tls-certs-cm` ConfigMap will be mounted as a volume at the mount path `/app/config/tls` in the pods of `argocd-server` and `argocd-repo-server`. It will create files for each data key in the mount path directory, so above example would leave the file `/app/config/tls/server.example.com`, which contains the certificate data. It might take a while for changes in the ConfigMap to be reflected in your pods, depending on your Kubernetes configuration.

### SSH known host public keys

If you are connecting repositories via SSH, Argo CD will need to know the SSH known hosts public key of the repository servers. You can manage the SSH known hosts data in the ConfigMap named `argocd-ssh-known-hosts-cm`. This ConfigMap contains a single key/value pair, with `ssh_known_hosts` as the key and the actual public keys of the SSH servers as data. As opposed to TLS configuration, the public key(s) of each single repository server Argo CD will connect via SSH must be configured, otherwise the connections to the repository will fail. There is no fallback. The data can be copied from any existing `ssh_known_hosts` file, or from the output of the `ssh-keyscan` utility. The basic format is `<servername> <keydata>`, one entry per line.

An example ConfigMap object:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-ssh-known-hosts-cm
  namespace: argocd
  labels:
    app.kubernetes.io/name: argocd-cm
    app.kubernetes.io/part-of: argocd
data:
  ssh_known_hosts: |
    bitbucket.org ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAubiN81eDcafrgMeLzaFPsw2kNvEcqTKl/VqLat/MaB33pZy0y3rJZtnqwR2qOOvbwKZYKiEO1O6VqNEBxKvJJelCq0dTXWT5pbO2gDXC6h6QDXCaHo6pOHGPUy+YBaGQRGuSusMEASYiWunYN0vCAI8QaXnWMXNMdFP3jHAJH0eDsoiGnLPBlBp4TNm6rYI74nMzgz3B9IikW4WVK+dc8KZJZWYjAuORU3jc1c/NPskD2ASinf8v3xnfXeukU0sJ5N6m5E8VLjObPEO+mN2t/FZTMZLiFqPWc/ALSqnMnnhwrNi2rbfg/rd/IpL8Le3pSBne8+seeFVBoGqzHM9yXw==
    github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==
    gitlab.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFSMqzJeV9rUzU4kWitGjeR4PWSa29SPqJ1fVkhtj3Hw9xjLVXVYrU9QlYWrOLXBpQ6KWjbjTDTdDkoohFzgbEY=
    gitlab.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAfuCHKVTjquxvt6CM6tdG4SLp1Btn/nOeHHE5UOzRdf
    gitlab.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCsj2bNKTBSpIYDEGk9KxsGh3mySTRgMtXL583qmBpzeQ+jqCMRgBqB98u3z++J1sKlXHWfM9dyhSevkMwSbhoR8XIq/U0tCNyokEi/ueaBMCvbcTHhO7FcwzY92WK4Yt0aGROY5qX2UKSeOvuP4D6TPqKF1onrSzH9bx9XUf2lEdWT/ia1NEKjunUqu1xOB/StKDHMoX4/OKyIzuS0q/T1zOATthvasJFoPrAjkohTyaDUz2LN5JoH839hViyEG82yB+MjcFV5MU3N1l1QL3cVUCh93xSaua1N85qivl+siMkPGbO5xR/En4iEY6K2XPASUEMaieWVNTRCtJ4S8H+9
    ssh.dev.azure.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
    vs-ssh.visualstudio.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
    github.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg=
    github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl
```

!!! note
    The `argocd-ssh-known-hosts-cm` ConfigMap will be mounted as a volume at the mount path `/app/config/ssh` in the pods of `argocd-server` and `argocd-repo-server`. It will create a file `ssh_known_hosts` in that directory, which contains the SSH known hosts data used by Argo CD for connecting to Git repositories via SSH. It might take a while for changes in the ConfigMap to be reflected in your pods, depending on your Kubernetes configuration.

### Configure repositories with proxy

Proxy for your repository can be specified in the `proxy` field of the repository secret, along with other repository configurations. Argo CD uses this proxy to access the repository. Argo CD looks for the standard proxy environment variables in the repository server if the custom proxy is absent.

An example repository with proxy:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: private-repo
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: git
  url: https://github.com/argoproj/private-repo
  proxy: https://proxy-server-url:8888
  password: my-password
  username: my-username
```

### Legacy behaviour

In Argo CD version 2.0 and earlier, repositories where stored as part of the `argocd-cm` config map. For
backward-compatibility, Argo CD will still honor repositories in the config map, but this style of repository
configuration is deprecated and support for it will be removed in a future version.

```yaml
apiVersion: v1
kind: ConfigMap
data:
  repositories: |
    - url: https://github.com/argoproj/my-private-repository
      passwordSecret:
        name: my-secret
        key: password
      usernameSecret:
        name: my-secret
        key: username
  repository.credentials: |
    - url: https://github.com/argoproj
      passwordSecret:
        name: my-secret
        key: password
      usernameSecret:
        name: my-secret
        key: username
---
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  namespace: argocd
stringData:
  password: my-password
  username: my-username
```

## Clusters

Cluster credentials are stored in secrets same as repositories or repository credentials. Each secret must have label
`argocd.argoproj.io/secret-type: cluster`.

The secret data must include following fields:

* `name` - cluster name
* `server` - cluster api server url
* `namespaces` - optional comma-separated list of namespaces which are accessible in that cluster. Cluster level resources would be ignored if namespace list is not empty.
* `config` - JSON representation of following data structure:

```yaml
# Basic authentication settings
username: string
password: string
# Bearer authentication settings
bearerToken: string
# IAM authentication configuration
awsAuthConfig:
    clusterName: string
    roleARN: string
# Configure external command to supply client credentials
# See https://godoc.org/k8s.io/client-go/tools/clientcmd/api#ExecConfig
execProviderConfig:
    command: string
    args: [
      string
    ]
    env: {
      key: value
    }
    apiVersion: string
    installHint: string
# Transport layer security configuration settings
tlsClientConfig:
    # Base64 encoded PEM-encoded bytes (typically read from a client certificate file).
    caData: string
    # Base64 encoded PEM-encoded bytes (typically read from a client certificate file).
    certData: string
    # Server should be accessed without verifying the TLS certificate
    insecure: boolean
    # Base64 encoded PEM-encoded bytes (typically read from a client certificate key file).
    keyData: string
    # ServerName is passed to the server for SNI and is used in the client to check server
    # certificates against. If ServerName is empty, the hostname used to contact the
    # server is used.
    serverName: string
```

Note that if you specify a command to run under `execProviderConfig`, that command must be available in the Argo CD image. See [BYOI (Build Your Own Image)](custom_tools.md#byoi-build-your-own-image).

Cluster secret example:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mycluster-secret
  labels:
    argocd.argoproj.io/secret-type: cluster
type: Opaque
stringData:
  name: mycluster.com
  server: https://mycluster.com
  config: |
    {
      "bearerToken": "<authentication token>",
      "tlsClientConfig": {
        "insecure": false,
        "caData": "<base64 encoded certificate>"
      }
    }
```

## Helm Chart Repositories

Non standard Helm Chart repositories have to be registered explicitly.
Each repository must have `url`, `type` and `name` fields. For private Helm repos you may need to configure access credentials and HTTPS settings using `username`, `password`,
`tlsClientCertData` and `tlsClientCertKey` fields.

Example:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: istio
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  name: istio.io
  url: https://storage.googleapis.com/istio-prerelease/daily-build/master-latest-daily/charts
  type: helm
---
apiVersion: v1
kind: Secret
metadata:
  name: argo-helm
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  name: argo
  url: https://argoproj.github.io/argo-helm
  type: helm
  username: my-username
  password: my-password
  tlsClientCertData: ...
  tlsClientCertKey: ...
```

## Resource Exclusion/Inclusion

Resources can be excluded from discovery and sync so that Argo CD is unaware of them. For example, `events.k8s.io` and `metrics.k8s.io` are always excluded. Use cases:

* You have temporal issues and you want to exclude problematic resources.
* There are many of a kind of resources that impacts Argo CD's performance.
* Restrict Argo CD's access to certain kinds of resources, e.g. secrets. See [security.md#cluster-rbac](security.md#cluster-rbac).

To configure this, edit the `argcd-cm` config map:

```shell
kubectl edit configmap argocd-cm -n argocd
```

Add `resource.exclusions`, e.g.:

```yaml
apiVersion: v1
data:
  resource.exclusions: |
    - apiGroups:
      - "*"
      kinds:
      - "*"
      clusters:
      - https://192.168.0.20
kind: ConfigMap
```

The `resource.exclusions` node is a list of objects. Each object can have:

* `apiGroups` A list of globs to match the API group.
* `kinds` A list of kinds to match. Can be `"*"` to match all.
* `clusters` A list of globs to match the cluster.

If all three match, then the resource is ignored.

In addition to exclusions, you might configure the list of included resources using the `resource.inclusions` setting.
By default, all resource group/kinds are included. The `resource.inclusions` setting allows customizing the list of included group/kinds: 

```yaml
apiVersion: v1
data:
  resource.inclusions: |
    - apiGroups:
      - "*"
      kinds:
      - Deployment
      clusters:
      - https://192.168.0.20
kind: ConfigMap
```

The `resource.inclusions` and `resource.exclusions` might be used together. The final list of resources includes group/kinds specified in `resource.inclusions` minus group/kinds
specified in `resource.exclusions` setting.

Notes:

* Quote globs in your YAML to avoid parsing errors.
* Invalid globs result in the whole rule being ignored.
* If you add a rule that matches existing resources, these will appear in the interface as `OutOfSync`.

## SSO & RBAC

* SSO configuration details: [SSO](./user-management/index.md)
* RBAC configuration details: [RBAC](./rbac.md)

## Manage Argo CD Using Argo CD

Argo CD is able to manage itself since all settings are represented by Kubernetes manifests. The suggested way is to create [Kustomize](https://github.com/kubernetes-sigs/kustomize)
based application which uses base Argo CD manifests from [https://github.com/argoproj/argo-cd](https://github.com/argoproj/argo-cd/tree/stable/manifests) and apply required changes on top.

Example of `kustomization.yaml`:

```yaml
bases:
- github.com/argoproj/argo-cd//manifests/cluster-install?ref=v1.0.1

# additional resources like ingress rules, cluster and repository secrets.
resources:
- clusters-secrets.yaml
- repos-secrets.yaml

# changes to config maps
patchesStrategicMerge:
- overlays/argo-cd-cm.yaml
```

The live example of self managed Argo CD config is available at [https://cd.apps.argoproj.io](https://cd.apps.argoproj.io) and with configuration
stored at [argoproj/argoproj-deployments](https://github.com/argoproj/argoproj-deployments/tree/master/argocd).

!!! note
    You will need to sign-in using your GitHub account to get access to [https://cd.apps.argoproj.io](https://cd.apps.argoproj.io)
