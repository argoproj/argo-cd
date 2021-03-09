## argocd-util proj generate-spec

Generate declarative config for a project

```
argocd-util proj generate-spec PROJECT [flags]
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
  -h, --help                                    help for generate-spec
      --orphaned-resources                      Enables orphaned resources monitoring
      --orphaned-resources-warn                 Specifies if applications should have a warning condition when orphaned resources detected
  -o, --output string                           Output format. One of: json|yaml (default "yaml")
      --signature-keys strings                  GnuPG public key IDs for commit signature verification
  -s, --src stringArray                         Permitted source repository URL
```

### SEE ALSO

* [argocd-util proj](argocd-util_proj.md)	 - Manage projects configuration

