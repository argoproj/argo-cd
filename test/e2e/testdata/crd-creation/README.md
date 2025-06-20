```
argocd app create crd-creation \
  --repo https://github.com/argoproj/argo-cd.git \
  --path test/e2e/functional/crd-creation \
  --dest-server https://kubernetes.default.svc \
  --dest-namespace default
```