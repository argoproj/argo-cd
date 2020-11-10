## argocd-util settings validate

Validate settings

### Synopsis

Validates settings specified in 'argocd-cm' ConfigMap and 'argocd-secret' Secret

```
argocd-util settings validate [flags]
```

### Examples

```

#Validates all settings in the specified YAML file
argocd-util settings validate --argocd-cm-path ./argocd-cm.yaml

#Validates accounts and plugins settings in Kubernetes cluster of current kubeconfig context
argocd-util settings validate --group accounts --group plugins --load-cluster-settings
```

### Options

```
      --group stringArray   Optional list of setting groups that have to be validated ( one of: accounts, general, kustomize, plugins, repositories, resource-overrides)
  -h, --help                help for validate
```

### Options inherited from parent commands

```
      --argocd-cm-path string          Path to local argocd-cm.yaml file
      --argocd-secret-path string      Path to local argocd-secret.yaml file
      --as string                      Username to impersonate for the operation
      --as-group stringArray           Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --certificate-authority string   Path to a cert file for the certificate authority
      --client-certificate string      Path to a client certificate file for TLS
      --client-key string              Path to a client key file for TLS
      --cluster string                 The name of the kubeconfig cluster to use
      --context string                 The name of the kubeconfig context to use
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to a kube config. Only required if out-of-cluster
      --load-cluster-settings          Indicates that config map and secret should be loaded from cluster unless local file path is provided
  -n, --namespace string               If present, the namespace scope for this CLI request
      --password string                Password for basic authentication to the API server
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
      --username string                Username for basic authentication to the API server
```

### SEE ALSO

* [argocd-util settings](argocd-util_settings.md)	 - Provides set of commands for settings validation and troubleshooting

