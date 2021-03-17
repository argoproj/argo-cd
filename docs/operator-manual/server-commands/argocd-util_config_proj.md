## argocd-util config proj

Generate declarative config for a project

```
argocd-util config proj PROJECT [flags]
```

### Options

```
      --description string        Project description
  -d, --dest stringArray          Permitted destination server and namespace (e.g. https://192.168.99.100:8443,default)
  -h, --help                      help for proj
      --unmanaged-resources        Enables unmanaged resources monitoring
      --unmanaged-resources-warn   Specifies if applications should have a warning condition when unmanaged resources detected
  -o, --output string             Output format. One of: json|yaml (default "yaml")
      --signature-keys strings    GnuPG public key IDs for commit signature verification
  -s, --src stringArray           Permitted source repository URL
```

### SEE ALSO

* [argocd-util config](argocd-util_config.md)	 - Generate declarative configuration files

