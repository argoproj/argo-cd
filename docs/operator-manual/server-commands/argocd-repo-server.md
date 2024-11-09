# `argocd-repo-server` Command Reference

## argocd-repo-server

Run ArgoCD Repository Server

### Synopsis

ArgoCD Repository Server is an internal service which maintains a local cache of the Git repository holding the application manifests, and is responsible for generating and returning the Kubernetes manifests.  This command runs Repository Server in the foreground.  It can be configured by following options.

```
argocd-repo-server [flags]
```

### Options

```
      --address string                                 Listen on given address for incoming connections (default "0.0.0.0")
      --allow-oob-symlinks                             Allow out-of-bounds symlinks in repositories (not recommended)
      --default-cache-expiration duration              Cache expiration default (default 24h0m0s)
      --disable-helm-manifest-max-extracted-size       Disable maximum size of helm manifest archives when extracted
      --disable-tls                                    Disable TLS on the gRPC endpoint
      --helm-manifest-max-extracted-size string        Maximum size of helm manifest archives when extracted (default "1G")
      --helm-registry-max-index-size string            Maximum size of registry index file (default "1G")
  -h, --help                                           help for argocd-repo-server
      --include-hidden-directories                     Include hidden directories from Git
      --logformat string                               Set the logging format. One of: text|json (default "text")
      --loglevel string                                Set the logging level. One of: debug|info|warn|error (default "info")
      --max-combined-directory-manifests-size string   Max combined size of manifest files in a directory-type Application (default "10M")
      --metrics-address string                         Listen on given address for metrics (default "0.0.0.0")
      --metrics-port int                               Start metrics server on given port (default 8084)
      --otlp-address string                            OpenTelemetry collector address to send traces to
      --otlp-attrs strings                             List of OpenTelemetry collector extra attrs when send traces, each attribute is separated by a colon(e.g. key:value)
      --otlp-headers stringToString                    List of OpenTelemetry collector extra headers sent with traces, headers are comma-separated key-value pairs(e.g. key1=value1,key2=value2) (default [])
      --otlp-insecure                                  OpenTelemetry collector insecure mode (default true)
      --parallelismlimit int                           Limit on number of concurrent manifests generate requests. Any value less the 1 means no limit.
      --plugin-tar-exclude stringArray                 Globs to filter when sending tarballs to plugins.
      --plugin-use-manifest-generate-paths             Pass the resources described in argocd.argoproj.io/manifest-generate-paths value to the cmpserver to generate the application manifests.
      --port int                                       Listen on given port for incoming connections (default 8081)
      --redis string                                   Redis server hostname and port (e.g. argocd-redis:6379). 
      --redis-ca-certificate string                    Path to Redis server CA certificate (e.g. /etc/certs/redis/ca.crt). If not specified, system trusted CAs will be used for server certificate validation.
      --redis-client-certificate string                Path to Redis client certificate (e.g. /etc/certs/redis/client.crt).
      --redis-client-key string                        Path to Redis client key (e.g. /etc/certs/redis/client.crt).
      --redis-compress string                          Enable compression for data sent to Redis with the required compression algorithm. (possible values: gzip, none) (default "gzip")
      --redis-insecure-skip-tls-verify                 Skip Redis server certificate validation.
      --redis-use-tls                                  Use TLS when connecting to Redis. 
      --redisdb int                                    Redis database.
      --repo-cache-expiration duration                 Cache expiration for repo state, incl. app lists, app details, manifest generation, revision meta-data (default 24h0m0s)
      --revision-cache-expiration duration             Cache expiration for cached revision (default 3m0s)
      --revision-cache-lock-timeout duration           Cache TTL for locks to prevent duplicate requests on revisions, set to 0 to disable (default 10s)
      --sentinel stringArray                           Redis sentinel hostname and port (e.g. argocd-redis-ha-announce-0:6379). 
      --sentinelmaster string                          Redis sentinel master group name. (default "master")
      --streamed-manifest-max-extracted-size string    Maximum size of streamed manifest archives when extracted (default "1G")
      --streamed-manifest-max-tar-size string          Maximum size of streamed manifest archives (default "100M")
      --tlsciphers string                              The list of acceptable ciphers to be used when establishing TLS connections. Use 'list' to list available ciphers. (default "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384")
      --tlsmaxversion string                           The maximum SSL/TLS version that is acceptable (one of: 1.0|1.1|1.2|1.3) (default "1.3")
      --tlsminversion string                           The minimum SSL/TLS version that is acceptable (one of: 1.0|1.1|1.2|1.3) (default "1.2")
```

