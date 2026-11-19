# `argocd app events` Command Reference

## argocd app events

List the events of an application or of one of its resources

### Synopsis

List the events of an application or of one of its resources. Without --kind and --resource-name, the events of the Application object are listed. With them, the events of the matching resource in the application are listed.

```
argocd app events APPNAME [flags]
```

### Examples

```
  # List the events of the application
  argocd app events my-app

  # List the events of a resource in the application
  argocd app events my-app --kind Deployment --resource-name guestbook-ui

  # Select the resource by group and namespace when several resources match
  argocd app events my-app --kind Deployment --group apps --namespace default --resource-name guestbook-ui

  # Print the events in JSON format
  argocd app events my-app -o json
```

### Options

```
  -N, --app-namespace string   Namespace of the application
      --group string           Group of the resource
  -h, --help[=true|false]      help for events
      --kind string            Kind of the resource
      --namespace string       Namespace of the resource
  -o, --output string          Output format. One of: wide|json|yaml (default "wide")
      --resource-name string   Name of the resource
```

### Options inherited from parent commands

```
      --argocd-context string           The name of the Argo-CD server context to use
      --auth-token string               Authentication token; set this or the ARGOCD_AUTH_TOKEN environment variable
      --client-crt string               Client certificate file
      --client-crt-key string           Client certificate key file
      --config string                   Path to Argo CD config (default "/home/user/.config/argocd/config")
      --controller-name string          Name of the Argo CD Application controller; set this or the ARGOCD_APPLICATION_CONTROLLER_NAME environment variable when the controller's name label differs from the default, for example when installing via the Helm chart (default "argocd-application-controller")
      --core[=true|false]               If set to true then CLI talks directly to Kubernetes instead of talking to Argo CD API server
      --grpc-web[=true|false]           Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2.
      --grpc-web-root-path string       Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2. Set web root.
  -H, --header strings                  Sets additional header to all requests made by Argo CD CLI. (Can be repeated multiple times to add multiple headers, also supports comma separated headers)
      --http-retry-max int              Maximum number of retries to establish http connection to Argo CD server
      --insecure[=true|false]           Skip server certificate and domain verification
      --kube-context string             Directs the command to the given kube-context
      --logformat string                Set the logging format. One of: json|text (default "json")
      --loglevel string                 Set the logging level. One of: debug|info|warn|error (default "info")
      --plaintext[=true|false]          Disable TLS
      --port-forward[=true|false]       Connect to a random argocd-server port using port forwarding
      --port-forward-namespace string   Namespace name which should be used for port forwarding
      --prompts-enabled[=true|false]    Force optional interactive prompts to be enabled or disabled, overriding local configuration. If not specified, the local configuration value will be used, which is false by default.
      --redis-compress string           Enable this if the application controller is configured with redis compression enabled. (possible values: gzip, none) (default "gzip")
      --redis-haproxy-name string       Name of the Redis HA Proxy; set this or the ARGOCD_REDIS_HAPROXY_NAME environment variable when the HA Proxy's name label differs from the default, for example when installing via the Helm chart (default "argocd-redis-ha-haproxy")
      --redis-name string               Name of the Redis deployment; set this or the ARGOCD_REDIS_NAME environment variable when the Redis's name label differs from the default, for example when installing via the Helm chart (default "argocd-redis")
      --repo-server-name string         Name of the Argo CD Repo server; set this or the ARGOCD_REPO_SERVER_NAME environment variable when the server's name label differs from the default, for example when installing via the Helm chart (default "argocd-repo-server")
      --server string                   Argo CD server address
      --server-crt string               Server certificate file
      --server-name string              Name of the Argo CD API server; set this or the ARGOCD_SERVER_NAME environment variable when the server's name label differs from the default, for example when installing via the Helm chart (default "argocd-server")
```

### SEE ALSO

* [argocd app](argocd_app.md)	 - Manage applications

