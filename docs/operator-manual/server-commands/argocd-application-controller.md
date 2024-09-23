# `argocd-application-controller` Command Reference

## argocd-application-controller

Run ArgoCD Application Controller

### Synopsis

ArgoCD application controller is a Kubernetes controller that continuously monitors running applications and compares the current, live state against the desired target state (as specified in the repo). This command runs Application Controller in the foreground.  It can be configured by following options.

```
argocd-application-controller [flags]
```

### Options

```
      --app-hard-resync int                                       Time period in seconds for application hard resync.
      --app-resync int                                            Time period in seconds for application resync. (default 180)
      --app-resync-jitter int                                     Maximum time period in seconds to add as a delay jitter for application resync.
      --app-state-cache-expiration duration                       Cache expiration for app state (default 1h0m0s)
      --application-namespaces strings                            List of additional namespaces that applications are allowed to be reconciled from
      --as string                                                 Username to impersonate for the operation
      --as-group stringArray                                      Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                                             UID to impersonate for the operation
      --certificate-authority string                              Path to a cert file for the certificate authority
      --client-certificate string                                 Path to a client certificate file for TLS
      --client-key string                                         Path to a client key file for TLS
      --cluster string                                            The name of the kubeconfig cluster to use
      --context string                                            The name of the kubeconfig context to use
      --default-cache-expiration duration                         Cache expiration default (default 24h0m0s)
      --disable-compression                                       If true, opt-out of response compression for all requests to the server
      --dynamic-cluster-distribution-enabled                      Enables dynamic cluster distribution.
      --enable-k8s-event none                                     Enable ArgoCD to use k8s event. For disabling all events, set the value as none. (e.g --enable-k8s-event=none), For enabling specific events, set the value as `event reason`. (e.g --enable-k8s-event=StatusRefreshed,ResourceCreated) (default [all])
      --gloglevel int                                             Set the glog logging level
  -h, --help                                                      help for argocd-application-controller
      --ignore-normalizer-jq-execution-timeout-seconds duration   Set ignore normalizer JQ execution timeout
      --insecure-skip-tls-verify                                  If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string                                         Path to a kube config. Only required if out-of-cluster
      --kubectl-parallelism-limit int                             Number of allowed concurrent kubectl fork/execs. Any value less than 1 means no limit. (default 20)
      --logformat string                                          Set the logging format. One of: text|json (default "text")
      --loglevel string                                           Set the logging level. One of: debug|info|warn|error (default "info")
      --metrics-application-conditions strings                    List of Application conditions that will be added to the argocd_application_conditions metric
      --metrics-application-labels strings                        List of Application labels that will be added to the argocd_application_labels metric
      --metrics-cache-expiration duration                         Prometheus metrics cache expiration (disabled  by default. e.g. 24h0m0s)
      --metrics-port int                                          Start metrics server on given port (default 8082)
  -n, --namespace string                                          If present, the namespace scope for this CLI request
      --operation-processors int                                  Number of application operation processors (default 10)
      --otlp-address string                                       OpenTelemetry collector address to send traces to
      --otlp-attrs strings                                        List of OpenTelemetry collector extra attrs when send traces, each attribute is separated by a colon(e.g. key:value)
      --otlp-headers stringToString                               List of OpenTelemetry collector extra headers sent with traces, headers are comma-separated key-value pairs(e.g. key1=value1,key2=value2) (default [])
      --otlp-insecure                                             OpenTelemetry collector insecure mode (default true)
      --password string                                           Password for basic authentication to the API server
      --persist-resource-health                                   Enables storing the managed resources health in the Application CRD (default true)
      --proxy-url string                                          If provided, this URL will be used to connect via proxy
      --redis string                                              Redis server hostname and port (e.g. argocd-redis:6379). 
      --redis-ca-certificate string                               Path to Redis server CA certificate (e.g. /etc/certs/redis/ca.crt). If not specified, system trusted CAs will be used for server certificate validation.
      --redis-client-certificate string                           Path to Redis client certificate (e.g. /etc/certs/redis/client.crt).
      --redis-client-key string                                   Path to Redis client key (e.g. /etc/certs/redis/client.crt).
      --redis-compress string                                     Enable compression for data sent to Redis with the required compression algorithm. (possible values: gzip, none) (default "gzip")
      --redis-insecure-skip-tls-verify                            Skip Redis server certificate validation.
      --redis-use-tls                                             Use TLS when connecting to Redis. 
      --redisdb int                                               Redis database.
      --repo-error-grace-period-seconds int                       Grace period in seconds for ignoring consecutive errors while communicating with repo server. (default 180)
      --repo-server string                                        Repo server address. (default "argocd-repo-server:8081")
      --repo-server-plaintext                                     Disable TLS on connections to repo server
      --repo-server-strict-tls                                    Whether to use strict validation of the TLS cert presented by the repo server
      --repo-server-timeout-seconds int                           Repo server RPC call timeout seconds. (default 60)
      --request-timeout string                                    The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --self-heal-timeout-seconds int                             Specifies timeout between application self heal attempts (default 5)
      --sentinel stringArray                                      Redis sentinel hostname and port (e.g. argocd-redis-ha-announce-0:6379). 
      --sentinelmaster string                                     Redis sentinel master group name. (default "master")
      --server string                                             The address and port of the Kubernetes API server
      --server-side-diff-enabled                                  Feature flag to enable ServerSide diff. Default ("false")
      --sharding-method string                                    Enables choice of sharding method. Supported sharding methods are : [legacy, round-robin, consistent-hashing]  (default "legacy")
      --status-processors int                                     Number of application status processors (default 20)
      --tls-server-name string                                    If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                                              Bearer token for authentication to the API server
      --user string                                               The name of the kubeconfig user to use
      --username string                                           Username for basic authentication to the API server
      --wq-backoff-factor float                                   Set Workqueue Per Item Rate Limiter Backoff Factor, default is 1.5 (default 1.5)
      --wq-basedelay-ns duration                                  Set Workqueue Per Item Rate Limiter Base Delay duration in nanoseconds, default 1000000 (1ms) (default 1ms)
      --wq-bucket-qps float                                       Set Workqueue Rate Limiter Bucket QPS, default set to MaxFloat64 which disables the bucket limiter (default 1.7976931348623157e+308)
      --wq-bucket-size int                                        Set Workqueue Rate Limiter Bucket Size, default 500 (default 500)
      --wq-cooldown-ns duration                                   Set Workqueue Per Item Rate Limiter Cooldown duration in ns, default 0(per item rate limiter disabled)
      --wq-maxdelay-ns duration                                   Set Workqueue Per Item Rate Limiter Max Delay duration in nanoseconds, default 1000000000 (1s) (default 1s)
```

