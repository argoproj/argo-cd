## argocd gpg

Manage GPG keys used for signature verification

### Synopsis

Manage GPG keys used for signature verification

```
argocd gpg [flags]
```

### Options

```
  -h, --help   help for gpg
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
* [argocd gpg add](argocd_gpg_add.md)	 - Adds a GPG public key to the server's keyring
* [argocd gpg get](argocd_gpg_get.md)	 - Get the GPG public key with ID <KEYID> from the server
* [argocd gpg list](argocd_gpg_list.md)	 - List configured GPG public keys
* [argocd gpg rm](argocd_gpg_rm.md)	 - Removes a GPG public key from the server's keyring

