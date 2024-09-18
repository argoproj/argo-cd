# `argocd proj windows` Command Reference

## argocd proj windows

Manage a project's sync windows

```
argocd proj windows [flags]
```

### Examples

```

#Add a sync window to a project
argocd proj windows add my-project \
--schedule "0 0 * * 1-5" \
--duration 3600 \
--prune

#Delete a sync window from a project 
argocd proj windows delete <project-name> <window-id>

#List project sync windows
argocd proj windows list <project-name>
```

### Options

```
  -h, --help   help for windows
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

* [argocd proj](argocd_proj.md)	 - Manage projects
* [argocd proj windows add](argocd_proj_windows_add.md)	 - Add a sync window to a project
* [argocd proj windows delete](argocd_proj_windows_delete.md)	 - Delete a sync window from a project. Requires ID which can be found by running "argocd proj windows list PROJECT"
* [argocd proj windows disable-manual-sync](argocd_proj_windows_disable-manual-sync.md)	 - Disable manual sync for a sync window
* [argocd proj windows enable-manual-sync](argocd_proj_windows_enable-manual-sync.md)	 - Enable manual sync for a sync window
* [argocd proj windows list](argocd_proj_windows_list.md)	 - List project sync windows
* [argocd proj windows update](argocd_proj_windows_update.md)	 - Update a project sync window

