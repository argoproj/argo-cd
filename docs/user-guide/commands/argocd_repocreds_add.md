## argocd repocreds add

Add git repository connection parameters

### Synopsis

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

```

### Options

```
  -h, --help                              help for add
      --password string                   password to the repository
      --ssh-private-key-path string       path to the private ssh key (e.g. ~/.ssh/id_rsa)
      --tls-client-cert-key-path string   path to the TLS client cert's key path (must be PEM format)
      --tls-client-cert-path string       path to the TLS client cert (must be PEM format)
      --upsert                            Override an existing repository with the same name even if the spec differs
      --username string                   username to the repository
```

### Options inherited from parent commands

```
      --auth-token string               Authentication token
      --client-crt string               Client certificate file
      --client-crt-key string           Client certificate key file
      --config string                   Path to Argo CD config (default "/home/user/.argocd/config")
      --grpc-web                        Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2.
      --grpc-web-root-path string       Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2. Set web root.
  -H, --header strings                  Sets additional header to all requests made by Argo CD CLI. (Can be repeated multiple times to add multiple headers, also supports comma separated headers)
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

