## argocd app sync

Sync an application to its target state

```
argocd app sync [APPNAME... | -l selector] [flags]
```

### Examples

```
  # Sync an app
  argocd app sync my-app

  # Sync multiples apps
  argocd app sync my-app other-app

  # Sync apps by label, in this example we sync apps that are children of another app (aka app-of-apps)
  argocd app sync -l app.kubernetes.io/instance=my-app

  # Sync a specific resource
  # Resource should be formatted as GROUP:KIND:NAME. If no GROUP is specified then :KIND:NAME
  argocd app sync my-app --resource :Service:my-service
  argocd app sync my-app --resource argoproj.io:Rollout:my-rollout
  # Specify namespace if the application has resources with the same name in different namespaces
  argocd app sync my-app --resource argoproj.io:Rollout:my-namespace/my-rollout
```

### Options

```
      --assumeYes                             Assume yes as answer for all user queries or prompts
      --async                                 Do not wait for application to sync before continuing
      --dry-run                               Preview apply without affecting cluster
      --force                                 Use a force apply
  -h, --help                                  help for sync
      --info stringArray                      A list of key-value pairs during sync process. These infos will be persisted in app.
      --label stringArray                     Sync only specific resources with a label. This option may be specified repeatedly.
      --local string                          Path to a local directory. When this flag is present no git queries will be made
      --local-repo-root string                Path to the repository root. Used together with --local allows setting the repository root (default "/")
      --preview-changes                       Preview difference against the target and live state before syncing app and wait for user confirmation
      --prune                                 Allow deleting unexpected resources
      --replace                               Use a kubectl create/replace instead apply
      --resource stringArray                  Sync only specific resources as GROUP:KIND:NAME. Fields may be blank. This option may be specified repeatedly
      --retry-backoff-duration duration       Retry backoff base duration. Input needs to be a duration (e.g. 2m, 1h) (default 5s)
      --retry-backoff-factor int              Factor multiplies the base duration after each failed retry (default 2)
      --retry-backoff-max-duration duration   Max retry backoff duration. Input needs to be a duration (e.g. 2m, 1h) (default 3m0s)
      --retry-limit int                       Max number of allowed sync retries
      --revision string                       Sync to a specific revision. Preserves parameter overrides
  -l, --selector string                       Sync apps that match this label
      --strategy string                       Sync strategy (one of: apply|hook)
      --timeout uint                          Time out after this many seconds
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

* [argocd app](argocd_app.md)	 - Manage applications

