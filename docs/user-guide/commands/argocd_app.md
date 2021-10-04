## argocd app

Manage applications

```
argocd app [flags]
```

### Examples

```
  # List all the applications.
  argocd app list
  
  # Get the details of a application
  argocd app get my-app
  
  # Set an override parameter
  argocd app set my-app -p image.tag=v1.0.1
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
  -h, --help                           help for app
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to a kube config. Only required if out-of-cluster
  -n, --namespace string               If present, the namespace scope for this CLI request
      --password string                Password for basic authentication to the API server
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --tls-server-name string         If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
      --username string                Username for basic authentication to the API server
```

### Options inherited from parent commands

```
      --auth-token string               Authentication token
      --client-crt string               Client certificate file
      --client-crt-key string           Client certificate key file
      --config string                   Path to Argo CD config (default "/home/user/.argocd/config")
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

* [argocd](argocd.md)	 - argocd controls a Argo CD server
* [argocd app actions](argocd_app_actions.md)	 - Manage Resource actions
* [argocd app create](argocd_app_create.md)	 - Create an application
* [argocd app delete](argocd_app_delete.md)	 - Delete an application
* [argocd app delete-resource](argocd_app_delete-resource.md)	 - Delete resource in an application
* [argocd app diff](argocd_app_diff.md)	 - Perform a diff against the target and live state.
* [argocd app edit](argocd_app_edit.md)	 - Edit application
* [argocd app get](argocd_app_get.md)	 - Get application details
* [argocd app history](argocd_app_history.md)	 - Show application deployment history
* [argocd app list](argocd_app_list.md)	 - List applications
* [argocd app logs](argocd_app_logs.md)	 - Get logs of application pods
* [argocd app manifests](argocd_app_manifests.md)	 - Print manifests of an application
* [argocd app patch](argocd_app_patch.md)	 - Patch application
* [argocd app patch-resource](argocd_app_patch-resource.md)	 - Patch resource in an application
* [argocd app resources](argocd_app_resources.md)	 - List resource of application
* [argocd app rollback](argocd_app_rollback.md)	 - Rollback application to a previous deployed version by History ID, omitted will Rollback to the previous version
* [argocd app set](argocd_app_set.md)	 - Set application parameters
* [argocd app sync](argocd_app_sync.md)	 - Sync an application to its target state
* [argocd app terminate-op](argocd_app_terminate-op.md)	 - Terminate running operation of an application
* [argocd app unset](argocd_app_unset.md)	 - Unset application parameters
* [argocd app wait](argocd_app_wait.md)	 - Wait for an application to reach a synced and healthy state

