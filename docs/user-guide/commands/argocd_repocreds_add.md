# `argocd repocreds add` Command Reference

## argocd repocreds add

Add git repository connection parameters

```
argocd repocreds add REPOURL [flags]
```

### Examples

```
  # Add credentials with user/pass authentication to use for all repositories under https://git.example.com/repos
  argocd repocreds add https://git.example.com/repos/ --username git --password secret

  # Add credentials with SSH private key authentication to use for all repositories under ssh://git@git.example.com/repos
  argocd repocreds add ssh://git@git.example.com/repos/ --ssh-private-key-path ~/.ssh/id_rsa

  # Add credentials with GitHub App authentication to use for all repositories under https://github.com/repos
  argocd repocreds add https://github.com/repos/ --github-app-id 1 --github-app-installation-id 2 --github-app-private-key-path test.private-key.pem

  # Add credentials with GitHub App authentication to use for all repositories under https://ghe.example.com/repos
  argocd repocreds add https://ghe.example.com/repos/ --github-app-id 1 --github-app-installation-id 2 --github-app-private-key-path test.private-key.pem --github-app-enterprise-base-url https://ghe.example.com/api/v3

  # Add credentials with helm oci registry so that these oci registry urls do not need to be added as repos individually.
  argocd repocreds add localhost:5000/myrepo --enable-oci --type helm 

  # Add credentials with GCP credentials for all repositories under https://source.developers.google.com/p/my-google-cloud-project/r/
  argocd repocreds add https://source.developers.google.com/p/my-google-cloud-project/r/ --gcp-service-account-key-path service-account-key.json

```

### Options

```
      --enable-oci                              Specifies whether helm-oci support should be enabled for this repo
      --force-http-basic-auth                   whether to force basic auth when connecting via HTTP
      --gcp-service-account-key-path string     service account key for the Google Cloud Platform
      --github-app-enterprise-base-url string   base url to use when using GitHub Enterprise (e.g. https://ghe.example.com/api/v3
      --github-app-id int                       id of the GitHub Application
      --github-app-installation-id int          installation id of the GitHub Application
      --github-app-private-key-path string      private key of the GitHub Application
  -h, --help                                    help for add
      --password string                         password to the repository
      --proxy-url string                        If provided, this URL will be used to connect via proxy
      --ssh-private-key-path string             path to the private ssh key (e.g. ~/.ssh/id_rsa)
      --tls-client-cert-key-path string         path to the TLS client cert's key path (must be PEM format)
      --tls-client-cert-path string             path to the TLS client cert (must be PEM format)
      --type string                             type of the repository, "git" or "helm" (default "git")
      --upsert                                  Override an existing repository with the same name even if the spec differs
      --username string                         username to the repository
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

* [argocd repocreds](argocd_repocreds.md)	 - Manage repository connection parameters

