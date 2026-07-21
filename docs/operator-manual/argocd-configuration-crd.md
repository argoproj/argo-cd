# ArgoCDConfiguration CRD

Argo CD ships a namespaced CRD, `ArgoCDConfiguration` (`argocdconfigurations.argoproj.io`),
as a typed home for settings that today live in `argocd-cm`, `argocd-cmd-params-cm`, and
`argocd-rbac-cm`.

## Singleton

Only one object is allowed per install namespace, and it **must** be named `argocd-config`
(enforced by a CEL rule on the CRD). Short name: `argocdconfig`.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCDConfiguration
metadata:
  name: argocd-config
  namespace: argocd
spec:
  controller:
    reconciliation:
      timeout: 2m
```

Install the CRD from the install manifests (`manifests/crds/argocdconfiguration-crd.yaml`).

> [!NOTE]
> The singleton CR is **absent by default**. Installs continue to use ConfigMaps / env
> until you create `argocd-config`. When a field is set on the CR, it takes precedence
> over the legacy source for that field.

## What the control plane reads

All six components (application-controller, API server, repo-server, ApplicationSet,
notifications, commit-server) optionally watch `argocd-config` through the config bus
(`util/configbus`). Registry settings that map to CRD fields prefer the CR when present.

Examples:

| CR path | Legacy source |
| --- | --- |
| `spec.controller.reconciliation.timeout` | `argocd-cm` `timeout.reconciliation` |
| `spec.controller.reconciliation.hardTimeout` / `jitter` | matching `timeout.*` keys |
| `spec.controller.resource.*` customizations | `argocd-cm` `resource.customizations*` |
| `spec.server.*`, `spec.repoServer.*`, `spec.applicationSet.*`, … | matching cmd-params / cm keys |

If the CRD is not installed or the informer cannot sync, components continue on the
legacy path and log a warning. Gaps (secrets, some structured blobs, pure bootstrap
flags) are listed in `CONFIGBUS_CRD_GAPS.md` at the repo root.

## Related

- Schema coverage gaps: [CONFIGBUS_CRD_GAPS.md](../../CONFIGBUS_CRD_GAPS.md)
- Go API: `pkg/apis/application/v1alpha1`
