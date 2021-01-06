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
  -h, --help   help for app
```

### Options inherited from parent commands

```
      --auth-token string               Authentication token
      --client-crt string               Client certificate file
      --client-crt-key string           Client certificate key file
      --config string                   Path to Argo CD config (default "/home/user/.argocd/config")
      --grpc-web                        Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2.
      --grpc-web-root-path string       Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2. Set web root.
  -H, --header strings                  Sets additional header to all requests made by Argo CD CLI. (Can be repeated multiple times to add multiple headers, also supports comma separated headers)
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
* [argocd app diff](argocd_app_diff.md)	 - Perform a diff against the target and live state.
* [argocd app edit](argocd_app_edit.md)	 - Edit application
* [argocd app get](argocd_app_get.md)	 - Get application details
* [argocd app history](argocd_app_history.md)	 - Show application deployment history
* [argocd app list](argocd_app_list.md)	 - List applications
* [argocd app manifests](argocd_app_manifests.md)	 - Print manifests of an application
* [argocd app patch](argocd_app_patch.md)	 - Patch application
* [argocd app patch-resource](argocd_app_patch-resource.md)	 - Patch resource in an application
* [argocd app resources](argocd_app_resources.md)	 - List resource of application
* [argocd app rollback](argocd_app_rollback.md)	 - Rollback application to a previous deployed version by History ID
* [argocd app set](argocd_app_set.md)	 - Set application parameters
* [argocd app sync](argocd_app_sync.md)	 - Sync an application to its target state
* [argocd app terminate-op](argocd_app_terminate-op.md)	 - Terminate running operation of an application
* [argocd app unset](argocd_app_unset.md)	 - Unset application parameters
* [argocd app wait](argocd_app_wait.md)	 - Wait for an application to reach a synced and healthy state

