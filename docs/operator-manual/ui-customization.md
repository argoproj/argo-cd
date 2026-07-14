# UI Customization

## Default Application Details View

By default, the Application Details will show the `Tree` view.

This can be configured on an Application basis, by setting the `pref.argocd.argoproj.io/default-view` annotation, accepting one of: `tree`, `pods`, `network`, `list` as values.

For the Pods view, the default grouping mechanism can be configured using the `pref.argocd.argoproj.io/default-pod-sort` annotation, accepting one of: `node`, `parentResource`, `topLevelResource` as values.

## Node Labels in Pod View

It's possible to propagate node labels to node information in the pod view by configuring `application.allowedNodeLabels` in the [argocd-cm](argocd-cm-yaml.md) ConfigMap.

The following configuration:
```yaml
application.allowedNodeLabels: topology.kubernetes.io/zone,karpenter.sh/capacity-type
```
Would result in:
![Node Labels in Pod View](../assets/application-pod-view-node-labels.png)

## CLI Download Links

The **Help** page (linked from the bottom of the sidebar) always shows a download button for the Linux CLI binary that is embedded in the Argo CD server, so users can fetch a working CLI without any configuration.

To offer the CLI for additional operating systems and architectures, set `help.download.<os>-<arch>` keys in the [argocd-cm](argocd-cm-yaml.md) ConfigMap. Each configured key adds a download button on the Help page pointing at the URL you provide:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  help.download.linux-amd64: "https://example.com/argocd-linux-amd64"
  help.download.linux-arm64: "https://example.com/argocd-linux-arm64"
  help.download.darwin-amd64: "https://example.com/argocd-darwin-amd64"
  help.download.darwin-arm64: "https://example.com/argocd-darwin-arm64"
  help.download.windows-amd64: "https://example.com/argocd-windows-amd64.exe"
```

The following `<os>-<arch>` keys are recognized; any other key is ignored:

- `linux-amd64`
- `linux-arm64`
- `linux-ppc64le`
- `linux-s390x`
- `darwin-amd64`
- `darwin-arm64`
- `windows-amd64`

> [!NOTE]
> The default Linux button is always shown in addition to any configured links, so configuring `help.download.linux-<arch>` for the server's own architecture results in two Linux buttons.
