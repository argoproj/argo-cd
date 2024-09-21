# `argocd app wait` Command Reference

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
  -N, --app-namespace string   Only wait for an application  in namespace
      --degraded               Wait for degraded
      --delete                 Wait for delete
      --health                 Wait for health
  -h, --help                   help for wait
      --operation              Wait for pending operations
  -o, --output string          Output format. One of: json|yaml|wide|tree|tree=detailed (default "wide")
      --resource stringArray   Sync only specific resources as GROUP:KIND:NAME or !GROUP:KIND:NAME. Fields may be blank and '*' can be used. This option may be specified repeatedly
  -l, --selector string        Wait for apps by label. Supports '=', '==', '!=', in, notin, exists & not exists. Matching apps must satisfy all of the specified label constraints.
      --suspended              Wait for suspended
      --sync                   Wait for sync
      --timeout uint           Time out after this many seconds
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

