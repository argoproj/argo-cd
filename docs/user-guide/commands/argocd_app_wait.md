## argocd app wait

Wait for an application to reach a synced and healthy state

```
argocd app wait [APPNAME.. | -l selector] [flags]
```

### Examples

```
  # Wait for an app
  argocd app wait my-app

  # Wait for multiple apps
  argocd app wait my-app other-app

  # Wait for apps by resource
  # Resource should be formatted as GROUP:KIND:NAME. If no GROUP is specified then :KIND:NAME.
  argocd app wait my-app --resource :Service:my-service
  argocd app wait my-app --resource argoproj.io:Rollout:my-rollout
  argocd app wait my-app --resource '!apps:Deployment:my-service'
  argocd app wait my-app --resource apps:Deployment:my-service --resource :Service:my-service
  argocd app wait my-app --resource '!*:Service:*'
  # Specify namespace if the application has resources with the same name in different namespaces
  argocd app wait my-app --resource argoproj.io:Rollout:my-namespace/my-rollout

  # Wait for apps by label, in this example we waiting for apps that are children of another app (aka app-of-apps)
  argocd app wait -l app.kubernetes.io/instance=my-app
  argocd app wait -l app.kubernetes.io/instance!=my-app
  argocd app wait -l app.kubernetes.io/instance
  argocd app wait -l '!app.kubernetes.io/instance'
  argocd app wait -l 'app.kubernetes.io/instance notin (my-app,other-app)'
```

### Options

```
      --degraded                       Wait for degraded
      --health                         Wait for health
  -h, --help                           help for wait
      --operation                      Wait for pending operations
      --redis-ha-haproxy-name string   Redis HA HAProxy name (default "argocd-redis-ha-haproxy")
      --redis-name string              Redis name (default "argocd-redis")
      --repo-server-name string        Repo server name (default "argocd-repo-server")
      --resource stringArray           Sync only specific resources as GROUP:KIND:NAME or !GROUP:KIND:NAME. Fields may be blank and '*' can be used. This option may be specified repeatedly
  -l, --selector string                Wait for apps by label. Supports '=', '==', '!=', in, notin, exists & not exists. Matching apps must satisfy all of the specified label constraints.
      --server-name string             Server name (default "argocd-server")
      --suspended                      Wait for suspended
      --sync                           Wait for sync
      --timeout uint                   Time out after this many seconds
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
      --kube-context string             Directs the command to the given kube-context
      --logformat string                Set the logging format. One of: text|json (default "text")
      --loglevel string                 Set the logging level. One of: debug|info|warn|error (default "info")
      --plaintext                       Disable TLS
      --port-forward                    Connect to a random argocd-server port using port forwarding
      --port-forward-namespace string   Namespace name which should be used for port forwarding
      --server string                   Argo CD server address
      --server-crt string               Server certificate file
```

### SEE ALSO

* [argocd app](argocd_app.md)	 - Manage applications

