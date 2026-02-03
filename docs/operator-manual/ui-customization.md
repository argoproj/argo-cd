# UI Customization

## Default Theme

You can configure the default theme for the ArgoCD UI by setting the `ui.defaulttheme` key in the [argocd-cm](argocd-cm-yaml.md) ConfigMap.

The following configuration:
```yaml
ui.defaulttheme: "auto"
```

**Behavior:**
- First-time users will see the theme specified in `ui.defaulttheme`
- Existing users will continue to see their previously selected theme (stored in browser localStorage)
- Users can change their theme preference at any time, which will be persisted in their browser

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
