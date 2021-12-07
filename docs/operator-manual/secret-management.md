# Secret Management

Argo CD is un-opinionated about how secrets are managed. There's many ways to do it and there's no one-size-fits-all solution. Here's some ways people are doing GitOps secrets:

* [Bitnami Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets)
* [GoDaddy Kubernetes External Secrets](https://github.com/godaddy/kubernetes-external-secrets)
* [External Secrets Operator](https://github.com/external-secrets/external-secrets)
* [Hashicorp Vault](https://www.vaultproject.io)
* [Banzai Cloud Bank-Vaults](https://github.com/banzaicloud/bank-vaults)
* [Helm Secrets](https://github.com/jkroepke/helm-secrets)
* [Kustomize secret generator plugins](https://github.com/kubernetes-sigs/kustomize/blob/fd7a353df6cece4629b8e8ad56b71e30636f38fc/examples/kvSourceGoPlugin.md#secret-values-from-anywhere)
* [aws-secret-operator](https://github.com/mumoshu/aws-secret-operator)
* [KSOPS](https://github.com/viaduct-ai/kustomize-sops#argo-cd-integration)
* [argocd-vault-plugin](https://github.com/argoproj-labs/argocd-vault-plugin)
* [argocd-vault-replacer](https://github.com/crumbhole/argocd-vault-replacer)

For discussion, see [#1364](https://github.com/argoproj/argo-cd/issues/1364)
