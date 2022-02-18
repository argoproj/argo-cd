## argocd repocreds add

Add git repository connection parameters

```
argocd repocreds add REPOURL [flags]
```

### Examples

```
  # Add credentials with user/pass authentication to use for all repositories under https://git.example.com/repos
  argocd repocreds add https://git.example.com/repos/ --username git --password secret

  # Add credentials with SSH private key authentication to use for all repositories under ssh://git@git.example.com/repos
  argocd repocreds add ssh://git@git.example.com/repos/ --ssh-private-key-path ~/.ssh/id_rsa

  # Add credentials with GitHub App authentication to use for all repositories under https://github.com/repos
  argocd repocreds add https://github.com/repos/ --github-app-id 1 --github-app-installation-id 2 --github-app-private-key-path test.private-key.pem

  # Add credentials with GitHub App authentication to use for all repositories under https://ghe.example.com/repos
  argocd repocreds add https://ghe.example.com/repos/ --github-app-id 1 --github-app-installation-id 2 --github-app-private-key-path test.private-key.pem --github-app-enterprise-base-url https://ghe.example.com/api/v3

  # Add credentials with helm oci registry so that these oci registry urls do not need to be added as repos individually.
  argocd repocreds add localhost:5000/myrepo --enable-oci --type helm 

```

### Options

```
      --enable-oci                              Specifies whether helm-oci support should be enabled for this repo
      --github-app-enterprise-base-url string   base url to use when using GitHub Enterprise (e.g. https://ghe.example.com/api/v3
      --github-app-id int                       id of the GitHub Application
      --github-app-installation-id int          installation id of the GitHub Application
      --github-app-private-key-path string      private key of the GitHub Application
  -h, --help                                    help for add
      --password string                         password to the repository
      --ssh-private-key-path string             path to the private ssh key (e.g. ~/.ssh/id_rsa)
      --tls-client-cert-key-path string         path to the TLS client cert's key path (must be PEM format)
      --tls-client-cert-path string             path to the TLS client cert (must be PEM format)
      --type string                             type of the repository, "git" or "helm" (default "git")
      --upsert                                  Override an existing repository with the same name even if the spec differs
      --username string                         username to the repository
```

### Options inherited from parent commands

```
      --auth-token string               Authentication token
      --client-crt string               Client certificate file
      --client-crt-key string           Client certificate key file
      --config string                   Path to Argo CD config (default "/home/user/.config/argocd/config")
      --core                            If set to true then CLI talks directly to Kubernetes instead of talking to Argo CD API server
      --grpc-web                        Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2.
      --grpc-web-root-path string       Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2. Set web root.
  -H, --header strings                  Sets additional header to all requests made by Argo CD CLI. (Can be repeated multiple times to add multiple headers, also supports comma separated headers)
      --http-retry-max int              Maximum number of retries to establish http connection to Argo CD server
      --insecure                        Skip server certificate and domain verification
      --logformat string                Set the logging format. One of: text|json (default "text")
      --loglevel string                 Set the logging level. One of: debug|info|warn|error (default "info")
      --plaintext                       Disable TLS
      --port-forward                    Connect to a random argocd-server port using port forwarding
      --port-forward-namespace string   Namespace name which should be used for port forwarding
      --server string                   Argo CD server address
      --server-crt string               Server certificate file
```

### SEE ALSO

* [argocd repocreds](argocd_repocreds.md)	 - Manage repository connection parameters

