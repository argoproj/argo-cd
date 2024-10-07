# Declarative Setup

Argo CD applications, projects and settings can be defined declaratively using Kubernetes manifests. These can be updated using `kubectl apply`, without needing to touch the `argocd` command-line tool.

## Quick Reference

All resources, including `Application` and `AppProject` specs, have to be installed in the Argo CD namespace (by default `argocd`).

### Atomic configuration

| Sample File                                                           | Resource Name                                                                      | Kind      | Description                                                                          |
|-----------------------------------------------------------------------|------------------------------------------------------------------------------------|-----------|--------------------------------------------------------------------------------------|
| [`argocd-cm.yaml`](argocd-cm-yaml.md)                                 | argocd-cm                                                                          | ConfigMap | General Argo CD configuration                                                        |
| [`argocd-repositories.yaml`](argocd-repositories-yaml.md)             | my-private-repo / istio-helm-repo / private-helm-repo / private-repo               | Secrets   | Sample repository connection details                                                 |
| [`argocd-repo-creds.yaml`](argocd-repo-creds-yaml.md)                    | argoproj-https-creds / argoproj-ssh-creds / github-creds / github-enterprise-creds | Secrets   | Sample repository credential templates                                               |
| [`argocd-cmd-params-cm.yaml`](argocd-cmd-params-cm-yaml.md)           | argocd-cmd-params-cm                                                               | ConfigMap | Argo CD env variables configuration                                                  |
| [`argocd-secret.yaml`](argocd-secret-yaml.md)                         | argocd-secret                                                                      | Secret    | User Passwords, Certificates (deprecated), Signing Key, Dex secrets, Webhook secrets |
| [`argocd-rbac-cm.yaml`](argocd-rbac-cm-yaml.md)                       | argocd-rbac-cm                                                                     | ConfigMap | RBAC Configuration                                                                   |
| [`argocd-tls-certs-cm.yaml`](argocd-tls-certs-cm-yaml.md)             | argocd-tls-certs-cm                                                                | ConfigMap | Custom TLS certificates for connecting Git repositories via HTTPS (v1.2 and later)   |
| [`argocd-ssh-known-hosts-cm.yaml`](argocd-ssh-known-hosts-cm-yaml.md) | argocd-ssh-known-hosts-cm                                                          | ConfigMap | SSH known hosts data for connecting Git repositories via SSH (v1.2 and later)        |

For each specific kind of ConfigMap and Secret resource, there is only a single supported resource name (as listed in the above table) - if you need to merge things you need to do it before creating them.

!!!warning "A note about ConfigMap resources"
    Be sure to annotate your ConfigMap resources using the label `app.kubernetes.io/part-of: argocd`, otherwise Argo CD will not be able to use them.

### Multiple configuration objects

