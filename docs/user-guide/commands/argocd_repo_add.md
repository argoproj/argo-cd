## argocd repo add

Add git repository connection parameters

### Synopsis

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
  argocd repo add https://kubernetes-charts.storage.googleapis.com --type helm --name stable  

  # Add a private Helm repository named 'stable' via HTTPS
  argocd repo add https://kubernetes-charts.storage.googleapis.com --type helm --name stable --username test --password test

  # Add a private Helm OCI-based repository named 'stable' via HTTPS
  argocd repo add helm-oci-registry.cn-zhangjiakou.cr.aliyuncs.com --type helm --name stable --enable-oci --username test --password test

```

### Options

```
      --enable-lfs                          enable git-lfs (Large File Support) on this repository
      --enable-oci                          enable helm-oci (Helm OCI-Based Repository)
  -h, --help                                help for add
      --insecure-ignore-host-key            disables SSH strict host key checking (deprecated, use --insecure-skip-server-verification instead)
      --insecure-skip-server-verification   disables server certificate and host key checks
      --name string                         name of the repository, mandatory for repositories of type helm
      --password string                     password to the repository
      --ssh-private-key-path string         path to the private ssh key (e.g. ~/.ssh/id_rsa)
      --tls-client-cert-key-path string     path to the TLS client cert's key path (must be PEM format)
      --tls-client-cert-path string         path to the TLS client cert (must be PEM format)
      --type string                         type of the repository, "git" or "helm" (default "git")
      --upsert                              Override an existing repository with the same name even if the spec differs
      --username string                     username to the repository
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

* [argocd repo](argocd_repo.md)	 - Manage repository connection parameters

