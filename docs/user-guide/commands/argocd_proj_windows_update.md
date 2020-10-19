## argocd proj windows update

Update a project sync window

### Synopsis

Update a project sync window. Requires ID which can be found by running "argocd proj windows list PROJECT"

```
argocd proj windows update PROJECT ID [flags]
```

### Options

```
      --applications strings   Applications that the schedule will be applied to. Comma separated, wildcards supported (e.g. --applications prod-\*,website)
      --clusters strings       Clusters that the schedule will be applied to. Comma separated, wildcards supported (e.g. --clusters prod,staging)
      --duration string        Sync window duration. (e.g. --duration 1h)
  -h, --help                   help for update
      --namespaces strings     Namespaces that the schedule will be applied to. Comma separated, wildcards supported (e.g. --namespaces default,\*-prod)
      --schedule string        Sync window schedule in cron format. (e.g. --schedule "0 22 * * *")
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

* [argocd proj windows](argocd_proj_windows.md)	 - Manage a project's sync windows

