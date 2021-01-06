## argocd-util projects update-role-policy

Implement bulk project role update. Useful to back-fill existing project policies or remove obsolete actions.

```
argocd-util projects update-role-policy PROJECT_GLOB MODIFICATION ACTION [flags]
```

### Examples

```
  # Add policy that allows executing any action (action/*) to roles which name matches to *deployer* in all projects  
  argocd-util projects update-role-policy '*' set 'action/*' --role '*deployer*' --resource applications --scope '*' --permission allow

  # Remove policy that which manages running (action/*) from all roles which name matches *deployer* in all projects
  argocd-util projects update-role-policy '*' remove override --role '*deployer*'

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
      --dry-run                        Dry run (default true)
  -h, --help                           help for update-role-policy
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to a kube config. Only required if out-of-cluster
  -n, --namespace string               If present, the namespace scope for this CLI request
      --password string                Password for basic authentication to the API server
      --permission string              Action permission
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --resource string                Resource e.g. 'applications'
      --role string                    Role name pattern e.g. '*deployer*' (default "*")
      --scope string                   Resource scope e.g. '*'
      --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
      --username string                Username for basic authentication to the API server
```

### SEE ALSO

* [argocd-util projects](argocd-util_projects.md)	 - Utility commands operate on ArgoCD Projects

