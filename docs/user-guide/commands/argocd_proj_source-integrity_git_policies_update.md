# `argocd proj source-integrity git policies update` Command Reference

## argocd proj source-integrity git policies update

Update a git source integrity policy

```
argocd proj source-integrity git policies update PROJECT POLICY_ID [flags]
```

### Examples

```
  # Update policy at index to set specific repo URLs, removing the old ones
  argocd proj source-integrity git policies update PROJECT POLICY_ID \
  --repo-url 'https://github.com/foo/*'
  
  # Update policy at index to add and remove repo URLs
  argocd proj source-integrity git policies update PROJECT POLICY_ID \
  --add-repo-url 'https://github.com/bar/*' \
  --delete-repo-url 'https://github.com/foo/*'
  
  # Update policy GPG mode and keys
  argocd proj source-integrity git policies update PROJECT POLICY_ID \
  --gpg-mode strict \
  --add-gpg-key D56C4FCA57A46444
```

### Options

```
      --add-gpg-key strings       Add GPG key ID
      --add-repo-url strings      Add repository URL pattern
      --delete-gpg-key strings    Delete GPG key ID
      --delete-repo-url strings   Delete repository URL pattern
      --gpg-key strings           Set GPG key ID (replaces existing)
      --gpg-mode string           Set GPG verification mode (strict, head, or none)
  -h, --help                      help for update
      --repo-url strings          Set repository URL pattern (replaces existing)
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

* [argocd proj source-integrity git policies](argocd_proj_source-integrity_git_policies.md)	 - Manage git source integrity policies

