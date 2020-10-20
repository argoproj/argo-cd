## argocd proj allow-namespace-resource

Removes a namespaced API resource from the blacklist or add a namespaced API resource to the whitelist

### Synopsis

Removes a namespaced API resource from the blacklist or add a namespaced API resource to the whitelist

```
argocd proj allow-namespace-resource PROJECT GROUP KIND [flags]
```

### Options

```
  -h, --help          help for allow-namespace-resource
  -l, --list string   Use blacklist or whitelist. This can only be 'white' or 'black' (default "black")
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

