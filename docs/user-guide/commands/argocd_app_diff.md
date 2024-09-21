# `argocd app diff` Command Reference

## argocd app diff

Perform a diff against the target and live state.

### Synopsis

Perform a diff against the target and live state.
Uses 'diff' to render the difference. KUBECTL_EXTERNAL_DIFF environment variable can be used to select your own diff tool.
Returns the following exit codes: 2 on general errors, 1 when a diff is found, and 0 when no diff is found
Kubernetes Secrets are ignored from this diff.

```
argocd app diff APPNAME [flags]
```

### Options

```
  -N, --app-namespace string                              Only render the difference in namespace
      --exit-code                                         Return non-zero exit code when there is a diff (default true)
      --hard-refresh                                      Refresh application data as well as target manifests cache
  -h, --help                                              help for diff
      --ignore-normalizer-jq-execution-timeout duration   Set ignore normalizer JQ execution timeout (default 1s)
      --local string                                      Compare live app to a local manifests
      --local-include stringArray                         Used with --server-side-generate, specify patterns of filenames to send. Matching is based on filename and not path. (default [*.yaml,*.yml,*.json])
      --local-repo-root string                            Path to the repository root. Used together with --local allows setting the repository root (default "/")
      --refresh                                           Refresh application data when retrieving
      --revision string                                   Compare live app to a particular revision
      --revisions stringArray                             Show manifests at specific revisions for source position in source-positions
      --server-side-generate                              Used with --local, this will send your manifests to the server for diffing
      --source-positions int64Slice                       List of source positions. Default is empty array. Counting start at 1. (default [])
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

* [argocd app](argocd_app.md)	 - Manage applications

