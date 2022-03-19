## notifications template get

Prints information about configured templates

```
notifications template get [flags]
```

### Examples

```

# prints all templates
notifications template get
# print YAML formatted app-sync-succeeded template definition
notifications template get app-sync-succeeded -o=yaml

```

### Options

```
  -h, --help            help for get
  -o, --output string   Output format. One of:json|yaml|wide|name (default "wide")
```

### Options inherited from parent commands

```
      --argocd-repo-server string       Argo CD repo server address (default "argocd-repo-server:8081")
      --argocd-repo-server-plaintext    Use a plaintext client (non-TLS) to connect to repository server
      --argocd-repo-server-strict-tls   Perform strict validation of TLS certificates when connecting to repo server
      --as string                       Username to impersonate for the operation
      --as-group stringArray            Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                   UID to impersonate for the operation
      --certificate-authority string    Path to a cert file for the certificate authority
      --client-certificate string       Path to a client certificate file for TLS
      --client-key string               Path to a client key file for TLS
      --cluster string                  The name of the kubeconfig cluster to use
      --config-map string               argocd-notifications-cm.yaml file path
      --context string                  The name of the kubeconfig context to use
      --insecure-skip-tls-verify        If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string               Path to a kube config. Only required if out-of-cluster
  -n, --namespace string                If present, the namespace scope for this CLI request
      --password string                 Password for basic authentication to the API server
      --request-timeout string          The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --secret string                   argocd-notifications-secret.yaml file path. Use empty secret if provided value is ':empty'
      --server string                   The address and port of the Kubernetes API server
      --tls-server-name string          If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                    Bearer token for authentication to the API server
      --user string                     The name of the kubeconfig user to use
      --username string                 Username for basic authentication to the API server
```

## notifications template notify

Generates notification using the specified template and send it to specified recipients

```
notifications template notify NAME RESOURCE_NAME [flags]
```

### Examples

```

# Trigger notification using in-cluster config map and secret
notifications template notify app-sync-succeeded guestbook --recipient slack:my-slack-channel

# Render notification render generated notification in console
notifications template notify app-sync-succeeded guestbook

```

### Options

```
  -h, --help                    help for notify
      --recipient stringArray   List of recipients (default [console:stdout])
```

### Options inherited from parent commands

```
      --argocd-repo-server string       Argo CD repo server address (default "argocd-repo-server:8081")
      --argocd-repo-server-plaintext    Use a plaintext client (non-TLS) to connect to repository server
      --argocd-repo-server-strict-tls   Perform strict validation of TLS certificates when connecting to repo server
      --as string                       Username to impersonate for the operation
      --as-group stringArray            Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                   UID to impersonate for the operation
      --certificate-authority string    Path to a cert file for the certificate authority
      --client-certificate string       Path to a client certificate file for TLS
      --client-key string               Path to a client key file for TLS
      --cluster string                  The name of the kubeconfig cluster to use
      --config-map string               argocd-notifications-cm.yaml file path
      --context string                  The name of the kubeconfig context to use
      --insecure-skip-tls-verify        If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string               Path to a kube config. Only required if out-of-cluster
  -n, --namespace string                If present, the namespace scope for this CLI request
      --password string                 Password for basic authentication to the API server
      --request-timeout string          The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --secret string                   argocd-notifications-secret.yaml file path. Use empty secret if provided value is ':empty'
      --server string                   The address and port of the Kubernetes API server
      --tls-server-name string          If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                    Bearer token for authentication to the API server
      --user string                     The name of the kubeconfig user to use
      --username string                 Username for basic authentication to the API server
```

## notifications trigger get

Prints information about configured triggers

```
notifications trigger get [flags]
```

### Examples

```

# prints all triggers
notifications trigger get
# print YAML formatted on-sync-failed trigger definition
notifications trigger get on-sync-failed -o=yaml

```

### Options

```
  -h, --help            help for get
  -o, --output string   Output format. One of:json|yaml|wide|name (default "wide")
```

### Options inherited from parent commands

```
      --argocd-repo-server string       Argo CD repo server address (default "argocd-repo-server:8081")
      --argocd-repo-server-plaintext    Use a plaintext client (non-TLS) to connect to repository server
      --argocd-repo-server-strict-tls   Perform strict validation of TLS certificates when connecting to repo server
      --as string                       Username to impersonate for the operation
      --as-group stringArray            Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                   UID to impersonate for the operation
      --certificate-authority string    Path to a cert file for the certificate authority
      --client-certificate string       Path to a client certificate file for TLS
      --client-key string               Path to a client key file for TLS
      --cluster string                  The name of the kubeconfig cluster to use
      --config-map string               argocd-notifications-cm.yaml file path
      --context string                  The name of the kubeconfig context to use
      --insecure-skip-tls-verify        If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string               Path to a kube config. Only required if out-of-cluster
  -n, --namespace string                If present, the namespace scope for this CLI request
      --password string                 Password for basic authentication to the API server
      --request-timeout string          The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --secret string                   argocd-notifications-secret.yaml file path. Use empty secret if provided value is ':empty'
      --server string                   The address and port of the Kubernetes API server
      --tls-server-name string          If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                    Bearer token for authentication to the API server
      --user string                     The name of the kubeconfig user to use
      --username string                 Username for basic authentication to the API server
```

## notifications trigger run

Evaluates specified trigger condition and prints the result

```
notifications trigger run NAME RESOURCE_NAME [flags]
```

### Examples

```

# Execute trigger configured in 'argocd-notification-cm' ConfigMap
notifications trigger run on-sync-status-unknown ./sample-app.yaml

# Execute trigger using my-config-map.yaml instead of 'argocd-notifications-cm' ConfigMap
notifications trigger run on-sync-status-unknown ./sample-app.yaml \
    --config-map ./my-config-map.yaml
```

### Options

```
  -h, --help   help for run
```

### Options inherited from parent commands

```
      --argocd-repo-server string       Argo CD repo server address (default "argocd-repo-server:8081")
      --argocd-repo-server-plaintext    Use a plaintext client (non-TLS) to connect to repository server
      --argocd-repo-server-strict-tls   Perform strict validation of TLS certificates when connecting to repo server
      --as string                       Username to impersonate for the operation
      --as-group stringArray            Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                   UID to impersonate for the operation
      --certificate-authority string    Path to a cert file for the certificate authority
      --client-certificate string       Path to a client certificate file for TLS
      --client-key string               Path to a client key file for TLS
      --cluster string                  The name of the kubeconfig cluster to use
      --config-map string               argocd-notifications-cm.yaml file path
      --context string                  The name of the kubeconfig context to use
      --insecure-skip-tls-verify        If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string               Path to a kube config. Only required if out-of-cluster
  -n, --namespace string                If present, the namespace scope for this CLI request
      --password string                 Password for basic authentication to the API server
      --request-timeout string          The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --secret string                   argocd-notifications-secret.yaml file path. Use empty secret if provided value is ':empty'
      --server string                   The address and port of the Kubernetes API server
      --tls-server-name string          If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                    Bearer token for authentication to the API server
      --user string                     The name of the kubeconfig user to use
      --username string                 Username for basic authentication to the API server
```

