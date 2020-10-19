## argocd cert

Manage repository certificates and SSH known hosts entries

### Synopsis

Manage repository certificates and SSH known hosts entries

```
argocd cert [flags]
```

### Examples

```
  # Add a TLS certificate for cd.example.com to ArgoCD cert store from a file
  argocd cert add-tls --from ~/mycert.pem cd.example.com

  # Add a TLS certificate for cd.example.com to ArgoCD via stdin
  cat ~/mycert.pem | argocd cert add-tls cd.example.com

  # Add SSH known host entries for cd.example.com to ArgoCD by scanning host
  ssh-keyscan cd.example.com | argocd cert add-ssh --batch

  # List all known TLS certificates
  argocd cert list --cert-type https

  # Remove all TLS certificates for cd.example.com
  argocd cert rm --cert-type https cd.example.com

  # Remove all certificates and SSH known host entries for cd.example.com
  argocd cert rm cd.example.com

```

### Options

```
  -h, --help   help for cert
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

* [argocd](argocd.md)	 - argocd controls a Argo CD server
* [argocd cert add-ssh](argocd_cert_add-ssh.md)	 - Add SSH known host entries for repository servers
* [argocd cert add-tls](argocd_cert_add-tls.md)	 - Add TLS certificate data for connecting to repository server SERVERNAME
* [argocd cert list](argocd_cert_list.md)	 - List configured certificates
* [argocd cert rm](argocd_cert_rm.md)	 - Remove certificate of TYPE for REPOSERVER

