# `argocd app get-resource` Command Reference

## argocd app get-resource

Get details about the live Kubernetes manifests of a resource in an application. The filter-fields flag can be used to only display fields you want to see.

```
argocd app get-resource APPNAME [flags]
```

### Examples

```

  # Get a specific resource, Pod my-app-pod, in 'my-app' by name in wide format
    argocd app get-resource my-app --kind Pod --resource-name my-app-pod

  # Get a specific resource, Pod my-app-pod, in 'my-app' by name in yaml format
    argocd app get-resource my-app --kind Pod --resource-name my-app-pod -o yaml

  # Get a specific resource, Pod my-app-pod, in 'my-app' by name in json format
    argocd app get-resource my-app --kind Pod --resource-name my-app-pod -o json

  # Get details about all Pods in the application
    argocd app get-resource my-app --kind Pod

  # Get a specific resource with managed fields, Pod my-app-pod, in 'my-app' by name in wide format
    argocd app get-resource my-app --kind Pod --resource-name my-app-pod --showManagedFields

  # Get the the details of a specific field in a resource in 'my-app' in the wide format
    argocd app get-resource my-app --kind Pod --filter-fields status.podIP

  # Get the details of multiple specific fields in a specific resource in 'my-app' in the wide format
    argocd app get-resource my-app --kind Pod --resource-name my-app-pod --filter-fields status.podIP,status.hostIP
```

### Options

```
      --filter-fields strings   A comma separated list of fields to display, if not provided will output the entire manifest
  -h, --help                    help for get-resource
      --kind string             Kind of resource [REQUIRED]
  -o, --output string           Format of the output, yaml or json (default "wide")
      --project string          Project of resource
      --resource-name string    Name of resource, if none is included will output details of all resources with specified kind
      --show-managed-fields     Show managed fields in the output manifest
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
      --logformat string                Set the logging format. One of: json|text (default "json")
      --loglevel string                 Set the logging level. One of: debug|info|warn|error (default "info")
      --plaintext                       Disable TLS
      --port-forward                    Connect to a random argocd-server port using port forwarding
      --port-forward-namespace string   Namespace name which should be used for port forwarding
      --prompts-enabled                 Force optional interactive prompts to be enabled or disabled, overriding local configuration. If not specified, the local configuration value will be used, which is false by default.
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

