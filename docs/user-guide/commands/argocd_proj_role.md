## argocd proj role

Manage a project's roles

```
argocd proj role [flags]
```

### Options

```
  -h, --help   help for role
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

* [argocd proj](argocd_proj.md)	 - Manage projects
* [argocd proj role add-group](argocd_proj_role_add-group.md)	 - Add a group claim to a project role
* [argocd proj role add-policy](argocd_proj_role_add-policy.md)	 - Add a policy to a project role
* [argocd proj role create](argocd_proj_role_create.md)	 - Create a project role
* [argocd proj role create-token](argocd_proj_role_create-token.md)	 - Create a project token
* [argocd proj role delete](argocd_proj_role_delete.md)	 - Delete a project role
* [argocd proj role delete-token](argocd_proj_role_delete-token.md)	 - Delete a project token
* [argocd proj role get](argocd_proj_role_get.md)	 - Get the details of a specific role
* [argocd proj role list](argocd_proj_role_list.md)	 - List all the roles in a project
* [argocd proj role list-tokens](argocd_proj_role_list-tokens.md)	 - List tokens for a given role.
* [argocd proj role remove-group](argocd_proj_role_remove-group.md)	 - Remove a group claim from a role within a project
* [argocd proj role remove-policy](argocd_proj_role_remove-policy.md)	 - Remove a policy from a role within a project

