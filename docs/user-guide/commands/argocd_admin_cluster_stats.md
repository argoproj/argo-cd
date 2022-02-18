## argocd admin cluster stats

Prints information cluster statistics and inferred shard number

```
argocd admin cluster stats [flags]
```

### Options

```
      --app-state-cache-expiration duration   Cache expiration for app state (default 1h0m0s)
      --as string                             Username to impersonate for the operation
      --as-group stringArray                  Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                         UID to impersonate for the operation
      --certificate-authority string          Path to a cert file for the certificate authority
      --client-certificate string             Path to a client certificate file for TLS
      --client-key string                     Path to a client key file for TLS
      --cluster string                        The name of the kubeconfig cluster to use
      --context string                        The name of the kubeconfig context to use
      --default-cache-expiration duration     Cache expiration default (default 24h0m0s)
  -h, --help                                  help for stats
      --insecure-skip-tls-verify              If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string                     Path to a kube config. Only required if out-of-cluster
  -n, --namespace string                      If present, the namespace scope for this CLI request
      --password string                       Password for basic authentication to the API server
      --port-forward-redis                    Automatically port-forward ha proxy redis from current namespace? (default true)
      --redis string                          Redis server hostname and port (e.g. argocd-redis:6379). 
      --redis-ca-certificate string           Path to Redis server CA certificate (e.g. /etc/certs/redis/ca.crt). If not specified, system trusted CAs will be used for server certificate validation.
      --redis-client-certificate string       Path to Redis client certificate (e.g. /etc/certs/redis/client.crt).
      --redis-client-key string               Path to Redis client key (e.g. /etc/certs/redis/client.crt).
      --redis-insecure-skip-tls-verify        Skip Redis server certificate validation.
      --redis-use-tls                         Use TLS when connecting to Redis. 
      --redisdb int                           Redis database.
      --replicas int                          Application controller replicas count. Inferred from number of running controller pods if not specified
      --request-timeout string                The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --sentinel stringArray                  Redis sentinel hostname and port (e.g. argocd-redis-ha-announce-0:6379). 
      --sentinelmaster string                 Redis sentinel master group name. (default "master")
      --shard int                             Cluster shard filter (default -1)
      --tls-server-name string                If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                          Bearer token for authentication to the API server
      --user string                           The name of the kubeconfig user to use
      --username string                       Username for basic authentication to the API server
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

* [argocd admin cluster](argocd_admin_cluster.md)	 - Manage clusters configuration

