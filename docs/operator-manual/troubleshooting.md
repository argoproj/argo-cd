# Troubleshooting Tools

The document describes how to use `argocd-tool` binary to simplify Argo CD settings customizations and troubleshot
connectivity issues.

## Settings

Argo CD provides multiple ways to customize system behavior and has a lot of settings. It might be dangerous to modify
settings on Argo CD used in production by multiple users. Before applying settings you can use `argocd-util` binary to
make sure that settings are valid and Argo CD is working as expected. The `argocd-util` binary is available in `argocd`
image and might be used using docker. Example:

```bash
docker run --rm -it -w /src -v $(pwd):/src argoproj/argocd:<version> \
  argocd-util settings validate --argocd-cm-path ./argocd-cm.yaml
```

If you are using Linux you can extract `argocd-util` binary from docker image:

```bash
docker run --rm -it -w /src -v $(pwd):/src argocd cp /usr/local/bin/argocd-util ./argocd-util
``` 

The `argocd-util settings validate` command performs basic settings validation and print short summary
of each settings group.

**Diffing Customization**

[Diffing customization](../user-guide/diffing.md) allows excluding some resource fields from diffing process.
The diffing customizations are configured in `resource.customizations` field of `argocd-cm` ConfigMap.

The following `argocd-util` command prints information about fields excluded from diffing in the specified ConfigMap.

```bash
docker run --rm -it -w /src -v $(pwd):/src argoproj/argocd:<version> \
  argocd-util settings resource-overrides ignore-differences ./deploy.yaml --argocd-cm-path ./argocd-cm.yaml
```

* Health Assessment

[Health assessment](../user-guide/diffing.md) allows excluding some resource fields from diffing process.
The diffing customizations are configured in `resource.customizations` field of `argocd-cm` ConfigMap. 

The following `argocd-util` command assess resource health using Lua script configured in the specified ConfigMap.

```bash
docker run --rm -it -w /src -v $(pwd):/src argoproj/argocd:<version> \
  argocd-util settings resource-overrides health ./deploy.yaml --argocd-cm-path ./argocd-cm.yaml
```

* Resource Actions

Resource actions allows configuring named Lua script which performs resource modification.

The following `argocd-util` command executes action using Lua script configured in the specified ConfigMap and prints
applied modifications.

```bash
docker run --rm -it -w /src -v $(pwd):/src argoproj/argocd:<version> \
  argocd-util settings resource-overrides run-action /tmp/deploy.yaml restart --argocd-cm-path /private/tmp/argocd-cm.yaml 
```

The following `argocd-util` command lists actions available for a given resource using Lua script configured in the specified ConfigMap.

```bash
docker run --rm -it -w /src -v $(pwd):/src argoproj/argocd:<version> \
  argocd-util settings resource-overrides list-actions /tmp/deploy.yaml --argocd-cm-path /private/tmp/argocd-cm.yaml 
```

## Cluster credentials

The `argocd-util kubeconfig` is useful if you manually created Secret with cluster credentials and trying need to
troubleshoot connectivity issues. In this case, it is suggested to use the following steps:

1 SSH into [argocd-application-controller] pod.

```
kubectl exec -n argocd -it \
  $(kubectl get pods -n argocd -l app.kubernetes.io/name=argocd-application-controller -o jsonpath='{.items[0].metadata.name}') bash
```

2 Use `argocd-util kubeconfig` command to export kubeconfig file from the configured Secret: 

```
argocd-util kubeconfig https://<api-server-url> /tmp/kubeconfig --namespace argocd
```

3 Use `kubectl` to get more details about connection issues, fix them and apply changes back to secret:

```
export KUBECONFIG=/tmp/kubeconfig
kubectl get pods -v 9
``` 