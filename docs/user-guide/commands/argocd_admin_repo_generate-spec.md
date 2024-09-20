# `argocd admin repo generate-spec` Command Reference

## argocd admin repo generate-spec

Generate declarative config for a repo

```
argocd admin repo generate-spec REPOURL [flags]
```

### Examples

```
  
  # Add a Git repository via SSH using a private key for authentication, ignoring the server's host key:
  argocd admin repo generate-spec git@git.example.com:repos/repo --insecure-ignore-host-key --ssh-private-key-path ~/id_rsa

  # Add a Git repository via SSH on a non-default port - need to use ssh:// style URLs here
  argocd admin repo generate-spec ssh://git@git.example.com:2222/repos/repo --ssh-private-key-path ~/id_rsa

  # Add a private Git repository via HTTPS using username/password and TLS client certificates:
  argocd admin repo generate-spec https://git.example.com/repos/repo --username git --password secret --tls-client-cert-path ~/mycert.crt --tls-client-cert-key-path ~/mycert.key

  # Add a private Git repository via HTTPS using username/password without verifying the server's TLS certificate
  argocd admin repo generate-spec https://git.example.com/repos/repo --username git --password secret --insecure-skip-server-verification

  # Add a public Helm repository named 'stable' via HTTPS
  argocd admin repo generate-spec https://charts.helm.sh/stable --type helm --name stable  

  # Add a private Helm repository named 'stable' via HTTPS
  argocd admin repo generate-spec https://charts.helm.sh/stable --type helm --name stable --username test --password test

  # Add a private Helm OCI-based repository named 'stable' via HTTPS
  argocd admin repo generate-spec helm-oci-registry.cn-zhangjiakou.cr.aliyuncs.com --type helm --name stable --enable-oci --username test --password test

```

### Options

```
      --enable-lfs                              enable git-lfs (Large File Support) on this repository
      --enable-oci                              enable helm-oci (Helm OCI-Based Repository)
      --force-http-basic-auth                   whether to force use of basic auth when connecting repository via HTTP
      --gcp-service-account-key-path string     service account key for the Google Cloud Platform
      --github-app-enterprise-base-url string   base url to use when using GitHub Enterprise (e.g. https://ghe.example.com/api/v3
      --github-app-id int                       id of the GitHub Application
      --github-app-installation-id int          installation id of the GitHub Application
      --github-app-private-key-path string      private key of the GitHub Application
  -h, --help                                    help for generate-spec
      --insecure-ignore-host-key                disables SSH strict host key checking (deprecated, use --insecure-skip-server-verification instead)
      --insecure-skip-server-verification       disables server certificate and host key checks
      --name string                             name of the repository, mandatory for repositories of type helm
      --no-proxy string                         don't access these targets via proxy
  -o, --output string                           Output format. One of: json|yaml (default "yaml")
      --password string                         password to the repository
      --project string                          project of the repository
      --proxy string                            use proxy to access repository
      --ssh-private-key-path string             path to the private ssh key (e.g. ~/.ssh/id_rsa)
      --tls-client-cert-key-path string         path to the TLS client cert's key path (must be PEM format)
      --tls-client-cert-path string             path to the TLS client cert (must be PEM format)
      --type string                             type of the repository, "git" or "helm" (default "git")
      --username string                         username to the repository
```

### Options inherited from parent commands

```
      --argocd-context string           The name of the Argo-CD server context to use
      --auth-token string               Authentication token; set this or the ARGOCD_AUTH_TOKEN environment variable
      --client-crt string               Client certificate file
      --client-crt-key string           Client certificate key file
      --config string                   Path to Argo CD config (default "/home/user/.config/argocd/config")
      --controller-name string          Name of the Argo CD Application controller; set this or the ARGOCD_APPLICATION_CONTROLLER_NAME environment variable when the controller's name label differs from the default, for example when installing via the Helm chart (default "argocd-application-controller")
      --core                            If set to true then CLI talks directly to Kubernetes instead of talking to Argo CD API server
      --grpc-web                        Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2.
      --grpc-web-root-path string       Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2. Set web root.
  -H, --header strings                  Sets additional header to all requests made by Argo CD CLI. (Can be repeated multiple times to add multiple headers, also supports comma separated headers)
      --http-retry-max int              Maximum number of retries to establish http connection to Argo CD server
      --insecure                        Skip server certificate and domain verification
      --kube-context string             Directs the command to the given kube-context
      --logformat string                Set the logging format. One of: text|json (default "text")
      --loglevel string                 Set the logging level. One of: debug|info|warn|error (default "info")
      --plaintext                       Disable TLS
      --port-forward                    Connect to a random argocd-server port using port forwarding
      --port-forward-namespace string   Namespace name which should be used for port forwarding
      --redis-haproxy-name string       Name of the Redis HA Proxy; set this or the ARGOCD_REDIS_HAPROXY_NAME environment variable when the HA Proxy's name label differs from the default, for example when installing via the Helm chart (default "argocd-redis-ha-haproxy")
      --redis-name string               Name of the Redis deployment; set this or the ARGOCD_REDIS_NAME environment variable when the Redis's name label differs from the default, for example when installing via the Helm chart (default "argocd-redis")
      --repo-server-name string         Name of the Argo CD Repo server; set this or the ARGOCD_REPO_SERVER_NAME environment variable when the server's name label differs from the default, for example when installing via the Helm chart (default "argocd-repo-server")
      --server string                   Argo CD server address
      --server-crt string               Server certificate file
      --server-name string              Name of the Argo CD API server; set this or the ARGOCD_SERVER_NAME environment variable when the server's name label differs from the default, for example when installing via the Helm chart (default "argocd-server")
```

### SEE ALSO

* [argocd admin repo](argocd_admin_repo.md)	 - Manage repositories configuration

