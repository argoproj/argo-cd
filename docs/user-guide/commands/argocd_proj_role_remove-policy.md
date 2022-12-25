## argocd proj role remove-policy

Remove a policy from a role within a project

```
argocd proj role remove-policy PROJECT ROLE-NAME [flags]
```

### Options

```
  -a, --action string                  Action to grant/deny permission on (e.g. get, create, list, update, delete)
  -h, --help                           help for remove-policy
  -o, --object string                  Object within the project to grant/deny access.  Use '*' for a wildcard. Will want access to '<project>/<object>'
  -p, --permission string              Whether to allow or deny access to object with the action.  This can only be 'allow' or 'deny' (default "allow")
      --redis-ha-haproxy-name string   Redis HA HAProxy name (default "argocd-redis-ha-haproxy")
      --redis-name string              Redis name (default "argocd-redis")
      --repo-server-name string        Repo server name (default "argocd-repo-server")
      --server-name string             Server name (default "argocd-server")
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
      --kube-context string             Directs the command to the given kube-context
      --logformat string                Set the logging format. One of: text|json (default "text")
      --loglevel string                 Set the logging level. One of: debug|info|warn|error (default "info")
      --plaintext                       Disable TLS
      --port-forward                    Connect to a random argocd-server port using port forwarding
      --port-forward-namespace string   Namespace name which should be used for port forwarding
      --server string                   Argo CD server address
      --server-crt string               Server certificate file
```

### SEE ALSO

* [argocd proj role](argocd_proj_role.md)	 - Manage a project's roles

