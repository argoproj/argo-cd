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

Resources: `clusters`, `projects`, `applications`, `repositories`, `certificates`, `accounts`, `gpgkeys`

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

## Validating and testing your RBAC policies

If you want to ensure that your RBAC policies are working as expected, you can
use the `argocd-util rbac` command to validate them. This tool allows you to
test whether a certain role or subject can perform the requested action with a
policy that's not live yet in the system, i.e. from a local file or config map.
Additionally, it can be used against the live policy in the cluster your Argo
CD is running in.

To check whether your new policy is valid and understood by Argo CD's RBAC
implementation, you can use the `argocd-util rbac validate` command.

### Validating a policy

To validate a policy stored in a local text file:

```shell
argocd-util rbac validate --policy-file somepolicy.csv
```

To validate a policy stored in a local K8s ConfigMap definition in a YAML file:

```shell
argocd-util rbac validate --policy-file argocd-rbac-cm.yaml
```

To validate a policy stored in K8s, used by Argo CD in namespace `argocd`,
ensure that your current context in `~/.kube/config` is pointing to your
Argo CD cluster and give appropriate namespace:

```shell
argocd-util rbac validate --namespace argocd
```

### Testing a policy

To test whether a role or subject (group or local user) has sufficient
permissions to execute certain actions on certain resources, you can
use the `argocd-util rbac can` command. Its general syntax is

```shell
argocd-util rbac can SOMEROLE ACTION RESOURCE SUBRESOURCE [flags]
```

Given the example from the above ConfigMap, which defines the role
`role:org-admin`, and is stored on your local system as `argocd-rbac-cm-yaml`,
you can test whether that role can do something like follows:

```shell
$ argocd-util rbac can role:org-admin get applications --policy-file argocd-rbac-cm.yaml
Yes
$ argocd-util rbac can role:org-admin get clusters --policy-file argocd-rbac-cm.yaml
Yes
$ argocd-util rbac can role:org-admin create clusters 'somecluster' --policy-file argocd-rbac-cm.yaml
No
$ argocd-util rbac can role:org-admin create applications 'someproj/someapp' --policy-file argocd-rbac-cm.yaml
Yes
```

Another example,  given the policy above from `policy.csv`, which defines the
role `role:staging-db-admins` and associates the group `db-admins` with it.
Policy is stored locally as `policy.csv`:

You can test against the role:

```shell
# Plain policy, without a default role defined
$ argocd-util rbac can role:stagin-db-admins get applications --policy-file policy.csv
No
$ argocd-util rbac can role:staging-db-admins get applications 'staging-db-admins/*' --policy-file policy.csv
Yes
# Argo CD augments a builtin policy with two roles defined, the default role
# being 'role:readonly' - You can include a named default role to use:
$ argocd-util rbac can role:stagin-db-admins get applications --policy-file policy.csv --default-role role:readonly
Yes
```

Or against the group defined:

```shell
$ argocd-util rbac can db-admins get applications 'staging-db-admins/*' --policy-file policy.csv
Yes
```
