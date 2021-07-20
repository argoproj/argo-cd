## argocd repo add

Add git repository connection parameters

```
argocd repo add REPOURL [flags]
```

### Examples

```
  # Add a Git repository via SSH using a private key for authentication, ignoring the server's host key:
  argocd repo add git@git.example.com:repos/repo --insecure-ignore-host-key --ssh-private-key-path ~/id_rsa

  # Add a Git repository via SSH on a non-default port - need to use ssh:// style URLs here
  argocd repo add ssh://git@git.example.com:2222/repos/repo --ssh-private-key-path ~/id_rsa

  # Add a private Git repository via HTTPS using username/password and TLS client certificates:
  argocd repo add https://git.example.com/repos/repo --username git --password secret --tls-client-cert-path ~/mycert.crt --tls-client-cert-key-path ~/mycert.key

  # Add a private Git repository via HTTPS using username/password without verifying the server's TLS certificate
  argocd repo add https://git.example.com/repos/repo --username git --password secret --insecure-skip-server-verification

  # Add a public Helm repository named 'stable' via HTTPS
  argocd repo add https://charts.helm.sh/stable --type helm --name stable  

  # Add a private Helm repository named 'stable' via HTTPS
  argocd repo add https://charts.helm.sh/stable --type helm --name stable --username test --password test

  # Add a private Helm OCI-based repository named 'stable' via HTTPS
  argocd repo add helm-oci-registry.cn-zhangjiakou.cr.aliyuncs.com --type helm --name stable --enable-oci --username test --password test

  # Add a private Git repository on GitHub.com via GitHub App
  argocd repo add https://git.example.com/repos/repo --github-app-id 1 --github-app-installation-id 2 --github-app-private-key-path test.private-key.pem

  # Add a private Git repository on GitHub Enterprise via GitHub App
  argocd repo add https://ghe.example.com/repos/repo --github-app-id 1 --github-app-installation-id 2 --github-app-private-key-path test.private-key.pem --github-app-enterprise-base-url https://ghe.example.com/api/v3

```

### Options

```
      --enable-lfs                              enable git-lfs (Large File Support) on this repository
      --enable-oci                              enable helm-oci (Helm OCI-Based Repository)
      --github-app-enterprise-base-url string   base url to use when using GitHub Enterprise (e.g. https://ghe.example.com/api/v3
      --github-app-id int                       id of the GitHub Application
      --github-app-installation-id int          installation id of the GitHub Application
      --github-app-private-key-path string      private key of the GitHub Application
  -h, --help                                    help for add
      --insecure-ignore-host-key                disables SSH strict host key checking (deprecated, use --insecure-skip-server-verification instead)
      --insecure-skip-server-verification       disables server certificate and host key checks
      --name string                             name of the repository, mandatory for repositories of type helm
      --proxy string                            use proxy to access repository
      --ssh-private-key-path string             path to the private ssh key (e.g. ~/.ssh/id_rsa)
      --tls-client-cert-key-path string         path to the TLS client cert's key path (must be PEM format)
      --tls-client-cert-path string             path to the TLS client cert (must be PEM format)
      --type string                             type of the repository, "git" or "helm" (default "git")
      --upsert                                  Override an existing repository with the same name even if the spec differs
```

### Options inherited from parent commands

```
      --as string                       Username to impersonate for the operation
      --as-group stringArray            Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --auth-token string               Authentication token
      --certificate-authority string    Path to a cert file for the certificate authority
      --client-certificate string       Path to a client certificate file for TLS
      --client-crt string               Client certificate file
      --client-crt-key string           Client certificate key file
      --client-key string               Path to a client key file for TLS
      --cluster string                  The name of the kubeconfig cluster to use
      --config string                   Path to Argo CD config (default "/home/user/.argocd/config")
      --context string                  The name of the kubeconfig context to use
      --grpc-web                        Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2.
      --grpc-web-root-path string       Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2. Set web root.
  -H, --header strings                  Sets additional header to all requests made by Argo CD CLI. (Can be repeated multiple times to add multiple headers, also supports comma separated headers)
      --headless                        If set to true then CLI talks directly to Kubernetes instead of talking to Argo CD API server
      --http-retry-max int              Maximum number of retries to establish http connection to Argo CD server
      --insecure                        Skip server certificate and domain verification
      --insecure-skip-tls-verify        If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --logformat string                Set the logging format. One of: text|json (default "text")
      --loglevel string                 Set the logging level. One of: debug|info|warn|error (default "info")
  -n, --namespace string                If present, the namespace scope for this CLI request
      --password string                 Password for basic authentication to the API server
      --plaintext                       Disable TLS
      --port-forward                    Connect to a random argocd-server port using port forwarding
      --port-forward-namespace string   Namespace name which should be used for port forwarding
      --request-timeout string          The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --server string                   The address and port of the Kubernetes API server
      --server-crt string               Server certificate file
      --tls-server-name string          If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                    Bearer token for authentication to the API server
      --user string                     The name of the kubeconfig user to use
      --username string                 Username for basic authentication to the API server
```

### SEE ALSO

* [argocd repo](argocd_repo.md)	 - Manage repository connection parameters

