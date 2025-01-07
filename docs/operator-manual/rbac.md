# RBAC Configuration

The RBAC feature enables restriction of access to Argo CD resources. Argo CD does not have its own
user management system and has only one built-in user `admin`. The `admin` user is a superuser and
it has unrestricted access to the system. RBAC requires [SSO configuration](user-management/index.md) or [one or more local users setup](user-management/index.md).
Once SSO or local users are configured, additional RBAC roles can be defined, and SSO groups or local users can then be mapped to roles.

## Basic Built-in Roles

Argo CD has two pre-defined roles but RBAC configuration allows defining roles and groups (see below).

* `role:readonly` - read-only access to all resources
* `role:admin` - unrestricted access to all resources

These default built-in role definitions can be seen in [builtin-policy.csv](https://github.com/argoproj/argo-cd/blob/master/assets/builtin-policy.csv)

### RBAC Permission Structure

Breaking down the permissions definition differs slightly between applications and every other resource type in Argo CD.

* All resources *except* application-specific permissions (see next bullet):

    `p, <role/user/group>, <resource>, <action>, <object>`

* Applications, applicationsets, logs, and exec (which belong to an `AppProject`):

    `p, <role/user/group>, <resource>, <action>, <appproject>/<object>`

### RBAC Resources and Actions

Resources: `clusters`, `projects`, `applications`, `applicationsets`,
`repositories`, `certificates`, `accounts`, `gpgkeys`, `logs`, `exec`,
`extensions`

Actions: `get`, `create`, `update`, `delete`, `sync`, `override`,`action/<group/kind/action-name>`

Note that `sync`, `override`, and `action/<group/kind/action-name>` only have meaning for the `applications` resource.

#### Application resources

The resource path for application objects is of the form
`<project-name>/<application-name>`.

Delete access to sub-resources of a project, such as a rollout or a pod, cannot
be managed granularly. `<project-name>/<application-name>` grants access to all
subresources of an application.

#### The `action` action

The `action` action corresponds to either built-in resource customizations defined
[in the Argo CD repository](https://github.com/argoproj/argo-cd/tree/master/resource_customizations),
or to [custom resource actions](resource_actions.md#custom-resource-actions) defined by you.
The `action` path is of the form `action/<api-group>/<Kind>/<action-name>`. For
example, a resource customization path
`resource_customizations/extensions/DaemonSet/actions/restart/action.lua`
corresponds to the `action` path `action/extensions/DaemonSet/restart`. You can
also use glob patterns in the action path: `action/*` (or regex patterns if you have
[enabled the `regex` match mode](https://github.com/argoproj/argo-cd/blob/master/docs/operator-manual/argocd-rbac-cm.yaml)).

If the resource is not under a group (for examples, Pods or ConfigMaps), then omit the group name from your RBAC
configuration:

```csv
p, example-user, applications, action//Pod/maintenance-off, default/*, allow
```

#### The `exec` resource

`exec` is a special resource. When enabled with the `create` action, this privilege allows a user to `exec` into Pods via
the Argo CD UI. The functionality is similar to `kubectl exec`.

See [Web-based Terminal](web_based_terminal.md) for more info.

#### The `applicationsets` resource

[ApplicationSets](applicationset/index.md) provide a declarative way to automatically create/update/delete Applications.

Granting `applicationsets, create` effectively grants the ability to create Applications. While it doesn't allow the
user to create Applications directly, they can create Applications via an ApplicationSet.

In v2.5, it is not possible to create an ApplicationSet with a templated Project field (e.g. `project: {{path.basename}}`)
via the API (or, by extension, the CLI). Disallowing templated projects makes project restrictions via RBAC safe:

```csv
p, dev-group, applicationsets, *, dev-project/*, allow
```

With this rule in place, a `dev-group` user will be unable to create an ApplicationSet capable of creating Applications
outside the `dev-project` project.

#### The `extensions` resource

With the `extensions` resource it is possible configure permissions to
invoke [proxy
extensions](../developer-guide/extensions/proxy-extensions.md). The
`extensions` RBAC validation works in conjunction with the
`applications` resource. A user logged in Argo CD (UI or CLI), needs
to have at least read permission on the project, namespace and
application where the request is originated from.

Consider the example below:

```csv
g, ext, role:extension
p, role:extension, applications, get, default/httpbin-app, allow
p, role:extension, extensions, invoke, httpbin, allow
```

Explanation:

* *line1*: defines the group `role:extension` associated with the
  subject `ext`.
* *line2*: defines a policy allowing this role to read (`get`) the
  `httpbin-app` application in the `default` project.
* *line3*: defines another policy allowing this role to `invoke` the
  `httpbin` extension.

**Note 1**: that for extensions requests to be allowed, the policy defined
in the *line2* is also required.

**Note 2**: `invoke` is a new action introduced specifically to be used
with the `extensions` resource. The current actions for `extensions`
are `*` or `invoke`.

## Tying It All Together

Additional roles and groups can be configured in `argocd-rbac-cm` ConfigMap. The example below
configures a custom role, named `org-admin`. The role is assigned to any user which belongs to
`your-github-org:your-team` group. All other users get the default policy of `role:readonly`,
which cannot modify Argo CD settings.

!!! warning
    All authenticated users get *at least* the permissions granted by the default policy. This access cannot be blocked
    by a `deny` rule. Instead, restrict the default policy and then grant permissions to individual roles as needed.

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
    p, role:org-admin, projects, get, *, allow
    p, role:org-admin, projects, create, *, allow
    p, role:org-admin, projects, update, *, allow
    p, role:org-admin, projects, delete, *, allow
    p, role:org-admin, logs, get, *, allow
    p, role:org-admin, exec, create, */*, allow

    g, your-github-org:your-team, role:org-admin
```

----

Another `policy.csv` example might look as follows:

```csv
p, role:staging-db-admin, applications, create, staging-db-project/*, allow
p, role:staging-db-admin, applications, delete, staging-db-project/*, allow
p, role:staging-db-admin, applications, get, staging-db-project/*, allow
p, role:staging-db-admin, applications, override, staging-db-project/*, allow
p, role:staging-db-admin, applications, sync, staging-db-project/*, allow
p, role:staging-db-admin, applications, update, staging-db-project/*, allow
p, role:staging-db-admin, logs, get, staging-db-project/*, allow
p, role:staging-db-admin, exec, create, staging-db-project/*, allow
p, role:staging-db-admin, projects, get, staging-db-project, allow
g, db-admins, role:staging-db-admin
```

This example defines a *role* called `staging-db-admin` with nine *permissions* that allow users with that role to perform the following *actions*:

* `create`, `delete`, `get`, `override`, `sync` and `update` for applications in the `staging-db-project` project,
* `get` logs for objects in the `staging-db-project` project,
* `create` exec for objects in the `staging-db-project` project, and
* `get` for the project named `staging-db-project`.

!!! note
    The `scopes` field controls which OIDC scopes to examine during rbac
    enforcement (in addition to `sub` scope). If omitted, defaults to:
    `'[groups]'`. The scope value can be a string, or a list of strings.

Following example shows targeting `email` as well as `groups` from your OIDC provider.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
  namespace: argocd
  labels:
    app.kubernetes.io/name: argocd-rbac-cm
    app.kubernetes.io/part-of: argocd
data:
  policy.csv: |
    p, my-org:team-alpha, applications, sync, my-project/*, allow
    g, my-org:team-beta, role:admin
    g, user@example.org, role:admin
  policy.default: role:readonly
  scopes: '[groups, email]'
```

For more information on `scopes` please review the [User Management Documentation](user-management/index.md).

## Policy CSV Composition

It is possible to provide additional entries in the `argocd-rbac-cm`
configmap to compose the final policy csv. In this case the key must
follow the pattern `policy.<any string>.csv`. Argo CD will concatenate
all additional policies it finds with this pattern below the main one
('policy.csv'). The order of additional provided policies are
determined by the key string. Example: if two additional policies are
provided with keys `policy.A.csv` and `policy.B.csv`, it will first
concatenate `policy.A.csv` and then `policy.B.csv`.

This is useful to allow composing policies in config management tools
like Kustomize, Helm, etc.

The example below shows how a Kustomize patch can be provided in an
overlay to add additional configuration to an existing RBAC policy.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
  namespace: argocd
data:
  policy.tester-overlay.csv: |
    p, role:tester, applications, *, */*, allow
    p, role:tester, projects, *, *, allow
    g, my-org:team-qa, role:tester
```

## Anonymous Access

The anonymous access to Argo CD can be enabled using `users.anonymous.enabled` field in `argocd-cm` (see [argocd-cm.yaml](argocd-cm.yaml)).
The anonymous users get default role permissions specified by `policy.default` in `argocd-rbac-cm.yaml`. For read-only access you'll want `policy.default: role:readonly` as above

## Validating and testing your RBAC policies

If you want to ensure that your RBAC policies are working as expected, you can
use the `argocd admin settings rbac` command to validate them. This tool allows you to
test whether a certain role or subject can perform the requested action with a
policy that's not live yet in the system, i.e. from a local file or config map.
Additionally, it can be used against the live policy in the cluster your Argo
CD is running in.

To check whether your new policy is valid and understood by Argo CD's RBAC
implementation, you can use the `argocd admin settings rbac validate` command.

### Validating a policy

To validate a policy stored in a local text file:

```shell
argocd admin settings rbac validate --policy-file somepolicy.csv
```

To validate a policy stored in a local K8s ConfigMap definition in a YAML file:

```shell
argocd admin settings rbac validate --policy-file argocd-rbac-cm.yaml
```

To validate a policy stored in K8s, used by Argo CD in namespace `argocd`,
ensure that your current context in `~/.kube/config` is pointing to your
Argo CD cluster and give appropriate namespace:

```shell
argocd admin settings rbac validate --namespace argocd
```

### Testing a policy

To test whether a role or subject (group or local user) has sufficient
permissions to execute certain actions on certain resources, you can
use the `argocd admin settings rbac can` command. Its general syntax is

```shell
argocd admin settings rbac can SOMEROLE ACTION RESOURCE SUBRESOURCE [flags]
```

Given the example from the above ConfigMap, which defines the role
`role:org-admin`, and is stored on your local system as `argocd-rbac-cm-yaml`,
you can test whether that role can do something like follows:

```console
$ argocd admin settings rbac can role:org-admin get applications --policy-file argocd-rbac-cm.yaml
Yes

$ argocd admin settings rbac can role:org-admin get clusters --policy-file argocd-rbac-cm.yaml
Yes

$ argocd admin settings rbac can role:org-admin create clusters 'somecluster' --policy-file argocd-rbac-cm.yaml
No

$ argocd admin settings rbac can role:org-admin create applications 'someproj/someapp' --policy-file argocd-rbac-cm.yaml
Yes
```

Another example,  given the policy above from `policy.csv`, which defines the
role `role:staging-db-admin` and associates the group `db-admins` with it.
Policy is stored locally as `policy.csv`:

You can test against the role:

```console
$ # Plain policy, without a default role defined
$ argocd admin settings rbac can role:staging-db-admin get applications --policy-file policy.csv
No

$ argocd admin settings rbac can role:staging-db-admin get applications 'staging-db-project/*' --policy-file policy.csv
Yes

$ # Argo CD augments a builtin policy with two roles defined, the default role
$ # being 'role:readonly' - You can include a named default role to use:
$ argocd admin settings rbac can role:staging-db-admin get applications --policy-file policy.csv --default-role role:readonly
Yes
```

Or against the group defined:

```console
$ argocd admin settings rbac can db-admins get applications 'staging-db-project/*' --policy-file policy.csv
Yes
```
