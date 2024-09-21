# `argocd` Command Reference

## argocd

argocd controls a Argo CD server

```
argocd [flags]
```

### Options

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
  -h, --help                            help for argocd
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

* [argocd account](argocd_account.md)	 - Manage account settings
* [argocd admin](argocd_admin.md)	 - Contains a set of commands useful for Argo CD administrators and requires direct Kubernetes access
* [argocd app](argocd_app.md)	 - Manage applications
* [argocd appset](argocd_appset.md)	 - Manage ApplicationSets
* [argocd cert](argocd_cert.md)	 - Manage repository certificates and SSH known hosts entries
* [argocd cluster](argocd_cluster.md)	 - Manage cluster credentials
* [argocd completion](argocd_completion.md)	 - output shell completion code for the specified shell (bash, zsh or fish)
* [argocd context](argocd_context.md)	 - Switch between contexts
* [argocd gpg](argocd_gpg.md)	 - Manage GPG keys used for signature verification
* [argocd login](argocd_login.md)	 - Log in to Argo CD
* [argocd logout](argocd_logout.md)	 - Log out from Argo CD
* [argocd proj](argocd_proj.md)	 - Manage projects
* [argocd relogin](argocd_relogin.md)	 - Refresh an expired authenticate token
* [argocd repo](argocd_repo.md)	 - Manage repository connection parameters
* [argocd repocreds](argocd_repocreds.md)	 - Manage repository connection parameters
* [argocd version](argocd_version.md)	 - Print version information

