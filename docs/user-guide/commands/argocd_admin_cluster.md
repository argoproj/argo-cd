## argocd admin cluster

Manage clusters configuration

```
argocd admin cluster [flags]
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
* [argocd admin cluster generate-spec](argocd_admin_cluster_generate-spec.md)	 - Generate declarative config for a cluster
* [argocd admin cluster kubeconfig](argocd_admin_cluster_kubeconfig.md)	 - Generates kubeconfig for the specified cluster
* [argocd admin cluster namespaces](argocd_admin_cluster_namespaces.md)	 - Print information namespaces which Argo CD manages in each cluster.
* [argocd admin cluster shards](argocd_admin_cluster_shards.md)	 - Print information about each controller shard and portion of Kubernetes resources it is responsible for.
* [argocd admin cluster stats](argocd_admin_cluster_stats.md)	 - Prints information cluster statistics and inferred shard number

