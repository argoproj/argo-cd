# `argocd app sync` Command Reference

## argocd app sync

Sync an application to its target state

```
argocd app sync [APPNAME... | -l selector | --project project-name] [flags]
```

### Examples

```
  # Sync an app
  argocd app sync my-app

  # Sync multiples apps
  argocd app sync my-app other-app

  # Sync apps by label, in this example we sync apps that are children of another app (aka app-of-apps)
  argocd app sync -l app.kubernetes.io/instance=my-app
  argocd app sync -l app.kubernetes.io/instance!=my-app
  argocd app sync -l app.kubernetes.io/instance
  argocd app sync -l '!app.kubernetes.io/instance'
  argocd app sync -l 'app.kubernetes.io/instance notin (my-app,other-app)'

  # Sync a multi-source application for specific revision of specific sources
  argocd app manifests my-app --revisions 0.0.1 --source-positions 1 --revisions 0.0.2 --source-positions 2

  # Sync a specific resource
  # Resource should be formatted as GROUP:KIND:NAME. If no GROUP is specified then :KIND:NAME
  argocd app sync my-app --resource :Service:my-service
  argocd app sync my-app --resource argoproj.io:Rollout:my-rollout
  argocd app sync my-app --resource '!apps:Deployment:my-service'
  argocd app sync my-app --resource apps:Deployment:my-service --resource :Service:my-service
  argocd app sync my-app --resource '!*:Service:*'
  # Specify namespace if the application has resources with the same name in different namespaces
  argocd app sync my-app --resource argoproj.io:Rollout:my-namespace/my-rollout
```

### Options

```
  -N, --app-namespace string                              Only sync an application in namespace
      --apply-out-of-sync-only                            Sync only out-of-sync resources
      --assumeYes                                         Assume yes as answer for all user queries or prompts
      --async                                             Do not wait for application to sync before continuing
      --dry-run                                           Preview apply without affecting cluster
      --force                                             Use a force apply
  -h, --help                                              help for sync
      --ignore-normalizer-jq-execution-timeout duration   Set ignore normalizer JQ execution timeout (default 1s)
      --info stringArray                                  A list of key-value pairs during sync process. These infos will be persisted in app.
      --label stringArray                                 Sync only specific resources with a label. This option may be specified repeatedly.
      --local string                                      Path to a local directory. When this flag is present no git queries will be made
      --local-repo-root string                            Path to the repository root. Used together with --local allows setting the repository root (default "/")
  -o, --output string                                     Output format. One of: json|yaml|wide|tree|tree=detailed (default "wide")
      --preview-changes                                   Preview difference against the target and live state before syncing app and wait for user confirmation
      --project stringArray                               Sync apps that belong to the specified projects. This option may be specified repeatedly.
      --prune                                             Allow deleting unexpected resources
      --replace                                           Use a kubectl create/replace instead apply
      --resource stringArray                              Sync only specific resources as GROUP:KIND:NAME or !GROUP:KIND:NAME. Fields may be blank and '*' can be used. This option may be specified repeatedly
      --retry-backoff-duration duration                   Retry backoff base duration. Input needs to be a duration (e.g. 2m, 1h) (default 5s)
      --retry-backoff-factor int                          Factor multiplies the base duration after each failed retry (default 2)
      --retry-backoff-max-duration duration               Max retry backoff duration. Input needs to be a duration (e.g. 2m, 1h) (default 3m0s)
      --retry-limit int                                   Max number of allowed sync retries
      --revision string                                   Sync to a specific revision. Preserves parameter overrides
      --revisions stringArray                             Show manifests at specific revisions for source position in source-positions
  -l, --selector string                                   Sync apps that match this label. Supports '=', '==', '!=', in, notin, exists & not exists. Matching apps must satisfy all of the specified label constraints.
      --server-side                                       Use server-side apply while syncing the application
      --source-positions int64Slice                       List of source positions. Default is empty array. Counting start at 1. (default [])
      --strategy string                                   Sync strategy (one of: apply|hook)
      --timeout uint                                      Time out after this many seconds
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

