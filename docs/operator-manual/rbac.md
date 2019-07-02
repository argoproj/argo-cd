# RBAC

## Overview

The RBAC feature enables restriction of access to Argo CD resources. Argo CD does not have its own
user management system and has only one built-in user `admin`. The `admin` user is a superuser and
it has unrestricted access to the system. RBAC requires [SSO configuration](./sso.md). Once SSO is
configured, additional RBAC roles can be defined, and SSO groups can man be mapped to roles.

## Configure RBAC

RBAC configuration allows defining roles and groups. Argo CD has two pre-defined roles:

* `role:readonly` - read-only access to all resources
* `role:admin` - unrestricted access to all resources

These role definitions can be seen in [builtin-policy.csv](https://github.com/argoproj/argo-cd/blob/master/assets/builtin-policy.csv)

Additional roles and groups can be configured in `argocd-rbac-cm` ConfigMap. The example below
configures a custom role, named `org-admin`. The role is assigned to any user which belongs to
`your-github-org:your-team` group. All other users get the default policy of `role:readonly`,
which cannot modify Argo CD settings.

*ConfigMap `argocd-rbac-cm` example:*

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
  namespace: argocd
data:
  policy.default: role:readonly
  policy.csv: |
    p, role:org-admin, applications, *, */*, allow
    p, role:org-admin, clusters, get, *, allow
    p, role:org-admin, repositories, get, *, allow
    p, role:org-admin, repositories, create, *, allow
    p, role:org-admin, repositories, update, *, allow
    p, role:org-admin, repositories, delete, *, allow

    g, your-github-org:your-team, role:org-admin
```

## Anonymous Access

THe anonymous access to Argo CD can be enabled using `users.anonymous.enabled` field in `argocd-cm` (see [./argocd-cm.yaml](argocd-cm.yaml)).
The anonymous users get default role permissions specified argocd-rbac-cm.yaml.