## argocd proj create

Create a project

```
argocd proj create PROJECT [flags]
```

### Options

```
      --allow-cluster-resource stringArray      List of allowed cluster level resources
      --allow-namespaced-resource stringArray   List of allowed namespaced resources
      --deny-cluster-resource stringArray       List of denied cluster level resources
      --deny-namespaced-resource stringArray    List of denied namespaced resources
      --description string                      Project description
  -d, --dest stringArray                        Permitted destination server and namespace (e.g. https://192.168.99.100:8443,default)
  -f, --file string                             Filename or URL to Kubernetes manifests for the project
  -h, --help                                    help for create
      --orphaned-resources                      Enables orphaned resources monitoring
      --orphaned-resources-warn                 Specifies if applications should have a warning condition when orphaned resources detected
      --signature-keys strings                  GnuPG public key IDs for commit signature verification
  -s, --src stringArray                         Permitted source repository URL
      --upsert                                  Allows to override a project with the same name even if supplied project spec is different from existing spec
```

### Options inherited from parent commands

```
      --as string                       Username to impersonate for the operation
      --as-group stringArray            Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --auth-token string               Authentication token
      --certificate-authority string    Path to a cert file for the certificate authority
      --client-certificate string       Path to a client certificate file for TLS
      --client-crt string               Client certificate file
      --client-crt-key string           Client certificate key file
      --client-key string               Path to a client key file for TLS
      --cluster string                  The name of the kubeconfig cluster to use
      --config string                   Path to Argo CD config (default "/home/user/.argocd/config")
      --context string                  The name of the kubeconfig context to use
      --grpc-web                        Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2.
      --grpc-web-root-path string       Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2. Set web root.
  -H, --header strings                  Sets additional header to all requests made by Argo CD CLI. (Can be repeated multiple times to add multiple headers, also supports comma separated headers)
      --headless                        If set to true then CLI talks directly to Kubernetes instead of talking to Argo CD API server
      --http-retry-max int              Maximum number of retries to establish http connection to Argo CD server
      --insecure                        Skip server certificate and domain verification
      --insecure-skip-tls-verify        If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --logformat string                Set the logging format. One of: text|json (default "text")
      --loglevel string                 Set the logging level. One of: debug|info|warn|error (default "info")
  -n, --namespace string                If present, the namespace scope for this CLI request
      --password string                 Password for basic authentication to the API server
      --plaintext                       Disable TLS
      --port-forward                    Connect to a random argocd-server port using port forwarding
      --port-forward-namespace string   Namespace name which should be used for port forwarding
      --request-timeout string          The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --server string                   The address and port of the Kubernetes API server
      --server-crt string               Server certificate file
      --tls-server-name string          If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                    Bearer token for authentication to the API server
      --user string                     The name of the kubeconfig user to use
      --username string                 Username for basic authentication to the API server
```

### SEE ALSO

* [argocd proj](argocd_proj.md)	 - Manage projects

