# Argo CD ssh_known_hosts file customization

The directory contains sample kustomize application which customizes `/etc/ssh/ssh_known_hosts` file in Argo CD. This is useful if you want to disable SSL cert validation
for Git repositories connected using SSL urls:

- `argocd-known-hosts-mounts.yaml` - define merge patches which inject `/etc/ssh/ssh_known_hosts` file mount into all Argo CD deployments.
- `argocd-known-hosts.yaml` - defines `ConfigMap` which includes `/etc/ssh/ssh_known_hosts` file content.
- `kustomization.yaml` - Kustomize application which bundles stable version of Argo CD and apply `argocd-known-hosts-mounts.yaml` patches on top.

!!! note
    The `/etc/ssh/ssh_known_hosts` should include Git host on each Argo CD deployment as well as on a computer where `argocd repo add` is executed. After resolving issue
    [#1514](https://github.com/argoproj/argo-cd/issues/1514) only `argocd-repo-server` deployment has to be customized.

For the known_hosts file to work with custom repository port you have to obtain the public key using `ssh-keyscan` and hash the file before adding it to configmap, i.e.:
```
    ssh-keyscan -p 1234 git.repo.com > known_hosts
    ssh-keygen -Hf known_hosts
    cat known_hosts
```
