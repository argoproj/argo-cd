## argocd-util

argocd-util tools used by Argo CD

### Synopsis

argocd-util has internal utility tools used by Argo CD

```
argocd-util [flags]
```

### Options

```
  -h, --help               help for argocd-util
      --logformat string   Set the logging format. One of: text|json (default "text")
      --loglevel string    Set the logging level. One of: debug|info|warn|error (default "info")
```

### SEE ALSO

* [argocd-util apps](argocd-util_apps.md)	 - Utility commands operate on ArgoCD applications
* [argocd-util config](argocd-util_config.md)	 - Generate declarative configuration files
* [argocd-util export](argocd-util_export.md)	 - Export all Argo CD data to stdout (default) or a file
* [argocd-util gendexcfg](argocd-util_gendexcfg.md)	 - Generates a dex config from Argo CD settings
* [argocd-util import](argocd-util_import.md)	 - Import Argo CD data from stdin (specify `-') or a file
* [argocd-util kubeconfig](argocd-util_kubeconfig.md)	 - Generates kubeconfig for the specified cluster
* [argocd-util projects](argocd-util_projects.md)	 - Utility commands operate on ArgoCD Projects
* [argocd-util rbac](argocd-util_rbac.md)	 - Validate and test RBAC configuration
* [argocd-util rundex](argocd-util_rundex.md)	 - Runs dex generating a config using settings from the Argo CD configmap and secret
* [argocd-util settings](argocd-util_settings.md)	 - Provides set of commands for settings validation and troubleshooting
* [argocd-util version](argocd-util_version.md)	 - Print version information

