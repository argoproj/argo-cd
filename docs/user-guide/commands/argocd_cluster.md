## argocd cluster

Manage cluster credentials

### Synopsis

Manage cluster credentials

```
argocd cluster [flags]
```

### Examples

```
  # List all known clusters in JSON format:
  argocd cluster list -o json

  # Add a target cluster configuration to ArgoCD. The context must exist in your kubectl config:
  argocd cluster add example-cluster

  # Get specific details about a cluster in plain text (wide) format:
  argocd cluster get example-cluster -o wide

  #	Remove a target cluster context from ArgoCD
  argocd cluster rm example-cluster

```

### Options

```
  -h, --help   help for cluster
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
* [argocd cluster add](argocd_cluster_add.md)	 - argocd cluster add CONTEXT
* [argocd cluster get](argocd_cluster_get.md)	 - Get cluster information
* [argocd cluster list](argocd_cluster_list.md)	 - List configured clusters
* [argocd cluster rm](argocd_cluster_rm.md)	 - Remove cluster credentials
* [argocd cluster rotate-auth](argocd_cluster_rotate-auth.md)	 - argocd cluster rotate-auth SERVER

