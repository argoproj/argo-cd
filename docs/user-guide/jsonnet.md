# Jsonnet

Any file matching `*.jsonnet` in a directory app is treated as a Jsonnet file.

## Build Environment

> v1.4

Jsonnet apps have access to the [standard build environment](build-environment.md) via substitution into *TLAs* and *external variables*.

E.g. via the CLI:

```bash
argocd app create APPNAME \
  --jsonnet-ext-str 'app=${ARGOCD_APP_NAME}' \
  --jsonnet-tla-str 'ns=${ARGOCD_APP_NAMESPACE}'
```

Or by declarative syntax:

```yaml
  directory:
    jsonnet:
      extVars:
      - name: app
        value: $ARGOCD_APP_NAME
      tlas:
        - name: ns
          value: $ARGOCD_APP_NAMESPACE
```