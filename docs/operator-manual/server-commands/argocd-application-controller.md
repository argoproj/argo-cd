## argocd-application-controller

Run ArgoCD Application Controller

### Synopsis

ArgoCD application controller is a Kubernetes controller that continuously monitors running applications and compares the current, live state against the desired target state (as specified in the repo). This command runs Application Controller in the foreground.  It can be configured by following options.

```
argocd-application-controller [flags]
```

### Options

```
      --app-resync int                        Time period in seconds for application resync. (default 180)
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
      --gloglevel int                         Set the glog logging level
  -h, --help                                  help for argocd-application-controller
      --insecure-skip-tls-verify              If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string                     Path to a kube config. Only required if out-of-cluster
      --kubectl-parallelism-limit int         Number of allowed concurrent kubectl fork/execs. Any value less the 1 means no limit. (default 20)
      --logformat string                      Set the logging format. One of: text|json (default "text")
      --loglevel string                       Set the logging level. One of: debug|info|warn|error (default "info")
      --metrics-application-labels strings    List of Application labels that will be added to the argocd_application_labels metric
      --metrics-cache-expiration duration     Prometheus metrics cache expiration (disabled  by default. e.g. 24h0m0s)
      --metrics-port int                      Start metrics server on given port (default 8082)
  -n, --namespace string                      If present, the namespace scope for this CLI request
      --operation-processors int              Number of application operation processors (default 10)
      --password string                       Password for basic authentication to the API server
      --redis string                          Redis server hostname and port (e.g. argocd-redis:6379). 
      --redis-ca-certificate string           Path to Redis server CA certificate (e.g. /etc/certs/redis/ca.crt). If not specified, system trusted CAs will be used for server certificate validation.
      --redis-client-certificate string       Path to Redis client certificate (e.g. /etc/certs/redis/client.crt).
      --redis-client-key string               Path to Redis client key (e.g. /etc/certs/redis/client.crt).
      --redis-insecure-skip-tls-verify        Skip Redis server certificate validation.
      --redis-use-tls                         Use TLS when connecting to Redis. 
      --redisdb int                           Redis database.
      --repo-server string                    Repo server address. (default "argocd-repo-server:8081")
      --repo-server-plaintext                 Disable TLS on connections to repo server
      --repo-server-strict-tls                Whether to use strict validation of the TLS cert presented by the repo server
      --repo-server-timeout-seconds int       Repo server RPC call timeout seconds. (default 60)
      --request-timeout string                The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --self-heal-timeout-seconds int         Specifies timeout between application self heal attempts (default 5)
      --sentinel stringArray                  Redis sentinel hostname and port (e.g. argocd-redis-ha-announce-0:6379). 
      --sentinelmaster string                 Redis sentinel master group name. (default "master")
      --server string                         The address and port of the Kubernetes API server
      --status-processors int                 Number of application status processors (default 20)
      --tls-server-name string                If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                          Bearer token for authentication to the API server
      --user string                           The name of the kubeconfig user to use
      --username string                       Username for basic authentication to the API server
```

