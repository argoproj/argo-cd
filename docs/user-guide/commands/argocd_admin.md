## argocd admin

Contains a set of commands useful for Argo CD administrators and requires direct Kubernetes access

```
argocd admin [flags]
```

### Options

```
  -h, --help   help for admin
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

* [argocd](argocd.md)	 - argocd controls a Argo CD server
* [argocd admin app](argocd_admin_app.md)	 - Manage applications configuration
* [argocd admin cluster](argocd_admin_cluster.md)	 - Manage clusters configuration
* [argocd admin dashboard](argocd_admin_dashboard.md)	 - Starts Argo CD Web UI locally
* [argocd admin export](argocd_admin_export.md)	 - Export all Argo CD data to stdout (default) or a file
* [argocd admin import](argocd_admin_import.md)	 - Import Argo CD data from stdin (specify `-') or a file
* [argocd admin notifications](argocd_admin_notifications.md)	 - Set of CLI commands that helps manage notifications settings
* [argocd admin proj](argocd_admin_proj.md)	 - Manage projects configuration
* [argocd admin repo](argocd_admin_repo.md)	 - Manage repositories configuration
* [argocd admin settings](argocd_admin_settings.md)	 - Provides set of commands for settings validation and troubleshooting

