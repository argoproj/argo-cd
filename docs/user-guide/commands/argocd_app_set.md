## argocd app set

Set application parameters

### Synopsis

Set application parameters

```
argocd app set APPNAME [flags]
```

### Options

```
      --allow-empty                               Set allow zero live resources when sync is automated
      --auto-prune                                Set automatic pruning when sync is automated
      --config-management-plugin string           Config management plugin name
      --dest-name string                          K8s cluster Name (e.g. minikube)
      --dest-namespace string                     K8s target namespace (overrides the namespace specified in the ksonnet app.yaml)
      --dest-server string                        K8s cluster URL (e.g. https://kubernetes.default.svc)
      --directory-exclude string                  Set glob expression used to exclude files from application source path
      --directory-recurse                         Recurse directory
      --env string                                Application environment to monitor
      --helm-chart string                         Helm Chart name
      --helm-set stringArray                      Helm set values on the command line (can be repeated to set several values: --helm-set key1=val1 --helm-set key2=val2)
      --helm-set-file stringArray                 Helm set values from respective files specified via the command line (can be repeated to set several values: --helm-set-file key1=path1 --helm-set-file key2=path2)
      --helm-set-string stringArray               Helm set STRING values on the command line (can be repeated to set several values: --helm-set-string key1=val1 --helm-set-string key2=val2)
      --helm-version string                       Helm version
  -h, --help                                      help for set
      --jsonnet-ext-var-code stringArray          Jsonnet ext var
      --jsonnet-ext-var-str stringArray           Jsonnet string ext var
      --jsonnet-libs stringArray                  Additional jsonnet libs (prefixed by repoRoot)
      --jsonnet-tla-code stringArray              Jsonnet top level code arguments
      --jsonnet-tla-str stringArray               Jsonnet top level string arguments
      --kustomize-common-annotation stringArray   Set common labels in Kustomize
      --kustomize-common-label stringArray        Set common labels in Kustomize
      --kustomize-image stringArray               Kustomize images (e.g. --kustomize-image node:8.15.0 --kustomize-image mysql=mariadb,alpine@sha256:24a0c4b4a4c0eb97a1aabb8e29f18e917d05abfe1b7a7c07857230879ce7d3d)
      --kustomize-version string                  Kustomize version
      --nameprefix string                         Kustomize nameprefix
      --namesuffix string                         Kustomize namesuffix
  -p, --parameter stringArray                     set a parameter override (e.g. -p guestbook=image=example/guestbook:latest)
      --path string                               Path in repository to the app directory, ignored if a file is set
      --project string                            Application project name
      --release-name string                       Helm release-name
      --repo string                               Repository URL, ignored if a file is set
      --revision string                           The tracking source branch, tag, commit or Helm chart version the application will sync to
      --revision-history-limit int                How many items to keep in revision history (default 10)
      --self-heal                                 Set self healing when sync is automated
      --sync-option Prune=false                   Add or remove a sync options, e.g add Prune=false. Remove using `!` prefix, e.g. `!Prune=false`
      --sync-policy string                        Set the sync policy (one of: none, automated (aliases of automated: auto, automatic))
      --validate                                  Validation of repo and cluster (default true)
      --values stringArray                        Helm values file(s) to use
      --values-literal-file string                Filename or URL to import as a literal Helm values block
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

* [argocd app](argocd_app.md)	 - Manage applications

