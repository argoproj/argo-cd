## argocd-util rbac can

Check RBAC permissions for a role or subject

### Synopsis


Check whether a given role or subject has appropriate RBAC permissions to do
something.


```
argocd-util rbac can ROLE/SUBJECT ACTION RESOURCE [SUB-RESOURCE] [flags]
```

### Examples

```

# Check whether role some:role has permissions to create an application in the
# 'default' project, using a local policy.csv file
argocd-util rbac can some:role create application 'default/app' --policy-file policy.csv

# Policy file can also be K8s config map with data keys like argocd-rbac-cm,
# i.e. 'policy.csv' and (optionally) 'policy.default'
argocd-util rbac can some:role create application 'default/app' --policy-file argocd-rbac-cm.yaml

# If --policy-file is not given, the ConfigMap 'argocd-rbac-cm' from K8s is
# used. You need to specify the argocd namespace, and make sure that your
# current Kubernetes context is pointing to the cluster Argo CD is running in
argocd-util rbac can some:role create application 'default/app' --namespace argocd

# You can override a possibly configured default role
argocd-util rbac can someuser create application 'default/app' --default-role role:readonly


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
      --default-role string            name of the default role to use
  -h, --help                           help for can
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to a kube config. Only required if out-of-cluster
  -n, --namespace string               If present, the namespace scope for this CLI request
      --password string                Password for basic authentication to the API server
      --policy-file string             path to the policy file to use
  -q, --quiet                          quiet mode - do not print results to stdout
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --server string                  The address and port of the Kubernetes API server
      --strict                         whether to perform strict check on action and resource names (default true)
      --tls-server-name string         If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                   Bearer token for authentication to the API server
      --use-builtin-policy             whether to also use builtin-policy (default true)
      --user string                    The name of the kubeconfig user to use
      --username string                Username for basic authentication to the API server
```

### SEE ALSO

* [argocd-util rbac](argocd-util_rbac.md)	 - Validate and test RBAC configuration

