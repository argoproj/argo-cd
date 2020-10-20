## argocd account

Manage account settings

### Synopsis

Manage account settings

```
argocd account [flags]
```

### Options

```
  -h, --help   help for account
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
* [argocd account can-i](argocd_account_can-i.md)	 - Can I
* [argocd account delete-token](argocd_account_delete-token.md)	 - Deletes account token
* [argocd account generate-token](argocd_account_generate-token.md)	 - Generate account token
* [argocd account get](argocd_account_get.md)	 - Get account details
* [argocd account get-user-info](argocd_account_get-user-info.md)	 - Get user info
* [argocd account list](argocd_account_list.md)	 - List accounts
* [argocd account update-password](argocd_account_update-password.md)	 - Update password

