# Microsoft

!!! note "Are you using this? Please contribute!"
    If you're using this IdP please consider [contributing](../../developer-guide/site.md) to this document.

<!-- markdownlint-disable MD033 -->
<div style="text-align:center"><img src="../../../assets/argo.png" /></div>
<!-- markdownlint-enable MD033 -->

<!-- markdownlint-disable MD046 -->
```yaml
ConfigMap -> argocd-cm

data:
    dex.config: |
      connectors:
      - type: microsoft
        id: microsoft
        name: Your Company GmbH
        config:
          clientID: $MICROSOFT_APPLICATION_ID
          clientSecret: $MICROSOFT_CLIENT_SECRET
          redirectURI: http://localhost:8080/api/dex/callback
          tenant: ffffffff-ffff-ffff-ffff-ffffffffffff
          groups: 
            - DevOps
```

https://github.com/dexidp/dex/blob/master/Documentation/connectors/microsoft.md#groups

```yaml
ConfigMap -> argocd-rbac-cm

data:
    policy.csv: |
        p, role:org-admin, applications, *, */*, allow
        p, role:org-admin, clusters, get, *, allow
        p, role:org-admin, repositories, get, *, allow
        p, role:org-admin, repositories, create, *, allow
        p, role:org-admin, repositories, update, *, allow
        p, role:org-admin, repositories, delete, *, allow

        g, DevOps, role:org-admin
    
    policy.default: role:readonly
```
