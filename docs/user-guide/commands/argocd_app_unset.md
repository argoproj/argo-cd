## argocd app unset

Unset application parameters

```
argocd app unset APPNAME parameters [flags]
```

### Examples

```
  # Unset kustomize override kustomize image
  argocd app unset my-app --kustomize-image=alpine

  # Unset kustomize override prefix
  argocd app unset my-app --namesuffix

  # Unset parameter override
  argocd app unset my-app -p COMPONENT=PARAM
```

### Options

```
  -h, --help                          help for unset
      --ignore-missing-value-files    Unset the helm ignore-missing-value-files option (revert to false)
      --kustomize-image stringArray   Kustomize images name (e.g. --kustomize-image node --kustomize-image mysql)
      --kustomize-version             Kustomize version
      --nameprefix                    Kustomize nameprefix
      --namesuffix                    Kustomize namesuffix
  -p, --parameter stringArray         Unset a parameter override (e.g. -p guestbook=image)
      --plugin-env stringArray        Unset plugin env variables (e.g --plugin-env name)
      --values stringArray            Unset one or more Helm values files
      --values-literal                Unset literal Helm values block
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

