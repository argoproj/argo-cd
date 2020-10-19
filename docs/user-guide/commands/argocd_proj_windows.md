## argocd proj windows

Manage a project's sync windows

### Synopsis

Manage a project's sync windows

```
argocd proj windows [flags]
```

### Options

```
  -h, --help   help for windows
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

* [argocd proj](argocd_proj.md)	 - Manage projects
* [argocd proj windows add](argocd_proj_windows_add.md)	 - Add a sync window to a project
* [argocd proj windows delete](argocd_proj_windows_delete.md)	 - Delete a sync window from a project. Requires ID which can be found by running "argocd proj windows list PROJECT"
* [argocd proj windows disable-manual-sync](argocd_proj_windows_disable-manual-sync.md)	 - Disable manual sync for a sync window
* [argocd proj windows enable-manual-sync](argocd_proj_windows_enable-manual-sync.md)	 - Enable manual sync for a sync window
* [argocd proj windows list](argocd_proj_windows_list.md)	 - List project sync windows
* [argocd proj windows update](argocd_proj_windows_update.md)	 - Update a project sync window

