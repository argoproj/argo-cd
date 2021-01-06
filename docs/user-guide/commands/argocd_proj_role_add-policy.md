## argocd proj role add-policy

Add a policy to a project role

```
argocd proj role add-policy PROJECT ROLE-NAME [flags]
```

### Options

```
  -a, --action string       Action to grant/deny permission on (e.g. get, create, list, update, delete)
  -h, --help                help for add-policy
  -o, --object string       Object within the project to grant/deny access.  Use '*' for a wildcard. Will want access to '<project>/<object>'
  -p, --permission string   Whether to allow or deny access to object with the action.  This can only be 'allow' or 'deny' (default "allow")
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

* [argocd proj role](argocd_proj_role.md)	 - Manage a project's roles

