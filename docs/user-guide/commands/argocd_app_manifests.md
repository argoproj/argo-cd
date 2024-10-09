# `argocd app manifests` Command Reference

## argocd app manifests

Print manifests of an application

```
argocd app manifests APPNAME [flags]
```

### Examples

```
  # Get manifests for an application
  argocd app manifests my-app
  
  # Get manifests for an application at a specific revision
  argocd app manifests my-app --revision 0.0.1
  
  # Get manifests for a multi-source application at specific revisions for specific sources
  argocd app manifests my-app --revisions 0.0.1 --source-positions 1 --revisions 0.0.2 --source-positions 2
```

### Options

```
  -h, --help                          help for manifests
      --local string                  If set, show locally-generated manifests. Value is the absolute path to app manifests within the manifest repo. Example: '/home/username/apps/env/app-1'.
      --local-repo-root string        Path to the local repository root. Used together with --local allows setting the repository root. Example: '/home/username/apps'. (default ".")
      --revision string               Show manifests at a specific revision
      --revisions stringArray         Show manifests at specific revisions for the source at position in source-positions
      --source string                 Source of manifests. One of: live|git (default "git")
      --source-positions int64Slice   List of source positions. Default is empty array. Counting start at 1. (default [])
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

