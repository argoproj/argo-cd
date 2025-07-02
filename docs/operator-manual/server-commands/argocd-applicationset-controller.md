# `argocd-applicationset-controller` Command Reference

## argocd-applicationset-controller

Starts Argo CD ApplicationSet controller

```
argocd-applicationset-controller [flags]
```

### Options

```
      --allowed-scm-providers strings           The list of allowed custom SCM provider API URLs. This restriction does not apply to SCM or PR generators which do not accept a custom API URL. (Default: Empty = all)
      --applicationset-namespaces strings       Argo CD applicationset namespaces
      --argocd-repo-server string               Argo CD repo server address (default "argocd-repo-server:8081")
      --as string                               Username to impersonate for the operation
      --as-group stringArray                    Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                           UID to impersonate for the operation
      --certificate-authority string            Path to a cert file for the certificate authority
      --client-certificate string               Path to a client certificate file for TLS
      --client-key string                       Path to a client key file for TLS
      --cluster string                          The name of the kubeconfig cluster to use
      --concurrent-reconciliations int          Max concurrent reconciliations limit for the controller (default 10)
      --context string                          The name of the kubeconfig context to use
      --debug                                   Print debug logs. Takes precedence over loglevel
      --disable-compression                     If true, opt-out of response compression for all requests to the server
      --dry-run                                 Enable dry run mode
      --enable-github-api-metrics               Enable GitHub API metrics for generators that use the GitHub API
      --enable-leader-election                  Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.
      --enable-new-git-file-globbing            Enable new globbing in Git files generator.
      --enable-policy-override                  For security reason if 'policy' is set, it is not possible to override it at applicationSet level. 'allow-policy-override' allows user to define their own policy (default true)
      --enable-progressive-syncs                Enable use of the experimental progressive syncs feature.
      --enable-scm-providers                    Enable retrieving information from SCM providers, used by the SCM and PR generators (Default: true) (default true)
  -h, --help                                    help for argocd-applicationset-controller
      --insecure-skip-tls-verify                If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string                       Path to a kube config. Only required if out-of-cluster
      --logformat string                        Set the logging format. One of: json|text (default "json")
      --loglevel string                         Set the logging level. One of: debug|info|warn|error (default "info")
      --metrics-addr string                     The address the metric endpoint binds to. (default ":8080")
      --metrics-applicationset-labels strings   List of Application labels that will be added to the argocd_applicationset_labels metric
  -n, --namespace string                        If present, the namespace scope for this CLI request
      --password string                         Password for basic authentication to the API server
      --policy string                           Modify how application is synced between the generator and the cluster. Default is '' (empty), which means AppSets default to 'sync', but they may override that default. Setting an explicit value prevents AppSet-level overrides, unless --allow-policy-override is enabled. Explicit options are: 'sync' (create & update & delete), 'create-only', 'create-update' (no deletion), 'create-delete' (no update)
      --preserved-annotations strings           Sets global preserved field values for annotations
      --preserved-labels strings                Sets global preserved field values for labels
      --probe-addr string                       The address the probe endpoint binds to. (default ":8081")
      --proxy-url string                        If provided, this URL will be used to connect via proxy
      --repo-server-plaintext                   Disable TLS on connections to repo server
      --repo-server-strict-tls                  Whether to use strict validation of the TLS cert presented by the repo server
      --repo-server-timeout-seconds int         Repo server RPC call timeout seconds. (default 60)
      --request-timeout string                  The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --scm-root-ca-path string                 Provide Root CA Path for self-signed TLS Certificates
      --server string                           The address and port of the Kubernetes API server
      --tls-server-name string                  If provided, this name will be used to validate server certificate. If this is not provided, hostname used to contact the server is used.
      --token string                            Bearer token for authentication to the API server
      --token-ref-strict-mode                   Set to true to require secrets referenced by SCM providers to have the argocd.argoproj.io/secret-type=scm-creds label set (Default: false)
      --user string                             The name of the kubeconfig user to use
      --username string                         Username for basic authentication to the API server
      --webhook-addr string                     The address the webhook endpoint binds to. (default ":7000")
      --webhook-parallelism-limit int           Number of webhook requests processed concurrently (default 50)
```

