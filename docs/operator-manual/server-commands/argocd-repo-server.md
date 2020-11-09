## argocd-repo-server

Run ArgoCD Repository Server

### Synopsis

ArgoCD Repository Server is an internal service which maintains a local cache of the Git repository holding the application manifests, and is responsible for generating and returning the Kubernetes manifests.  This command runs Repository Server in the foreground.  It can be configured by following options.

```
argocd-repo-server [flags]
```

### Options

```
      --default-cache-expiration duration   Cache expiration default (default 24h0m0s)
  -h, --help                                help for argocd-repo-server
      --logformat string                    Set the logging format. One of: text|json (default "text")
      --loglevel string                     Set the logging level. One of: debug|info|warn|error (default "info")
      --metrics-port int                    Start metrics server on given port (default 8084)
      --parallelismlimit int                Limit on number of concurrent manifests generate requests. Any value less the 1 means no limit.
      --port int                            Listen on given port for incoming connections (default 8081)
      --redis string                        Redis server hostname and port (e.g. argocd-redis:6379). 
      --redisdb int                         Redis database.
      --repo-cache-expiration duration      Cache expiration for repo state, incl. app lists, app details, manifest generation, revision meta-data (default 24h0m0s)
      --sentinel stringArray                Redis sentinel hostname and port (e.g. argocd-redis-ha-announce-0:6379). 
      --sentinelmaster string               Redis sentinel master group name. (default "master")
      --tlsciphers string                   The list of acceptable ciphers to be used when establishing TLS connections. Use 'list' to list available ciphers. (default "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_RSA_WITH_AES_256_GCM_SHA384")
      --tlsmaxversion string                The maximum SSL/TLS version that is acceptable (one of: 1.0|1.1|1.2|1.3) (default "1.3")
      --tlsminversion string                The minimum SSL/TLS version that is acceptable (one of: 1.0|1.1|1.2|1.3) (default "1.2")
```

