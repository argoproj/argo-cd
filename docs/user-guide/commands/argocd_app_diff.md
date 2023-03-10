## argocd app diff

Perform a diff against the target and live state.

### Synopsis

Perform a diff against the target and live state.
Uses 'diff' to render the difference. KUBECTL_EXTERNAL_DIFF environment variable can be used to select your own diff tool.
Returns the following exit codes: 2 on general errors, 1 when a diff is found, and 0 when no diff is found

```
argocd app diff APPNAME [flags]
```

### Examples

```
  # Diff the live state against the target state
  argocd app diff guestbook

  # Diff the live state against the target state and exit with code 1 if a diff is found
  argocd app diff guestbook --exit-code

  # Diff the live state against the target state if the target revision were different
  argocd app diff guestbook --revision HEAD~1

  # Diff the live state against the target state if the application manifest were different
  # Note that the app's name, namespace, and project must be the same as the live application.
  argocd app diff guestbook --app ./app.yaml

```

### Options

```
      --app string                  Path to an app manifest to compare to the live version
      --exit-code                   Return non-zero exit code when there is a diff (default true)
      --hard-refresh                Refresh application data as well as target manifests cache
  -h, --help                        help for diff
      --local string                Compare live app to a local manifests
      --local-include stringArray   Used with --server-side-generate, specify patterns of filenames to send. Matching is based on filename and not path. (default [*.yaml,*.yml,*.json])
      --local-repo-root string      Path to the repository root. Used together with --local allows setting the repository root (default "/")
      --refresh                     Refresh application data when retrieving
      --revision string             Compare live app to a particular revision
      --server-side-generate        Used with --local, this will send your manifests to the server for diffing
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

