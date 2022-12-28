## argocd admin settings validate

Validate settings

### Synopsis

Validates settings specified in 'argocd-cm' ConfigMap and 'argocd-secret' Secret

```
argocd admin settings validate [flags]
```

### Examples

```

#Validates all settings in the specified YAML file
argocd admin settings validate --argocd-cm-path ./argocd-cm.yaml

#Validates accounts and plugins settings in Kubernetes cluster of current kubeconfig context
argocd admin settings validate --group accounts --group plugins --load-cluster-settings
```

### Options

```
      --group stringArray   Optional list of setting groups that have to be validated ( one of: accounts, general, kustomize, plugins, repositories, resource-overrides)
  -h, --help                help for validate
```

### Options inherited from parent commands

```
      --argocd-cm-path string           Path to local argocd-cm.yaml file
      --argocd-secret-path string       Path to local argocd-secret.yaml file
      --as string                       Username to impersonate for the operation
      --as-group stringArray            Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                   UID to impersonate for the operation
      --auth-token string               Authentication token
      --certificate-authority string    Path to a cert file for the certificate authority
      --client-certificate string       Path to a client certificate file for TLS
      --client-crt string               Client certificate file
      --client-crt-key string           Client certificate key file
      --client-key string               Path to a client key file for TLS
      --cluster string                  The name of the kubeconfig cluster to use
      --config string                   Path to Argo CD config (default "/home/user/.config/argocd/config")
      --context string                  The name of the kubeconfig context to use
      --core                            If set to true then CLI talks directly to Kubernetes instead of talking to Argo CD API server
      --grpc-web                        Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2.
      --grpc-web-root-path string       Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2. Set web root.
  -H, --header strings                  Sets additional header to all requests made by Argo CD CLI. (Can be repeated multiple times to add multiple headers, also supports comma separated headers)
      --http-retry-max int              Maximum number of retries to establish http connection to Argo CD server
      --insecure                        Skip server certificate and domain verification
      --insecure-skip-tls-verify        If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kube-context string             Directs the command to the given kube-context
      --kubeconfig string               Path to a kube config. Only required if out-of-cluster
      --load-cluster-settings           Indicates that config map and secret should be loaded from cluster unless local file path is provided
      --logformat string                Set the logging format. One of: text|json (default "text")
      --loglevel string                 Set the logging level. One of: debug|info|warn|error (default "info")
  -n, --namespace string                If present, the namespace scope for this CLI request
      --password string                 Password for basic authentication to the API server
      --plaintext                       Disable TLS
      --port-forward                    Connect to a random argocd-server port using port forwarding
      --port-forward-namespace string   Namespace name which should be used for port forwarding
      --proxy-url string                If provided, this URL will be used to connect via proxy
      --request-timeout string          The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --server string                   The address and port of the Kubernetes API server
      --server-crt string               Server certificate file
      --tls-server-name string          If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                    Bearer token for authentication to the API server
      --user string                     The name of the kubeconfig user to use
      --username string                 Username for basic authentication to the API server
```

### SEE ALSO

* [argocd admin settings](argocd_admin_settings.md)	 - Provides set of commands for settings validation and troubleshooting

