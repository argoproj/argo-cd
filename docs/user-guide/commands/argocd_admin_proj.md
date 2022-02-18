## argocd admin proj

Manage projects configuration

```
argocd admin proj [flags]
```

### Options

```
  -h, --help   help for proj
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

* [argocd admin](argocd_admin.md)	 - Contains a set of commands useful for Argo CD administrators and requires direct Kubernetes access
* [argocd admin proj generate-allow-list](argocd_admin_proj_generate-allow-list.md)	 - Generates project allow list from the specified clusterRole file
* [argocd admin proj generate-spec](argocd_admin_proj_generate-spec.md)	 - Generate declarative config for a project
* [argocd admin proj update-role-policy](argocd_admin_proj_update-role-policy.md)	 - Implement bulk project role update. Useful to back-fill existing project policies or remove obsolete actions.

