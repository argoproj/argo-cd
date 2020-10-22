## argocd proj set

Set project parameters

### Synopsis

Set project parameters

```
argocd proj set PROJECT [flags]
```

### Options

```
      --description string        Project description
  -d, --dest stringArray          Permitted destination server and namespace (e.g. https://192.168.99.100:8443,default)
  -h, --help                      help for set
      --orphaned-resources        Enables orphaned resources monitoring
      --orphaned-resources-warn   Specifies if applications should be a warning condition when orphaned resources detected
      --signature-keys strings    GnuPG public key IDs for commit signature verification
  -s, --src stringArray           Permitted source repository URL
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

