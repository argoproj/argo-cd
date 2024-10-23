# `argocd admin cluster` Command Reference

## argocd admin cluster

Manage clusters configuration

```
argocd admin cluster [flags]
```

### Examples

```

#Generate declarative config for a cluster
argocd admin cluster generate-spec my-cluster -o yaml

#Generate a kubeconfig for a cluster named "my-cluster" and display it in the console
argocd admin cluster kubeconfig my-cluster

#Print information namespaces which Argo CD manages in each cluster
argocd admin cluster namespaces my-cluster 
```

### Options

```
  -h, --help   help for cluster
```

### Options inherited from parent commands

```
      --argocd-context string           The name of the Argo-CD server context to use
      --auth-token string               Authentication token; set this or the ARGOCD_AUTH_TOKEN environment variable
      --client-crt string               Client certificate file
      --client-crt-key string           Client certificate key file
      --config string                   Path to Argo CD config (default "/home/user/.config/argocd/config")
      --controller-name string          Name of the Argo CD Application controller; set this or the ARGOCD_APPLICATION_CONTROLLER_NAME environment variable when the controller's name label differs from the default, for example when installing via the Helm chart (default "argocd-application-controller")
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
      --redis-haproxy-name string       Name of the Redis HA Proxy; set this or the ARGOCD_REDIS_HAPROXY_NAME environment variable when the HA Proxy's name label differs from the default, for example when installing via the Helm chart (default "argocd-redis-ha-haproxy")
      --redis-name string               Name of the Redis deployment; set this or the ARGOCD_REDIS_NAME environment variable when the Redis's name label differs from the default, for example when installing via the Helm chart (default "argocd-redis")
      --repo-server-name string         Name of the Argo CD Repo server; set this or the ARGOCD_REPO_SERVER_NAME environment variable when the server's name label differs from the default, for example when installing via the Helm chart (default "argocd-repo-server")
      --server string                   Argo CD server address
      --server-crt string               Server certificate file
      --server-name string              Name of the Argo CD API server; set this or the ARGOCD_SERVER_NAME environment variable when the server's name label differs from the default, for example when installing via the Helm chart (default "argocd-server")
```

### SEE ALSO

* [argocd admin](argocd_admin.md)	 - Contains a set of commands useful for Argo CD administrators and requires direct Kubernetes access
* [argocd admin cluster generate-spec](argocd_admin_cluster_generate-spec.md)	 - Generate declarative config for a cluster
* [argocd admin cluster kubeconfig](argocd_admin_cluster_kubeconfig.md)	 - Generates kubeconfig for the specified cluster
* [argocd admin cluster namespaces](argocd_admin_cluster_namespaces.md)	 - Print information namespaces which Argo CD manages in each cluster.
* [argocd admin cluster shards](argocd_admin_cluster_shards.md)	 - Print information about each controller shard and the estimated portion of Kubernetes resources it is responsible for.
* [argocd admin cluster stats](argocd_admin_cluster_stats.md)	 - Prints information cluster statistics and inferred shard number

