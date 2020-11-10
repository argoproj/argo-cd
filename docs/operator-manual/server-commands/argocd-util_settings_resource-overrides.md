## argocd-util settings resource-overrides

Troubleshoot resource overrides

### Synopsis

Troubleshoot resource overrides

```
argocd-util settings resource-overrides [flags]
```

### Options

```
  -h, --help   help for resource-overrides
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
* [argocd-util settings resource-overrides health](argocd-util_settings_resource-overrides_health.md)	 - Assess resource health
* [argocd-util settings resource-overrides ignore-differences](argocd-util_settings_resource-overrides_ignore-differences.md)	 - Renders fields excluded from diffing
* [argocd-util settings resource-overrides list-actions](argocd-util_settings_resource-overrides_list-actions.md)	 - List available resource actions
* [argocd-util settings resource-overrides run-action](argocd-util_settings_resource-overrides_run-action.md)	 - Executes resource action

