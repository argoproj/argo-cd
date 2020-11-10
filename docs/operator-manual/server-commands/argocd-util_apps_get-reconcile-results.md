## argocd-util apps get-reconcile-results

Reconcile all applications and stores reconciliation summary in the specified file.

### Synopsis

Reconcile all applications and stores reconciliation summary in the specified file.

```
argocd-util apps get-reconcile-results PATH [flags]
```

### Options

```
      --as string                      Username to impersonate for the operation
      --as-group stringArray           Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --certificate-authority string   Path to a cert file for the certificate authority
      --client-certificate string      Path to a client certificate file for TLS
      --client-key string              Path to a client key file for TLS
      --cluster string                 The name of the kubeconfig cluster to use
      --context string                 The name of the kubeconfig context to use
  -h, --help                           help for get-reconcile-results
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to a kube config. Only required if out-of-cluster
      --l string                       Label selector
  -n, --namespace string               If present, the namespace scope for this CLI request
      --o string                       Output format (yaml|json) (default "yaml")
      --password string                Password for basic authentication to the API server
      --refresh                        If set to true then recalculates apps reconciliation
      --repo-server string             Repo server address.
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
      --username string                Username for basic authentication to the API server
```

### SEE ALSO

* [argocd-util apps](argocd-util_apps.md)	 - Utility commands operate on ArgoCD applications

