## argocd account update-password

Update an account's password

### Synopsis


This command can be used to update the password of the currently logged on
user, or an arbitrary local user account when the currently logged on user
has appropriate RBAC permissions to change other accounts.


```
argocd account update-password [flags]
```

### Examples

```

	# Update the current user's password
	argocd account update-password

	# Update the password for user foobar
	argocd account update-password --account foobar

```

### Options

```
      --account string            an account name that should be updated. Defaults to current user account
      --current-password string   password of the currently logged on user
  -h, --help                      help for update-password
      --new-password string       new password you want to update to
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

* [argocd account](argocd_account.md)	 - Manage account settings