| Sample File                                                      | Kind        | Description              |
|------------------------------------------------------------------|-------------|--------------------------|
| [`application.yaml`](../user-guide/application-specification.md) | Application | Example application spec |
| [`project.yaml`](./project-specification.md)                     | AppProject  | Example project spec     |
| [`argocd-repositories.yaml`](./argocd-repositories-yaml.md)                                                                | Secret      | Repository credentials   |

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
    Without the `resources-finalizer.argocd.argoproj.io` finalizer, deleting an application will not delete the resources it manages. To perform a cascading delete, you must add the finalizer. See [App Deletion](../user-guide/app_deletion.md#about-the-deletion-finalizer).

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
* `destinations` reference to clusters and namespaces that applications within the project can deploy into.
* `roles` list of entities with definitions of their access to resources within the project.

!!!warning "Projects which can deploy to the Argo CD namespace grant admin access"
    If a Project's `destinations` configuration allows deploying to the namespace in which Argo CD is installed, then
    Applications under that project have admin-level access. [RBAC access](https://argo-cd.readthedocs.io/en/stable/operator-manual/rbac/)
    to admin-level Projects should be carefully restricted, and push access to allowed `sourceRepos` should be limited
    to only admins.

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
Credentials can be scoped to a project using the optional `project` field. When omitted, the credential will be used as the default for all projects without a scoped credential.

!!!warning
    When using [bitnami-labs/sealed-secrets](https://github.com/bitnami-labs/sealed-secrets) the labels will be removed and have to be readded as described here: https://github.com/bitnami-labs/sealed-secrets#sealedsecrets-as-templates-for-secrets

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
  project: my-project
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
  url: git@github.com:argoproj/my-private-repository.git
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
  url: https://github.com/argoproj/my-private-repository
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
  url: https://ghe.example.com/argoproj/my-private-repository
  githubAppID: 1
  githubAppInstallationID: 2
  githubAppEnterpriseBaseUrl: https://ghe.example.com/api/v3
  githubAppPrivateKey: |
    -----BEGIN OPENSSH PRIVATE KEY-----
    ...
    -----END OPENSSH PRIVATE KEY-----
```

Example for Google Cloud Source repositories:

```yaml
kind: Secret
metadata:
  name: github-repo
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: git
  url: https://source.developers.google.com/p/my-google-project/r/my-repo
  gcpServiceAccountKey: |
    {
      "type": "service_account",
      "project_id": "my-google-project",
      "private_key_id": "REDACTED",
      "private_key": "-----BEGIN PRIVATE KEY-----\nREDACTED\n-----END PRIVATE KEY-----\n",
      "client_email": "argocd-service-account@my-google-project.iam.gserviceaccount.com",
      "client_id": "REDACTED",
      "auth_uri": "https://accounts.google.com/o/oauth2/auth",
      "token_uri": "https://oauth2.googleapis.com/token",
      "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
      "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/argocd-service-account%40my-google-project.iam.gserviceaccount.com"
    }
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
    MIIF1zCCA7+gAwIBAgIUQdTcSHY2Sxd3Tq/v1eIEZPCNbOowDQYJKoZIhvcNAQEL
    BQAwezELMAkGA1UEBhMCREUxFTATBgNVBAgMDExvd2VyIFNheG9ueTEQMA4GA1UE
    BwwHSGFub3ZlcjEVMBMGA1UECgwMVGVzdGluZyBDb3JwMRIwEAYDVQQLDAlUZXN0
    c3VpdGUxGDAWBgNVBAMMD2Jhci5leGFtcGxlLmNvbTAeFw0xOTA3MDgxMzU2MTda
    Fw0yMDA3MDcxMzU2MTdaMHsxCzAJBgNVBAYTAkRFMRUwEwYDVQQIDAxMb3dlciBT
    YXhvbnkxEDAOBgNVBAcMB0hhbm92ZXIxFTATBgNVBAoMDFRlc3RpbmcgQ29ycDES
    MBAGA1UECwwJVGVzdHN1aXRlMRgwFgYDVQQDDA9iYXIuZXhhbXBsZS5jb20wggIi
    MA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQCv4mHMdVUcafmaSHVpUM0zZWp5
    NFXfboxA4inuOkE8kZlbGSe7wiG9WqLirdr39Ts+WSAFA6oANvbzlu3JrEQ2CHPc
    CNQm6diPREFwcDPFCe/eMawbwkQAPVSHPts0UoRxnpZox5pn69ghncBR+jtvx+/u
    P6HdwW0qqTvfJnfAF1hBJ4oIk2AXiip5kkIznsAh9W6WRy6nTVCeetmIepDOGe0G
    ZJIRn/OfSz7NzKylfDCat2z3EAutyeT/5oXZoWOmGg/8T7pn/pR588GoYYKRQnp+
    YilqCPFX+az09EqqK/iHXnkdZ/Z2fCuU+9M/Zhrnlwlygl3RuVBI6xhm/ZsXtL2E
    Gxa61lNy6pyx5+hSxHEFEJshXLtioRd702VdLKxEOuYSXKeJDs1x9o6cJ75S6hko
    Ml1L4zCU+xEsMcvb1iQ2n7PZdacqhkFRUVVVmJ56th8aYyX7KNX6M9CD+kMpNm6J
    kKC1li/Iy+RI138bAvaFplajMF551kt44dSvIoJIbTr1LigudzWPqk31QaZXV/4u
    kD1n4p/XMc9HYU/was/CmQBFqmIZedTLTtK7clkuFN6wbwzdo1wmUNgnySQuMacO
    gxhHxxzRWxd24uLyk9Px+9U3BfVPaRLiOPaPoC58lyVOykjSgfpgbus7JS69fCq7
    bEH4Jatp/10zkco+UQIDAQABo1MwUTAdBgNVHQ4EFgQUjXH6PHi92y4C4hQpey86
    r6+x1ewwHwYDVR0jBBgwFoAUjXH6PHi92y4C4hQpey86r6+x1ewwDwYDVR0TAQH/
    BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAgEAFE4SdKsX9UsLy+Z0xuHSxhTd0jfn
    Iih5mtzb8CDNO5oTw4z0aMeAvpsUvjJ/XjgxnkiRACXh7K9hsG2r+ageRWGevyvx
    CaRXFbherV1kTnZw4Y9/pgZTYVWs9jlqFOppz5sStkfjsDQ5lmPJGDii/StENAz2
    XmtiPOgfG9Upb0GAJBCuKnrU9bIcT4L20gd2F4Y14ccyjlf8UiUi192IX6yM9OjT
    +TuXwZgqnTOq6piVgr+FTSa24qSvaXb5z/mJDLlk23npecTouLg83TNSn3R6fYQr
    d/Y9eXuUJ8U7/qTh2Ulz071AO9KzPOmleYPTx4Xty4xAtWi1QE5NHW9/Ajlv5OtO
    OnMNWIs7ssDJBsB7VFC8hcwf79jz7kC0xmQqDfw51Xhhk04kla+v+HZcFW2AO9so
    6ZdVHHQnIbJa7yQJKZ+hK49IOoBR6JgdB5kymoplLLiuqZSYTcwSBZ72FYTm3iAr
    jzvt1hxpxVDmXvRnkhRrIRhK4QgJL0jRmirBjDY+PYYd7bdRIjN7WNZLFsgplnS8
    9w6CwG32pRlm0c8kkiQ7FXA6BYCqOsDI8f1VGQv331OpR2Ck+FTv+L7DAmg6l37W
    +LB9LGh4OAp68ImTjqf6ioGKG0RBSznwME+r4nXtT1S/qLR6ASWUS4ViWRhbRlNK
    XWyb96wrUlv+E8I=
    -----END CERTIFICATE-----

```

!!! note
    The `argocd-tls-certs-cm` ConfigMap will be mounted as a volume at the mount path `/app/config/tls` in the pods of `argocd-server` and `argocd-repo-server`. It will create files for each data key in the mount path directory, so above example would leave the file `/app/config/tls/server.example.com`, which contains the certificate data. It might take a while for changes in the ConfigMap to be reflected in your pods, depending on your Kubernetes configuration.

### SSH known host public keys

If you are configuring repositories to use SSH, Argo CD will need to know their SSH public keys. In order for Argo CD to connect via SSH the public key(s) for each repository server must be pre-configured in Argo CD (unlike TLS configuration), otherwise the connections to the repository will fail.

You can manage the SSH known hosts data in the `argocd-ssh-known-hosts-cm` ConfigMap. This ConfigMap contains a single entry, `ssh_known_hosts`, with the public keys of the SSH servers as its value. The value can be filled in from any existing `ssh_known_hosts` file, or from the output of the `ssh-keyscan` utility (which is part of OpenSSH's client package). The basic format is `<server_name> <keytype> <base64-encoded_key>`, one entry per line.

Here is an example of running `ssh-keyscan`:
```bash
$ for host in bitbucket.org github.com gitlab.com ssh.dev.azure.com vs-ssh.visualstudio.com ; do ssh-keyscan $host 2> /dev/null ; done
bitbucket.org ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDQeJzhupRu0u0cdegZIa8e86EG2qOCsIsD1Xw0xSeiPDlCr7kq97NLmMbpKTX6Esc30NuoqEEHCuc7yWtwp8dI76EEEB1VqY9QJq6vk+aySyboD5QF61I/1WeTwu+deCbgKMGbUijeXhtfbxSxm6JwGrXrhBdofTsbKRUsrN1WoNgUa8uqN1Vx6WAJw1JHPhglEGGHea6QICwJOAr/6mrui/oB7pkaWKHj3z7d1IC4KWLtY47elvjbaTlkN04Kc/5LFEirorGYVbt15kAUlqGM65pk6ZBxtaO3+30LVlORZkxOh+LKL/BvbZ/iRNhItLqNyieoQj/uh/7Iv4uyH/cV/0b4WDSd3DptigWq84lJubb9t/DnZlrJazxyDCulTmKdOR7vs9gMTo+uoIrPSb8ScTtvw65+odKAlBj59dhnVp9zd7QUojOpXlL62Aw56U4oO+FALuevvMjiWeavKhJqlR7i5n9srYcrNV7ttmDw7kf/97P5zauIhxcjX+xHv4M=
github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl
github.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCj7ndNxQowgcQnjshcLrqPEiiphnt+VTTvDP6mHBL9j1aNUkY4Ue1gvwnGLVlOhGeYrnZaMgRK6+PKCUXaDbC7qtbW8gIkhL7aGCsOr/C56SJMy/BCZfxd1nWzAOxSDPgVsmerOBYfNqltV9/hWCqBywINIR+5dIg6JTJ72pcEpEjcYgXkE2YEFXV1JHnsKgbLWNlhScqb2UmyRkQyytRLtL+38TGxkxCflmO+5Z8CSSNY7GidjMIZ7Q4zMjA2n1nGrlTDkzwDCsw+wqFPGQA179cnfGWOWRVruj16z6XyvxvjJwbz0wQZ75XK5tKSb7FNyeIEs4TT4jk+S4dhPeAUC5y+bDYirYgM4GC7uEnztnZyaVWQ7B381AK4Qdrwt51ZqExKbQpTUNn+EjqoTwvqNj4kqx5QUCI0ThS/YkOxJCXmPUWZbhjpCg56i+2aB6CmK2JGhn57K5mj0MNdBXA4/WnwH6XoPWJzK5Nyu2zB3nAZp+S5hpQs+p1vN1/wsjk=
github.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg=
gitlab.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFSMqzJeV9rUzU4kWitGjeR4PWSa29SPqJ1fVkhtj3Hw9xjLVXVYrU9QlYWrOLXBpQ6KWjbjTDTdDkoohFzgbEY=
gitlab.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAfuCHKVTjquxvt6CM6tdG4SLp1Btn/nOeHHE5UOzRdf
gitlab.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCsj2bNKTBSpIYDEGk9KxsGh3mySTRgMtXL583qmBpzeQ+jqCMRgBqB98u3z++J1sKlXHWfM9dyhSevkMwSbhoR8XIq/U0tCNyokEi/ueaBMCvbcTHhO7FcwzY92WK4Yt0aGROY5qX2UKSeOvuP4D6TPqKF1onrSzH9bx9XUf2lEdWT/ia1NEKjunUqu1xOB/StKDHMoX4/OKyIzuS0q/T1zOATthvasJFoPrAjkohTyaDUz2LN5JoH839hViyEG82yB+MjcFV5MU3N1l1QL3cVUCh93xSaua1N85qivl+siMkPGbO5xR/En4iEY6K2XPASUEMaieWVNTRCtJ4S8H+9
ssh.dev.azure.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
vs-ssh.visualstudio.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
```

Here is an example `ConfigMap` object using the output from `ssh-keyscan` above:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app.kubernetes.io/name: argocd-ssh-known-hosts-cm
    app.kubernetes.io/part-of: argocd
  name: argocd-ssh-known-hosts-cm
data:
  ssh_known_hosts: |
    # This file was automatically generated by hack/update-ssh-known-hosts.sh. DO NOT EDIT
    [ssh.github.com]:443 ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg=
    [ssh.github.com]:443 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl
    [ssh.github.com]:443 ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCj7ndNxQowgcQnjshcLrqPEiiphnt+VTTvDP6mHBL9j1aNUkY4Ue1gvwnGLVlOhGeYrnZaMgRK6+PKCUXaDbC7qtbW8gIkhL7aGCsOr/C56SJMy/BCZfxd1nWzAOxSDPgVsmerOBYfNqltV9/hWCqBywINIR+5dIg6JTJ72pcEpEjcYgXkE2YEFXV1JHnsKgbLWNlhScqb2UmyRkQyytRLtL+38TGxkxCflmO+5Z8CSSNY7GidjMIZ7Q4zMjA2n1nGrlTDkzwDCsw+wqFPGQA179cnfGWOWRVruj16z6XyvxvjJwbz0wQZ75XK5tKSb7FNyeIEs4TT4jk+S4dhPeAUC5y+bDYirYgM4GC7uEnztnZyaVWQ7B381AK4Qdrwt51ZqExKbQpTUNn+EjqoTwvqNj4kqx5QUCI0ThS/YkOxJCXmPUWZbhjpCg56i+2aB6CmK2JGhn57K5mj0MNdBXA4/WnwH6XoPWJzK5Nyu2zB3nAZp+S5hpQs+p1vN1/wsjk=
    bitbucket.org ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBPIQmuzMBuKdWeF4+a2sjSSpBK0iqitSQ+5BM9KhpexuGt20JpTVM7u5BDZngncgrqDMbWdxMWWOGtZ9UgbqgZE=
    bitbucket.org ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIazEu89wgQZ4bqs3d63QSMzYVa0MuJ2e2gKTKqu+UUO
    bitbucket.org ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDQeJzhupRu0u0cdegZIa8e86EG2qOCsIsD1Xw0xSeiPDlCr7kq97NLmMbpKTX6Esc30NuoqEEHCuc7yWtwp8dI76EEEB1VqY9QJq6vk+aySyboD5QF61I/1WeTwu+deCbgKMGbUijeXhtfbxSxm6JwGrXrhBdofTsbKRUsrN1WoNgUa8uqN1Vx6WAJw1JHPhglEGGHea6QICwJOAr/6mrui/oB7pkaWKHj3z7d1IC4KWLtY47elvjbaTlkN04Kc/5LFEirorGYVbt15kAUlqGM65pk6ZBxtaO3+30LVlORZkxOh+LKL/BvbZ/iRNhItLqNyieoQj/uh/7Iv4uyH/cV/0b4WDSd3DptigWq84lJubb9t/DnZlrJazxyDCulTmKdOR7vs9gMTo+uoIrPSb8ScTtvw65+odKAlBj59dhnVp9zd7QUojOpXlL62Aw56U4oO+FALuevvMjiWeavKhJqlR7i5n9srYcrNV7ttmDw7kf/97P5zauIhxcjX+xHv4M=
    github.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg=
    github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl
    github.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCj7ndNxQowgcQnjshcLrqPEiiphnt+VTTvDP6mHBL9j1aNUkY4Ue1gvwnGLVlOhGeYrnZaMgRK6+PKCUXaDbC7qtbW8gIkhL7aGCsOr/C56SJMy/BCZfxd1nWzAOxSDPgVsmerOBYfNqltV9/hWCqBywINIR+5dIg6JTJ72pcEpEjcYgXkE2YEFXV1JHnsKgbLWNlhScqb2UmyRkQyytRLtL+38TGxkxCflmO+5Z8CSSNY7GidjMIZ7Q4zMjA2n1nGrlTDkzwDCsw+wqFPGQA179cnfGWOWRVruj16z6XyvxvjJwbz0wQZ75XK5tKSb7FNyeIEs4TT4jk+S4dhPeAUC5y+bDYirYgM4GC7uEnztnZyaVWQ7B381AK4Qdrwt51ZqExKbQpTUNn+EjqoTwvqNj4kqx5QUCI0ThS/YkOxJCXmPUWZbhjpCg56i+2aB6CmK2JGhn57K5mj0MNdBXA4/WnwH6XoPWJzK5Nyu2zB3nAZp+S5hpQs+p1vN1/wsjk=
    gitlab.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFSMqzJeV9rUzU4kWitGjeR4PWSa29SPqJ1fVkhtj3Hw9xjLVXVYrU9QlYWrOLXBpQ6KWjbjTDTdDkoohFzgbEY=
    gitlab.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAfuCHKVTjquxvt6CM6tdG4SLp1Btn/nOeHHE5UOzRdf
    gitlab.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCsj2bNKTBSpIYDEGk9KxsGh3mySTRgMtXL583qmBpzeQ+jqCMRgBqB98u3z++J1sKlXHWfM9dyhSevkMwSbhoR8XIq/U0tCNyokEi/ueaBMCvbcTHhO7FcwzY92WK4Yt0aGROY5qX2UKSeOvuP4D6TPqKF1onrSzH9bx9XUf2lEdWT/ia1NEKjunUqu1xOB/StKDHMoX4/OKyIzuS0q/T1zOATthvasJFoPrAjkohTyaDUz2LN5JoH839hViyEG82yB+MjcFV5MU3N1l1QL3cVUCh93xSaua1N85qivl+siMkPGbO5xR/En4iEY6K2XPASUEMaieWVNTRCtJ4S8H+9
    ssh.dev.azure.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
    vs-ssh.visualstudio.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
```

!!! note
    The `argocd-ssh-known-hosts-cm` ConfigMap will be mounted as a volume at the mount path `/app/config/ssh` in the pods of `argocd-server` and `argocd-repo-server`. It will create a file `ssh_known_hosts` in that directory, which contains the SSH known hosts data used by Argo CD for connecting to Git repositories via SSH. It might take a while for changes in the ConfigMap to be reflected in your pods, depending on your Kubernetes configuration.

### Configure repositories with proxy

Proxy for your repository can be specified in the `proxy` field of the repository secret, along with a corresponding `noProxy` config. Argo CD uses this proxy/noProxy config to access the repository and do related helm/kustomize operations. Argo CD looks for the standard proxy environment variables in the repository server if the custom proxy config is absent.

An example repository with proxy and noProxy:

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
  noProxy: ".internal.example.com,company.org,10.123.0.0/16"
  password: my-password
  username: my-username
```

A note on noProxy: Argo CD uses exec to interact with different tools such as helm and kustomize. Not all of these tools support the same noProxy syntax as the [httpproxy go package](https://cs.opensource.google/go/x/net/+/internal-branch.go1.21-vendor:http/httpproxy/proxy.go;l=38-50) does. In case you run in trouble with noProxy not beeing respected you might want to try using the full domain instead of a wildcard pattern or IP range to find a common syntax that all tools support.

### Legacy behaviour

In Argo CD version 2.0 and earlier, repositories were stored as part of the `argocd-cm` config map. For
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
* `clusterResources` - optional boolean string (`"true"` or `"false"`) determining whether Argo CD can manage cluster-level resources on this cluster. This setting is used only if the list of managed namespaces is not empty.
* `project` - optional string to designate this as a project-scoped cluster.
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
    profile: string
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
  name: mycluster.example.com
  server: https://mycluster.example.com
  config: |
    {
      "bearerToken": "<authentication token>",
      "tlsClientConfig": {
        "insecure": false,
        "caData": "<base64 encoded certificate>"
      }
    }
```

### EKS

EKS cluster secret example using argocd-k8s-auth and [IRSA](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mycluster-secret
  labels:
    argocd.argoproj.io/secret-type: cluster
type: Opaque
stringData:
  name: "mycluster.example.com"
  server: "https://mycluster.example.com"
  config: |
    {
      "awsAuthConfig": {
        "clusterName": "my-eks-cluster-name",
        "roleARN": "arn:aws:iam::<AWS_ACCOUNT_ID>:role/<IAM_ROLE_NAME>"
      },
      "tlsClientConfig": {
        "insecure": false,
        "caData": "<base64 encoded certificate>"
      }        
    }
```

Note that you should have IRSA enabled on your EKS cluster, create an appropriate IAM role which allows it to assume 
other IAM roles (whichever `roleARN`s that Argo CD needs to assume) and have an assume role policy which allows 
the argocd-application-controller and argocd-server pods to assume said role via OIDC.

Example trust relationship config for `<arn:aws:iam::<AWS_ACCOUNT_ID>:role/<ARGO_CD_MANAGEMENT_IAM_ROLE_NAME>`, which 
is required for Argo CD to perform actions via IAM. Ensure that the cluster has an [IAM OIDC provider configured](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html) 
for it.  

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Federated": "arn:aws:iam::<AWS_ACCOUNT_ID>:oidc-provider/oidc.eks.<AWS_REGION>.amazonaws.com/id/EXAMPLED539D4633E53DE1B71EXAMPLE"
            },
            "Action": "sts:AssumeRoleWithWebIdentity",
            "Condition": {
                "StringEquals": {
                    "oidc.eks.<AWS_REGION>.amazonaws.com/id/EXAMPLED539D4633E53DE1B71EXAMPLE:sub": ["system:serviceaccount:argocd:argocd-application-controller", "system:serviceaccount:argocd:argocd-server"],
                    "oidc.eks.<AWS_REGION>.amazonaws.com/id/EXAMPLED539D4633E53DE1B71EXAMPLE:aud": "sts.amazonaws.com"
                }
            }
        }
    ]
}
```

The Argo CD management role also needs to be allowed to assume other roles, in this case we want it to assume 
`arn:aws:iam::<AWS_ACCOUNT_ID>:role/<IAM_ROLE_NAME>` so that it can manage the cluster mapped to that role. This can be 
extended to allow assumption of multiple roles, either as an explicit array of role ARNs or by using `*` where appropriate.

```json
{
    "Version" : "2012-10-17",
    "Statement" : {
      "Effect" : "Allow",
      "Action" : "sts:AssumeRole",
      "Resource" : [
        "<arn:aws:iam::<AWS_ACCOUNT_ID>:role/<IAM_ROLE_NAME>"
      ]
    }
  }
```

Example service account configs for `argocd-application-controller` and `argocd-server`.

!!! warning
    Once the annotations have been set on the service accounts, both the application controller and server pods need to be restarted.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  annotations:
    eks.amazonaws.com/role-arn: "<arn:aws:iam::<AWS_ACCOUNT_ID>:role/<ARGO_CD_MANAGEMENT_IAM_ROLE_NAME>"
  name: argocd-application-controller
---
apiVersion: v1
kind: ServiceAccount
metadata:
  annotations:
    eks.amazonaws.com/role-arn: "<arn:aws:iam::<AWS_ACCOUNT_ID>:role/<ARGO_CD_MANAGEMENT_IAM_ROLE_NAME>"
  name: argocd-server
```

In turn, the `roleARN` of each managed cluster needs to be added to each respective cluster's `aws-auth` config map (see
[Enabling IAM principal access to your cluster](https://docs.aws.amazon.com/eks/latest/userguide/add-user-role.html)), as
well as having an assume role policy which allows it to be assumed by the Argo CD pod role.

Example assume role policy for a cluster which is managed by Argo CD:

```json
{
    "Version" : "2012-10-17",
    "Statement" : {
      "Effect" : "Allow",
      "Action" : "sts:AssumeRole",
      "Principal" : {
        "AWS" : "<arn:aws:iam::<AWS_ACCOUNT_ID>:role/<ARGO_CD_MANAGEMENT_IAM_ROLE_NAME>"
      }
    }
  }
```

Example kube-system/aws-auth configmap for your cluster managed by Argo CD:

```yaml
apiVersion: v1
data:
  # Other groups and accounts omitted for brevity. Ensure that no other rolearns and/or groups are inadvertently removed, 
  # or you risk borking access to your cluster.
  #
  # The group name is a RoleBinding which you use to map to a [Cluster]Role. See https://kubernetes.io/docs/reference/access-authn-authz/rbac/#role-binding-examples  
  mapRoles: |
    - "groups":
      - "<GROUP-NAME-IN-K8S-RBAC>"
      "rolearn": "<arn:aws:iam::<AWS_ACCOUNT_ID>:role/<IAM_ROLE_NAME>"
      "username": "<some-username>"
```

#### Alternative EKS Authentication Methods
In some scenarios it may not be possible to use IRSA, such as when the Argo CD cluster is running on a different cloud
provider's platform. In this case, there are two options:
1. Use `execProviderConfig` to call the AWS authentication mechanism which enables the injection of environment variables to supply credentials
2. Leverage the new AWS profile option available in Argo CD release 2.10

Both of these options will require the steps involving IAM and the `aws-auth` config map (defined above) to provide the 
principal with access to the cluster.

##### Using execProviderConfig with Environment Variables
```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: mycluster-secret
  labels:
    argocd.argoproj.io/secret-type: cluster
type: Opaque
stringData:
  name: mycluster
  server: https://mycluster.example.com
  namespaces: "my,managed,namespaces"
  clusterResources: "true"
  config: |
    {
      "execProviderConfig": {
        "command": "argocd-k8s-auth",
        "args": ["aws", "--cluster-name", "my-eks-cluster"],
        "apiVersion": "client.authentication.k8s.io/v1beta1",
        "env": {
          "AWS_REGION": "xx-east-1",
          "AWS_ACCESS_KEY_ID": "{{ .aws_key_id }}",
          "AWS_SECRET_ACCESS_KEY": "{{ .aws_key_secret }}",
          "AWS_SESSION_TOKEN": "{{ .aws_token }}"
        }
      },
      "tlsClientConfig": {
        "insecure": false,
        "caData": "{{ .cluster_cert }}"
      }
    }
```

This example assumes that the role being attached to the credentials that have been supplied, if this is not the case
the role can be appended to the `args` section like so:

```yaml
...
    "args": ["aws", "--cluster-name", "my-eks-cluster", "--role-arn", "arn:aws:iam::<AWS_ACCOUNT_ID>:role/<IAM_ROLE_NAME>"],
...
```
This construct can be used in conjunction with something like the External Secrets Operator to avoid storing the keys in
plain text and additionally helps to provide a foundation for key rotation.

##### Using An AWS Profile For Authentication
The option to use profiles, added in release 2.10, provides a method for supplying credentials while still using the
standard Argo CD EKS cluster declaration with an additional command flag that points to an AWS credentials file:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mycluster-secret
  labels:
    argocd.argoproj.io/secret-type: cluster
type: Opaque
stringData:
  name: "mycluster.com"
  server: "https://mycluster.com"
  config: |
    {
      "awsAuthConfig": {
        "clusterName": "my-eks-cluster-name",
        "roleARN": "arn:aws:iam::<AWS_ACCOUNT_ID>:role/<IAM_ROLE_NAME>",
        "profile": "/mount/path/to/my-profile-file"
      },
      "tlsClientConfig": {
        "insecure": false,
        "caData": "<base64 encoded certificate>"
      }        
    }
```
This will instruct Argo CD to read the file at the provided path and use the credentials defined within to authenticate to AWS. 
The profile must be mounted in both the `argocd-server` and `argocd-application-controller` components in order for this to work.
For example, the following values can be defined in a Helm-based Argo CD deployment:

```yaml
controller:
  extraVolumes:
    - name: my-profile-volume
      secret:
        secretName: my-aws-profile
        items:
          - key: my-profile-file
            path: my-profile-file
  extraVolumeMounts:
    - name: my-profile-mount
      mountPath: /mount/path/to
      readOnly: true

server:
  extraVolumes:
    - name: my-profile-volume
      secret:
        secretName: my-aws-profile
        items:
          - key: my-profile-file
            path: my-profile-file
  extraVolumeMounts:
    - name: my-profile-mount
      mountPath: /mount/path/to
      readOnly: true
```

Where the secret is defined as follows:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-aws-profile
type: Opaque
stringData:
  my-profile-file: |
    [default]
    region = <aws_region>
    aws_access_key_id = <aws_access_key_id>
    aws_secret_access_key = <aws_secret_access_key>
    aws_session_token = <aws_session_token>
```

> ⚠️ Secret mounts are updated on an interval, not real time. If rotation is a requirement ensure the token lifetime outlives the mount update interval and the rotation process doesn't immediately invalidate the existing token


### GKE

GKE cluster secret example using argocd-k8s-auth and [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mycluster-secret
  labels:
    argocd.argoproj.io/secret-type: cluster
type: Opaque
stringData:
  name: mycluster.example.com
  server: https://mycluster.example.com
  config: |
    {
      "execProviderConfig": {
        "command": "argocd-k8s-auth",
        "args": ["gcp"],
        "apiVersion": "client.authentication.k8s.io/v1beta1"
      },
      "tlsClientConfig": {
        "insecure": false,
        "caData": "<base64 encoded certificate>"
      }
    }
```

Note that you must enable Workload Identity on your GKE cluster, create GCP service account with appropriate IAM role and bind it to Kubernetes service account for argocd-application-controller and argocd-server (showing Pod logs on UI). See [Use Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity) and [Authenticating to the Kubernetes API server](https://cloud.google.com/kubernetes-engine/docs/how-to/api-server-authentication).

### AKS

Azure cluster secret example using argocd-k8s-auth and [kubelogin](https://github.com/Azure/kubelogin).  The option *azure* to the argocd-k8s-auth execProviderConfig encapsulates the *get-token* command for kubelogin.  Depending upon which authentication flow is desired (devicecode, spn, ropc, msi, azurecli, workloadidentity), set the environment variable AAD_LOGIN_METHOD with this value.  Set other appropriate environment variables depending upon which authentication flow is desired.

|Variable Name|Description|
|-------------|-----------|
|AAD_LOGIN_METHOD|One of devicecode, spn, ropc, msi, azurecli, or workloadidentity|
|AAD_SERVICE_PRINCIPAL_CLIENT_CERTIFICATE|AAD client cert in pfx.  Used in spn login|
|AAD_SERVICE_PRINCIPAL_CLIENT_ID|AAD client application ID|
|AAD_SERVICE_PRINCIPAL_CLIENT_SECRET|AAD client application secret|
|AAD_USER_PRINCIPAL_NAME|Used in the ropc flow|
|AAD_USER_PRINCIPAL_PASSWORD|Used in the ropc flow|
|AZURE_TENANT_ID|The AAD tenant ID.|
|AZURE_AUTHORITY_HOST|Used in the WorkloadIdentityLogin flow|
|AZURE_FEDERATED_TOKEN_FILE|Used in the WorkloadIdentityLogin flow|
|AZURE_CLIENT_ID|Used in the WorkloadIdentityLogin flow|

In addition to the environment variables above, argocd-k8s-auth accepts two extra environment variables to set the AAD environment, and to set the AAD server application ID.  The AAD server application ID will default to 6dae42f8-4368-4678-94ff-3960e28e3630 if not specified.  See [here](https://github.com/azure/kubelogin#exec-plugin-format) for details.

|Variable Name|Description|
|-------------|-----------|
|AAD_ENVIRONMENT_NAME|The azure environment to use, default of AzurePublicCloud|
|AAD_SERVER_APPLICATION_ID|The optional AAD server application ID, defaults to 6dae42f8-4368-4678-94ff-3960e28e3630|

This is an example of using the [federated workload login flow](https://github.com/Azure/kubelogin#azure-workload-federated-identity-non-interactive).  The federated token file needs to be mounted as a secret into argoCD, so it can be used in the flow.  The location of the token file needs to be set in the environment variable AZURE_FEDERATED_TOKEN_FILE.

If your AKS cluster utilizes the [Mutating Admission Webhook](https://azure.github.io/azure-workload-identity/docs/installation/mutating-admission-webhook.html) from the Azure Workload Identity project, follow these steps to enable the `argocd-application-controller` and `argocd-server` pods to use the federated identity:

1. **Label the Pods**: Add the `azure.workload.identity/use: "true"` label to the `argocd-application-controller` and `argocd-server` pods.

2. **Create Federated Identity Credential**: Generate an Azure federated identity credential for the `argocd-application-controller` and `argocd-server` service accounts. Refer to the [Federated Identity Credential](https://azure.github.io/azure-workload-identity/docs/topics/federated-identity-credential.html) documentation for detailed instructions.

3. **Add Annotations to Service Account** Add `"azure.workload.identity/client-id": "$CLIENT_ID"` and `"azure.workload.identity/tenant-id": "$TENANT_ID"` annotations to the `argocd-application-controller` and `argocd-server` service accounts using the details from the federated credential.

4. **Set the AZURE_CLIENT_ID**: Update the `AZURE_CLIENT_ID` in the cluster secret to match the client id of the newly created federated identity credential.


```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mycluster-secret
  labels:
    argocd.argoproj.io/secret-type: cluster
type: Opaque
stringData:
  name: mycluster.example.com
  server: https://mycluster.example.com
  config: |
    {
      "execProviderConfig": {
        "command": "argocd-k8s-auth",
        "env": {
          "AAD_ENVIRONMENT_NAME": "AzurePublicCloud",
          "AZURE_CLIENT_ID": "fill in client id",
          "AZURE_TENANT_ID": "fill in tenant id", # optional, injected by workload identity mutating admission webhook if enabled
          "AZURE_FEDERATED_TOKEN_FILE": "/opt/path/to/federated_file.json", # optional, injected by workload identity mutating admission webhook if enabled
          "AZURE_AUTHORITY_HOST": "https://login.microsoftonline.com/", # optional, injected by workload identity mutating admission webhook if enabled
          "AAD_LOGIN_METHOD": "workloadidentity"
        },
        "args": ["azure"],
        "apiVersion": "client.authentication.k8s.io/v1beta1"
      },
      "tlsClientConfig": {
        "insecure": false,
        "caData": "<base64 encoded certificate>"
      }
    }
```

This is an example of using the spn (service principal name) flow.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mycluster-secret
  labels:
    argocd.argoproj.io/secret-type: cluster
type: Opaque
stringData:
  name: mycluster.example.com
  server: https://mycluster.example.com
  config: |
    {
      "execProviderConfig": {
        "command": "argocd-k8s-auth",
        "env": {
          "AAD_ENVIRONMENT_NAME": "AzurePublicCloud",
          "AAD_SERVICE_PRINCIPAL_CLIENT_SECRET": "fill in your service principal client secret",
          "AZURE_TENANT_ID": "fill in tenant id",
          "AAD_SERVICE_PRINCIPAL_CLIENT_ID": "fill in your service principal client id",
          "AAD_LOGIN_METHOD": "spn"
        },
        "args": ["azure"],
        "apiVersion": "client.authentication.k8s.io/v1beta1"
      },
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

Resources can be excluded from discovery and sync so that Argo CD is unaware of them. For example, the apiGroup/kind `events.k8s.io/*`, `metrics.k8s.io/*` and `coordination.k8s.io/Lease` are always excluded. Use cases:

* You have temporal issues and you want to exclude problematic resources.
* There are many of a kind of resources that impacts Argo CD's performance.
* Restrict Argo CD's access to certain kinds of resources, e.g. secrets. See [security.md#cluster-rbac](security.md#cluster-rbac).

To configure this, edit the `argocd-cm` config map:

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

## Auto respect RBAC for controller

Argocd controller can be restricted from discovering/syncing specific resources using just controller rbac, without having to manually configure resource exclusions.
This feature can be enabled by setting `resource.respectRBAC` key in argocd cm, once it is set the controller will automatically stop watching for resources 
that it does not have the permission to list/access. Possible values for `resource.respectRBAC` are:
    - `strict` : This setting checks whether the list call made by controller is forbidden/unauthorized and if it is, it will cross-check the permission by making a `SelfSubjectAccessReview` call for the resource.
    - `normal` : This will only check whether the list call response is forbidden/unauthorized and skip `SelfSubjectAccessReview` call, to minimize any extra api-server calls.
    - unset/empty (default) : This will disable the feature and controller will continue to monitor all resources.

Users who are comfortable with an increase in kube api-server calls can opt for `strict` option while users who are concerned with higher api calls and are willing to compromise on the accuracy can opt for the `normal` option.

Notes:

* When set to use `strict` mode controller must have rbac permission to `create` a `SelfSubjectAccessReview` resource 
* The `SelfSubjectAccessReview` request will be only made for the `list` verb, it is assumed that if `list` is allowed for a resource then all other permissions are also available to the controller.

Example argocd cm with `resource.respectRBAC` set to `strict`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
data:
  resource.respectRBAC: "strict"
```

## Resource Custom Labels

Custom Labels configured with `resource.customLabels` (comma separated string) will be displayed in the UI (for any resource that defines them).

## Labels on Application Events

An optional comma-separated list of `metadata.labels` keys can be configured with `resource.includeEventLabelKeys` to add to Kubernetes events generated for Argo CD Applications. When events are generated for Applications containing the specified labels, the controller adds the matching labels to the event. This establishes an easy link between the event and the application, allowing for filtering using labels. In case of conflict between labels on the Application and AppProject, the Application label values are prioritized and added to the event.

```yaml
  resource.includeEventLabelKeys: team,env*
```

To exclude certain labels from events, use the `resource.excludeEventLabelKeys` key, which takes a comma-separated list of `metadata.labels` keys.

```yaml
  resource.excludeEventLabelKeys: environment,bu
```

Both `resource.includeEventLabelKeys` and `resource.excludeEventLabelKeys` support wildcards.

## SSO & RBAC

* SSO configuration details: [SSO](./user-management/index.md)
* RBAC configuration details: [RBAC](./rbac.md)

## Manage Argo CD Using Argo CD

Argo CD is able to manage itself since all settings are represented by Kubernetes manifests. The suggested way is to create [Kustomize](https://github.com/kubernetes-sigs/kustomize)
based application which uses base Argo CD manifests from [https://github.com/argoproj/argo-cd](https://github.com/argoproj/argo-cd/tree/stable/manifests) and apply required changes on top.

Example of `kustomization.yaml`:

```yaml
# additional resources like ingress rules, cluster and repository secrets.
resources:
- github.com/argoproj/argo-cd//manifests/cluster-install?ref=stable
- clusters-secrets.yaml
- repos-secrets.yaml

# changes to config maps
patches:
- path: overlays/argo-cd-cm.yaml
```

The live example of self managed Argo CD config is available at [https://cd.apps.argoproj.io](https://cd.apps.argoproj.io) and with configuration
stored at [argoproj/argoproj-deployments](https://github.com/argoproj/argoproj-deployments/tree/master/argocd).

!!! note
    You will need to sign-in using your GitHub account to get access to [https://cd.apps.argoproj.io](https://cd.apps.argoproj.io)
