## argocd-util config proj

Generate declarative config for a project

```
argocd-util config proj PROJECT [flags]
```

### Options

```
      --description string        Project description
  -d, --dest stringArray          Permitted destination server and namespace (e.g. https://192.168.99.100:8443,default)
  -f, --file string               Filename or URL to Kubernetes manifests for the project
  -h, --help                      help for proj
      --orphaned-resources        Enables orphaned resources monitoring
      --orphaned-resources-warn   Specifies if applications should be a warning condition when orphaned resources detected
  -o, --output string             Output format. One of: json|yaml (default "yaml")
      --signature-keys strings    GnuPG public key IDs for commit signature verification
  -s, --src stringArray           Permitted source repository URL
```

### SEE ALSO

* [argocd-util config](argocd-util_config.md)	 - Generate declarative configuration files

