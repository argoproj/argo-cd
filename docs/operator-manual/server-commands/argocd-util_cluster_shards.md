## argocd-util cluster shards

Print information about each controller shard and portion of Kubernetes resources it is responsible for.

```
argocd-util cluster shards [flags]
```

### Options

```
      --app-state-cache-expiration duration   Cache expiration for app state (default 1h0m0s)
      --as string                             Username to impersonate for the operation
      --as-group stringArray                  Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --certificate-authority string          Path to a cert file for the certificate authority
      --client-certificate string             Path to a client certificate file for TLS
      --client-key string                     Path to a client key file for TLS
      --cluster string                        The name of the kubeconfig cluster to use
      --context string                        The name of the kubeconfig context to use
      --default-cache-expiration duration     Cache expiration default (default 24h0m0s)
  -h, --help                                  help for shards
      --insecure-skip-tls-verify              If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string                     Path to a kube config. Only required if out-of-cluster
  -n, --namespace string                      If present, the namespace scope for this CLI request
      --password string                       Password for basic authentication to the API server
      --port-forward-redis                    Automatically port-forward ha proxy redis from current namespace? (default true)
      --redis string                          Redis server hostname and port (e.g. argocd-redis:6379). 
      --redisdb int                           Redis database.
      --replicas int                          Application controller replicas count. Inferred from number of running controller pods if not specified
      --request-timeout string                The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --sentinel stringArray                  Redis sentinel hostname and port (e.g. argocd-redis-ha-announce-0:6379). 
      --sentinelmaster string                 Redis sentinel master group name. (default "master")
      --server string                         The address and port of the Kubernetes API server
      --shard int                             Cluster shard filter (default -1)
      --tls-server-name string                If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                          Bearer token for authentication to the API server
      --user string                           The name of the kubeconfig user to use
      --username string                       Username for basic authentication to the API server
```

### SEE ALSO

* [argocd-util cluster](argocd-util_cluster.md)	 - Manage clusters configuration

