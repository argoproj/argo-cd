# `argocd app logs` Command Reference

## argocd app logs

Get logs of application pods

```
argocd app logs APPNAME [flags]
```

### Examples

```
  # Get logs of pods associated with the application "my-app"
  argocd app logs my-app
  
  # Get logs of pods associated with the application "my-app" in a specific resource group
  argocd app logs my-app --group my-group
  
  # Get logs of pods associated with the application "my-app" in a specific resource kind
  argocd app logs my-app --kind my-kind
  
  # Get logs of pods associated with the application "my-app" in a specific namespace
  argocd app logs my-app --namespace my-namespace
  
  # Get logs of pods associated with the application "my-app" for a specific resource name
  argocd app logs my-app --name my-resource
  
  # Stream logs in real-time for the application "my-app"
  argocd app logs my-app -f
  
  # Get the last N lines of logs for the application "my-app"
  argocd app logs my-app --tail 100
  
  # Get logs since a specified number of seconds ago
  argocd app logs my-app --since-seconds 3600
  
  # Get logs until a specified time (format: "2023-10-10T15:30:00Z")
  argocd app logs my-app --until-time "2023-10-10T15:30:00Z"
  
  # Filter logs to show only those containing a specific string
  argocd app logs my-app --filter "error"
  
  # Get logs for a specific container within the pods
  argocd app logs my-app -c my-container
  
  # Get previously terminated container logs
  argocd app logs my-app -p
```

### Options

```
  -c, --container string    Optional container name
      --filter string       Show logs contain this string
  -f, --follow              Specify if the logs should be streamed
      --group string        Resource group
  -h, --help                help for logs
      --kind string         Resource kind
      --name string         Resource name
      --namespace string    Resource namespace
  -p, --previous            Specify if the previously terminated container logs should be returned
      --since-seconds int   A relative time in seconds before the current time from which to show logs
      --tail int            The number of lines from the end of the logs to show
      --until-time string   Show logs until this time
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

