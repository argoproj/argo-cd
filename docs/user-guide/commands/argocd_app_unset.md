# `argocd app unset` Command Reference

## argocd app unset

Unset application parameters

```
argocd app unset APPNAME parameters [flags]
```

### Examples

```
  # Unset kustomize override kustomize image
  argocd app unset my-app --kustomize-image=alpine

  # Unset kustomize override suffix
  argocd app unset my-app --namesuffix

  # Unset kustomize override suffix for source at position 1 under spec.sources of app my-app. source-position starts at 1.
  argocd app unset my-app --source-position 1 --namesuffix

  # Unset parameter override
  argocd app unset my-app -p COMPONENT=PARAM
```

### Options

```
  -N, --app-namespace string            Unset application parameters in namespace
  -h, --help                            help for unset
      --ignore-missing-value-files      Unset the helm ignore-missing-value-files option (revert to false)
      --kustomize-image stringArray     Kustomize images name (e.g. --kustomize-image node --kustomize-image mysql)
      --kustomize-namespace             Kustomize namespace
      --kustomize-replica stringArray   Kustomize replicas name (e.g. --kustomize-replica my-deployment --kustomize-replica my-statefulset)
      --kustomize-version               Kustomize version
      --nameprefix                      Kustomize nameprefix
      --namesuffix                      Kustomize namesuffix
  -p, --parameter stringArray           Unset a parameter override (e.g. -p guestbook=image)
      --pass-credentials                Unset passCredentials
      --plugin-env stringArray          Unset plugin env variables (e.g --plugin-env name)
      --ref                             Unset ref on the source
      --source-position int             Position of the source from the list of sources of the app. Counting starts at 1. (default -1)
      --values stringArray              Unset one or more Helm values files
      --values-literal                  Unset literal Helm values block
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

