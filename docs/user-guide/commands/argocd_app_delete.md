## argocd app delete

Delete an application

```
argocd app delete APPNAME [flags]
```

### Examples

```
  # Delete an app
  argocd app delete my-app

  # Delete multiple apps
  argocd app delete my-app other-app

  # Delete apps by label
  argocd app delete -l app.kubernetes.io/instance=my-app
  argocd app delete -l app.kubernetes.io/instance!=my-app
  argocd app delete -l app.kubernetes.io/instance
  argocd app delete -l '!app.kubernetes.io/instance'
  argocd app delete -l 'app.kubernetes.io/instance notin (my-app,other-app)'
```

### Options

```
      --cascade                     Perform a cascaded deletion of all application resources (default true)
  -h, --help                        help for delete
  -p, --propagation-policy string   Specify propagation policy for deletion of application's resources. One of: foreground|background (default "foreground")
  -l, --selector string             Delete all apps with matching label. Supports '=', '==', '!=', in, notin, exists & not exists. Matching apps must satisfy all of the specified label constraints.
  -y, --yes                         Turn off prompting to confirm cascaded deletion of application resources
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

