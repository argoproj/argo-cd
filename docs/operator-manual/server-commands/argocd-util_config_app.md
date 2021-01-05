## argocd-util config app

Generate declarative config for an application

### Synopsis

Generate declarative config for an application

```
argocd-util config app APPNAME [flags]
```

### Examples

```

	# Generate declarative config for a directory app
	argocd-util config app guestbook --repo https://github.com/argoproj/argocd-example-apps.git --path guestbook --dest-namespace default --dest-server https://kubernetes.default.svc --directory-recurse

	# Generate declarative config for a Jsonnet app
	argocd-util config app jsonnet-guestbook --repo https://github.com/argoproj/argocd-example-apps.git --path jsonnet-guestbook --dest-namespace default --dest-server https://kubernetes.default.svc --jsonnet-ext-str replicas=2

	# Generate declarative config for a Helm app
	argocd-util config app helm-guestbook --repo https://github.com/argoproj/argocd-example-apps.git --path helm-guestbook --dest-namespace default --dest-server https://kubernetes.default.svc --helm-set replicaCount=2

	# Generate declarative config for a Helm app from a Helm repo
	argocd-util config app nginx-ingress --repo https://kubernetes-charts.storage.googleapis.com --helm-chart nginx-ingress --revision 1.24.3 --dest-namespace default --dest-server https://kubernetes.default.svc

	# Generate declarative config for a Kustomize app
	argocd-util config app kustomize-guestbook --repo https://github.com/argoproj/argocd-example-apps.git --path kustomize-guestbook --dest-namespace default --dest-server https://kubernetes.default.svc --kustomize-image gcr.io/heptio-images/ks-guestbook-demo:0.1

	# Generate declarative config for a app using a custom tool:
	argocd-util config app ksane --repo https://github.com/argoproj/argocd-example-apps.git --path plugins/kasane --dest-namespace default --dest-server https://kubernetes.default.svc --config-management-plugin kasane

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
      --directory-include string                  Set glob expression used to include files from application source path
      --directory-recurse                         Recurse directory
      --env string                                Application environment to monitor
  -f, --file string                               Filename or URL to Kubernetes manifests for the app
      --helm-chart string                         Helm Chart name
      --helm-set stringArray                      Helm set values on the command line (can be repeated to set several values: --helm-set key1=val1 --helm-set key2=val2)
      --helm-set-file stringArray                 Helm set values from respective files specified via the command line (can be repeated to set several values: --helm-set-file key1=path1 --helm-set-file key2=path2)
      --helm-set-string stringArray               Helm set STRING values on the command line (can be repeated to set several values: --helm-set-string key1=val1 --helm-set-string key2=val2)
      --helm-version string                       Helm version
  -h, --help                                      help for app
      --jsonnet-ext-var-code stringArray          Jsonnet ext var
      --jsonnet-ext-var-str stringArray           Jsonnet string ext var
      --jsonnet-libs stringArray                  Additional jsonnet libs (prefixed by repoRoot)
      --jsonnet-tla-code stringArray              Jsonnet top level code arguments
      --jsonnet-tla-str stringArray               Jsonnet top level string arguments
      --kustomize-common-annotation stringArray   Set common labels in Kustomize
      --kustomize-common-label stringArray        Set common labels in Kustomize
      --kustomize-image stringArray               Kustomize images (e.g. --kustomize-image node:8.15.0 --kustomize-image mysql=mariadb,alpine@sha256:24a0c4b4a4c0eb97a1aabb8e29f18e917d05abfe1b7a7c07857230879ce7d3d)
      --kustomize-version string                  Kustomize version
  -l, --label stringArray                         Labels to apply to the app
      --name string                               A name for the app, ignored if a file is set (DEPRECATED)
      --nameprefix string                         Kustomize nameprefix
      --namesuffix string                         Kustomize namesuffix
  -o, --output string                             Output format. One of: json|yaml (default "yaml")
  -p, --parameter stringArray                     set a parameter override (e.g. -p guestbook=image=example/guestbook:latest)
      --path string                               Path in repository to the app directory, ignored if a file is set
      --plugin-env stringArray                    Additional plugin envs
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

### SEE ALSO

* [argocd-util config](argocd-util_config.md)	 - Generate declarative configuration files

