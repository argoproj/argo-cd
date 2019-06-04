# Disaster Recovery

You can use `argocd-util` to import and export all Argo CD data.

To get this, you'll need to build it yourself.

```bash
git clone https://github.com/argoproj/argo-cd.git
cd argo-cd
make argocd-util
./dist/argocd-util version ;# check it works
```

Export to a backup:

```bash
./dist/argocd-util export -o backup.yaml
```

Import from a backup:

```bash
./dist/argocd-util import backup.yaml
```

