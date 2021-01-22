## argocd proj

Manage projects

```
argocd proj [flags]
```

### Options

```
  -h, --help   help for proj
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
* [argocd proj add-destination](argocd_proj_add-destination.md)	 - Add project destination
* [argocd proj add-orphaned-ignore](argocd_proj_add-orphaned-ignore.md)	 - Add a resource to orphaned ignore list
* [argocd proj add-signature-key](argocd_proj_add-signature-key.md)	 - Add GnuPG signature key to project
* [argocd proj add-source](argocd_proj_add-source.md)	 - Add project source repository
* [argocd proj allow-cluster-resource](argocd_proj_allow-cluster-resource.md)	 - Adds a cluster-scoped API resource to the allow list and removes it from deny list
* [argocd proj allow-namespace-resource](argocd_proj_allow-namespace-resource.md)	 - Removes a namespaced API resource from the deny list or add a namespaced API resource to the allow list
* [argocd proj create](argocd_proj_create.md)	 - Create a project
* [argocd proj delete](argocd_proj_delete.md)	 - Delete project
* [argocd proj deny-cluster-resource](argocd_proj_deny-cluster-resource.md)	 - Removes a cluster-scoped API resource from the allow list and adds it to deny list
* [argocd proj deny-namespace-resource](argocd_proj_deny-namespace-resource.md)	 - Adds a namespaced API resource to the deny list or removes a namespaced API resource from the allow list
* [argocd proj edit](argocd_proj_edit.md)	 - Edit project
* [argocd proj get](argocd_proj_get.md)	 - Get project details
* [argocd proj list](argocd_proj_list.md)	 - List projects
* [argocd proj remove-destination](argocd_proj_remove-destination.md)	 - Remove project destination
* [argocd proj remove-orphaned-ignore](argocd_proj_remove-orphaned-ignore.md)	 - Remove a resource from orphaned ignore list
* [argocd proj remove-signature-key](argocd_proj_remove-signature-key.md)	 - Remove GnuPG signature key from project
* [argocd proj remove-source](argocd_proj_remove-source.md)	 - Remove project source repository
* [argocd proj role](argocd_proj_role.md)	 - Manage a project's roles
* [argocd proj set](argocd_proj_set.md)	 - Set project parameters
* [argocd proj windows](argocd_proj_windows.md)	 - Manage a project's sync windows

