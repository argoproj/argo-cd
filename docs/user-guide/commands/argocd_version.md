## argocd version

Print version information

```
argocd version [flags]
```

### Examples

```
  # Print the full version of client and server to stdout
  argocd version

  # Print only full version of the client - no connection to server will be made
  argocd version --client

  # Print the full version of client and server in JSON format
  argocd version -o json

  # Print only client and server core version strings in YAML format
  argocd version --short -o yaml

```

### Options

```
      --client          client version only (no server required)
  -h, --help            help for version
  -o, --output string   Output format. One of: json|yaml|wide|short (default "wide")
      --short           print just the version number
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

* [argocd](argocd.md)	 - argocd controls a Argo CD server

