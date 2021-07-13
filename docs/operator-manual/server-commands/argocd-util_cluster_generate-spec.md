## argocd-util cluster generate-spec

Generate declarative config for a cluster

```
argocd-util cluster generate-spec CONTEXT [flags]
```

### Options

```
      --aws-cluster-name string            AWS Cluster name if set then aws cli eks token command will be used to access cluster
      --aws-role-arn string                Optional AWS role arn. If set then AWS IAM Authenticator assumes a role to perform cluster operations instead of the default AWS credential provider chain.
      --bearer-token string                Authentication token that should be used to access K8S API server
      --exec-command string                Command to run to provide client credentials to the cluster. You may need to build a custom ArgoCD image to ensure the command is available at runtime.
      --exec-command-api-version string    Preferred input version of the ExecInfo for the --exec-command executable
      --exec-command-args stringArray      Arguments to supply to the --exec-command executable
      --exec-command-env stringToString    Environment vars to set when running the --exec-command executable (default [])
      --exec-command-install-hint string   Text shown to the user when the --exec-command executable doesn't seem to be present
      --generate-bearer-token              Generate authentication token that should be used to access K8S API server
  -h, --help                               help for generate-spec
      --in-cluster                         Indicates Argo CD resides inside this cluster and should connect using the internal k8s hostname (kubernetes.default.svc)
      --kubeconfig string                  use a particular kubeconfig file
      --name string                        Overwrite the cluster name
      --namespace stringArray              List of namespaces which are allowed to manage
  -o, --output string                      Output format. One of: json|yaml (default "yaml")
      --service-account string             System namespace service account to use for kubernetes resource management. If not set then default "argocd-manager" SA will be used (default "argocd-manager")
      --shard int                          Cluster shard number; inferred from hostname if not set (default -1)
      --system-namespace string            Use different system namespace (default "kube-system")
```

### SEE ALSO

* [argocd-util cluster](argocd-util_cluster.md)	 - Manage clusters configuration

