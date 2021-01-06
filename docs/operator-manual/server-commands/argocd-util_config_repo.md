## argocd-util config repo

Generate declarative config for a repo

```
argocd-util config repo REPOURL [flags]
```

### Examples

```
  
  # Add a Git repository via SSH using a private key for authentication, ignoring the server's host key:
  argocd-util config repo git@git.example.com:repos/repo --insecure-ignore-host-key --ssh-private-key-path ~/id_rsa

  # Add a Git repository via SSH on a non-default port - need to use ssh:// style URLs here
  argocd-util config repo ssh://git@git.example.com:2222/repos/repo --ssh-private-key-path ~/id_rsa

  # Add a private Git repository via HTTPS using username/password and TLS client certificates:
  argocd-util config repo https://git.example.com/repos/repo --username git --password secret --tls-client-cert-path ~/mycert.crt --tls-client-cert-key-path ~/mycert.key

  # Add a private Git repository via HTTPS using username/password without verifying the server's TLS certificate
  argocd-util config repo https://git.example.com/repos/repo --username git --password secret --insecure-skip-server-verification

  # Add a public Helm repository named 'stable' via HTTPS
  argocd-util config repo https://kubernetes-charts.storage.googleapis.com --type helm --name stable  

  # Add a private Helm repository named 'stable' via HTTPS
  argocd-util config repo https://kubernetes-charts.storage.googleapis.com --type helm --name stable --username test --password test

  # Add a private Helm OCI-based repository named 'stable' via HTTPS
  argocd-util config repo helm-oci-registry.cn-zhangjiakou.cr.aliyuncs.com --type helm --name stable --enable-oci --username test --password test

```

### Options

```
      --enable-lfs                          enable git-lfs (Large File Support) on this repository
      --enable-oci                          enable helm-oci (Helm OCI-Based Repository)
  -h, --help                                help for repo
      --insecure-ignore-host-key            disables SSH strict host key checking (deprecated, use --insecure-skip-server-verification instead)
      --insecure-skip-server-verification   disables server certificate and host key checks
      --name string                         name of the repository, mandatory for repositories of type helm
  -o, --output string                       Output format. One of: json|yaml (default "yaml")
      --password string                     password to the repository
      --ssh-private-key-path string         path to the private ssh key (e.g. ~/.ssh/id_rsa)
      --tls-client-cert-key-path string     path to the TLS client cert's key path (must be PEM format)
      --tls-client-cert-path string         path to the TLS client cert (must be PEM format)
      --type string                         type of the repository, "git" or "helm" (default "git")
      --username string                     username to the repository
```

### SEE ALSO

* [argocd-util config](argocd-util_config.md)	 - Generate declarative configuration files

