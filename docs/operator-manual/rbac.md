# RBAC Configuration

The RBAC feature enables restriction of access to Argo CD resources. Argo CD does not have its own
user management system and has only one built-in user `admin`. The `admin` user is a superuser and
it has unrestricted access to the system. RBAC requires [SSO configuration](user-management/index.md) or [one or more local users setup](user-management/index.md). 
Once SSO or local users are configured, additional RBAC roles can be defined, and SSO groups or local users can man be mapped to roles.

## Basic Built-in Roles

Argo CD has two pre-defined roles but RBAC configuration allows defining roles and groups (see below).

* `role:readonly` - read-only access to all resources
* `role:admin` - unrestricted access to all resources

These default built-in role definitions can be seen in [builtin-policy.csv](https://github.com/argoproj/argo-cd/blob/master/assets/builtin-policy.csv)

### RBAC Permission Structure

Breaking down the permissions definition differs slightly between applications and every other resource type in Argo CD.

* All resources *except* applications permissions (see next bullet):

    `p, <role/user/group>, <resource>, <action>, <object>`

* Applications (which belong to an AppProject):

    `p, <role/user/group>, <resource>, <action>, <appproject>/<object>`

### RBAC Resources and Actions

Resources: `clusters`, `projects`, `applications`, `repositories`, `certificates`

Actions: `get`, `create`, `update`, `delete`, `sync`, `override`, `action`

## Tying It All Together

Additional roles and groups can be configured in `argocd-rbac-cm` ConfigMap. The example below
configures a custom role, named `org-admin`. The role is assigned to any user which belongs to
`your-github-org:your-team` group. All other users get the default policy of `role:readonly`,
which cannot modify Argo CD settings.

*ArgoCD ConfigMap `argocd-rbac-cm` Example:*

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
----

Another `policy.csv` example might look as follows:

```csv
p, role:staging-db-admins, applications, create, staging-db-admins/*, allow
p, role:staging-db-admins, applications, delete, staging-db-admins/*, allow
p, role:staging-db-admins, applications, get, staging-db-admins/*, allow
p, role:staging-db-admins, applications, override, staging-db-admins/*, allow
p, role:staging-db-admins, applications, sync, staging-db-admins/*, allow
p, role:staging-db-admins, applications, update, staging-db-admins/*, allow
p, role:staging-db-admins, projects, get, staging-db-admins, allow
g, db-admins, role:staging-db-admins
```

This example defines a *role* called `staging-db-admins` with *seven permissions* that allow that role to perform the *actions* (`create`/`delete`/`get`/`override`/`sync`/`update` applications, and `get` appprojects) against `*` (all) objects in the `staging-db-admins` Argo CD AppProject.

## Anonymous Access

The anonymous access to Argo CD can be enabled using `users.anonymous.enabled` field in `argocd-cm` (see [argocd-cm.yaml](argocd-cm.yaml)).
The anonymous users get default role permissions specified by `policy.default` in `argocd-rbac-cm.yaml`. For read-only access you'll want `policy.default: role:readonly` as above
