# `argocd admin cluster stats` Command Reference

## argocd admin cluster stats

Prints information cluster statistics and inferred shard number

```
argocd admin cluster stats [flags]
```

### Examples

```

#Display stats and shards for clusters 
argocd admin cluster stats

#Display Cluster Statistics for a Specific Shard
argocd admin cluster stats --shard=1

#In a multi-cluster environment to print stats for a specific cluster say(target-cluster)
argocd admin cluster stats target-cluster
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
      --disable-compression                   If true, opt-out of response compression for all requests to the server
  -h, --help                                  help for stats
      --insecure-skip-tls-verify              If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string                     Path to a kube config. Only required if out-of-cluster
  -n, --namespace string                      If present, the namespace scope for this CLI request
      --password string                       Password for basic authentication to the API server
      --port-forward-redis                    Automatically port-forward ha proxy redis from current namespace? (default true)
      --proxy-url string                      If provided, this URL will be used to connect via proxy
      --redis string                          Redis server hostname and port (e.g. argocd-redis:6379). 
      --redis-ca-certificate string           Path to Redis server CA certificate (e.g. /etc/certs/redis/ca.crt). If not specified, system trusted CAs will be used for server certificate validation.
      --redis-client-certificate string       Path to Redis client certificate (e.g. /etc/certs/redis/client.crt).
      --redis-client-key string               Path to Redis client key (e.g. /etc/certs/redis/client.crt).
      --redis-compress string                 Enable compression for data sent to Redis with the required compression algorithm. (possible values: gzip, none) (default "gzip")
      --redis-insecure-skip-tls-verify        Skip Redis server certificate validation.
      --redis-use-tls                         Use TLS when connecting to Redis. 
      --redisdb int                           Redis database.
      --replicas int                          Application controller replicas count. Inferred from number of running controller pods if not specified
      --request-timeout string                The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --sentinel stringArray                  Redis sentinel hostname and port (e.g. argocd-redis-ha-announce-0:6379). 
      --sentinelmaster string                 Redis sentinel master group name. (default "master")
      --server string                         The address and port of the Kubernetes API server
      --shard int                             Cluster shard filter (default -1)
      --sharding-method string                Sharding method. Defaults: legacy. Supported sharding methods are : [legacy, round-robin, consistent-hashing]  (default "legacy")
      --tls-server-name string                If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                          Bearer token for authentication to the API server
      --user string                           The name of the kubeconfig user to use
      --username string                       Username for basic authentication to the API server
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
      --server-crt string               Server certificate file
      --server-name string              Name of the Argo CD API server; set this or the ARGOCD_SERVER_NAME environment variable when the server's name label differs from the default, for example when installing via the Helm chart (default "argocd-server")
```

### SEE ALSO

* [argocd admin cluster](argocd_admin_cluster.md)	 - Manage clusters configuration

