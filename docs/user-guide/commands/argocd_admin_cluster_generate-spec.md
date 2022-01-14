## argocd admin cluster generate-spec

Generate declarative config for a cluster

```
argocd admin cluster generate-spec CONTEXT [flags]
```

### Options

```
      --annotation stringArray             Set metadata annotations (e.g. --annotation key=value)
      --aws-cluster-name string            AWS Cluster name if set then aws cli eks token command will be used to access cluster
      --aws-role-arn string                Optional AWS role arn. If set then AWS IAM Authenticator assumes a role to perform cluster operations instead of the default AWS credential provider chain.
      --bearer-token string                Authentication token that should be used to access K8S API server
      --cluster-resources                  Indicates if cluster level resources should be managed. The setting is used only if list of managed namespaces is not empty.
      --exec-command string                Command to run to provide client credentials to the cluster. You may need to build a custom ArgoCD image to ensure the command is available at runtime.
      --exec-command-api-version string    Preferred input version of the ExecInfo for the --exec-command executable
      --exec-command-args stringArray      Arguments to supply to the --exec-command executable
      --exec-command-env stringToString    Environment vars to set when running the --exec-command executable (default [])
      --exec-command-install-hint string   Text shown to the user when the --exec-command executable doesn't seem to be present
      --generate-bearer-token              Generate authentication token that should be used to access K8S API server
  -h, --help                               help for generate-spec
      --in-cluster                         Indicates Argo CD resides inside this cluster and should connect using the internal k8s hostname (kubernetes.default.svc)
      --kubeconfig string                  use a particular kubeconfig file
      --label stringArray                  Set metadata labels (e.g. --label key=value)
      --name string                        Overwrite the cluster name
      --namespace stringArray              List of namespaces which are allowed to manage
  -o, --output string                      Output format. One of: json|yaml (default "yaml")
      --project string                     project of the cluster
      --service-account string             System namespace service account to use for kubernetes resource management. If not set then default "argocd-manager" SA will be used (default "argocd-manager")
      --shard int                          Cluster shard number; inferred from hostname if not set (default -1)
      --system-namespace string            Use different system namespace (default "kube-system")
```

### Options inherited from parent commands

```
      --auth-token string               Authentication token
      --client-crt string               Client certificate file
      --client-crt-key string           Client certificate key file
      --config string                   Path to Argo CD config (default "/home/user/.config/argocd/config")
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

* [argocd admin cluster](argocd_admin_cluster.md)	 - Manage clusters configuration

