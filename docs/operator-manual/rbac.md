# RBAC Configuration

The RBAC feature enables restriction of access to Argo CD resources. Argo CD does not have its own
user management system and has only one built-in user `admin`. The `admin` user is a superuser and
it has unrestricted access to the system. RBAC requires [SSO configuration](user-management/index.md) or [one or more local users setup](user-management/index.md).
Once SSO or local users are configured, additional RBAC roles can be defined, and SSO groups or local users can then be mapped to roles.

## Basic Built-in Roles

Argo CD has two pre-defined roles but RBAC configuration allows defining roles and groups (see below).

- `role:readonly` - read-only access to all resources
- `role:admin` - unrestricted access to all resources

These default built-in role definitions can be seen in [builtin-policy.csv](https://github.com/argoproj/argo-cd/blob/master/assets/builtin-policy.csv)

## Default policy for Authenticated Users

When a user is authenticated in Argo CD, it will be granted the role specified in `policy.default`.

!!! warning "Denying Default Permissions"
All authenticated users get _at least_ the permissions granted by the default policy. This access cannot be blocked
by a `deny` rule. It is recommended to create a new `role:authenticated` with the minimum set of permission possible,
then grant permissions to individual roles as needed.

## Anonymous Access

Enabling anonymous access to the Argo CD instance allows users to assume the default role permissions specified by `policy.default` **without being authenticated**.

The anonymous access to Argo CD can be enabled using `users.anonymous.enabled` field in `argocd-cm` (see [argocd-cm.yaml](argocd-cm.yaml)).

!!! warning
When enabling anonymous access, consider creating a new default role, and assign it to the default policy
with `policy.default: role:unauthenticated`.

## RBAC Policy Structure

TODO: exaplain deny
TODO: explain globing
TODO: Explain scopes

There are two different types of policy syntax, one for assiging permissions, and another one to assign users to roles.

- **Group**: Allows to assign authenticated users/groups to internal roles.

  `g, <user/group>, <role>`

  - `<user/group>`: The entity to whom the role will be assigned. It can be a local user or a user authenticated with SSO.
    When SSO is used, the `user` will be based on the `sub` claims, while the group is one of the value returned by the `scopes` configuration.
  - `<role>`: The internal role to which the entity will be assigned.

- **Permission**: Allows to assign permissions to an entity.

  `p, <role/user/group>, <resource>, <action>, <object>, <effect>`

  - `<role/user/group>`: The entity to whom the permission will be assigned
  - `<resource>`: The type of resource on which the action is performed.
  - `<action>`: The operation that is being performed on the resource.
  - `<object>`: The object identifier representing the resource on which the action is performed. Depending on the resource, the object's format will vary.
  - `<effect>`: Whether this permission should grant or restrict the operation on the target object. One of `allow` or `deny`.

Below is a table that summarize all possible resources, and which actions are valid for each of them.

| Resource\Action     | get | create | update | delete | sync | action | override | invoke |
| :------------------ | :-: | :----: | :----: | :----: | :--: | :----: | :------: | :----: |
| **applications**    | ✅  |   ✅   |   ✅   |   ✅   |  ✅  |   ✅   |    ✅    |   ❌   |
| **applicationsets** | ✅  |   ✅   |   ✅   |   ✅   |  ❌  |   ❌   |    ❌    |   ❌   |
| **clusters**        | ✅  |   ✅   |   ✅   |   ✅   |  ❌  |   ❌   |    ❌    |   ❌   |
| **projects**        | ✅  |   ✅   |   ✅   |   ✅   |  ❌  |   ❌   |    ❌    |   ❌   |
| **repositories**    | ✅  |   ✅   |   ✅   |   ✅   |  ❌  |   ❌   |    ❌    |   ❌   |
| **accounts**        | ✅  |   ❌   |   ✅   |   ❌   |  ❌  |   ❌   |    ❌    |   ❌   |
| **certificates**    | ✅  |   ✅   |   ❌   |   ✅   |  ❌  |   ❌   |    ❌    |   ❌   |
| **gpgkeys**         | ✅  |   ✅   |   ❌   |   ✅   |  ❌  |   ❌   |    ❌    |   ❌   |
| **logs**            | ✅  |   ❌   |   ❌   |   ❌   |  ❌  |   ❌   |    ❌    |   ❌   |
| **exec**            | ❌  |   ✅   |   ❌   |   ❌   |  ❌  |   ❌   |    ❌    |   ❌   |
| **extensions**      | ❌  |   ❌   |   ❌   |   ❌   |  ❌  |   ❌   |    ❌    |   ✅   |

### Application-Specific permissions

Some permissions only have meaning within an application. It is the case of the following resources:

- `applications`
- `applicationsets`
- `logs`
- `exec`

While they can be set in the global configuration, they can also be configured in [AppProject's roles](../user-guide/projects.md#project-roles).
The expected `<object>` value in the policy structure is replaced by `<app-project>/<app-name>`.

For instance, these permissions would grant `example-user` access to get any applications,
but only be able to see logs in the `my-app` application part of the `example-project` project.

```csv
p, example-user, applications, get, *, allow
p, example-user, logs, get, example-project/my-app, allow
```

#### Application in any namespaces

When [application in any namespace](app-any-namespace.md) is enabled, the expected `<object>` value in the policy structure is replaced by `<app-project>/<app-ns>/<app-name>`.
Since multiple application could have the same name in the same project, the policy below make sure to restrict the access only to `app-namespace`.

```csv
p, example-user, applications, get, */app-namespace/*, allow
p, example-user, logs, get, example-project/app-namespace/my-app, allow
```

### The `applications` resource

The `applications` resource is an [Application-Specific permission](#application-specific-permissions).

#### Fine-grained permisisons for `update`/`delete` action

The `update` and `delete` actions, when granted on an application, will allow the user to perform the operation on the application itself, **and** all of its resources.
It can be desirable to only allow `update` or `delete` on specific resources within an application.

To do so, when the action if performed on an applciation's resource, the `<action>` will have the `<action>/<group>/<kind>/<ns>/<name>` format.

For instance, to grant access to `example-user` to only delete Pods in the `prod-app`, the policy could be:

```csv
p, example-user, applications, delete/*/Pod/*, default/prod-app, allow
```

If we want to grant the permissions to the user to update all resources of an application, but not the application itself:

```csv
p, example-user, applications, update/*, default/prod-app, allow
```

If we want to explictly deny delete of the application, but allow the user to delete Pods:

```csv
p, example-user, applications, delete, default/prod-app, deny
p, example-user, applications, delete/*/Pod/*, default/prod-app, allow
```

!!! important
It is not possible to deny fine-grained permissions for a sub-resource if the action was **explicitly allowed on the application**.
For instance, the following policy will **allow** a user to delete the Pod and any other resources in the application:

    ```csv
    p, example-user, applications, delete, default/prod-app, allow
    p, example-user, applications, delete/*/Pod/*, default/prod-app, deny
    ```

#### The `action` action

The `action` action corresponds to either built-in resource customizations defined
[in the Argo CD repository](https://github.com/argoproj/argo-cd/tree/master/resource_customizations),
or to [custom resource actions](resource_actions.md#custom-resource-actions) defined by you.

The `<action>` have the `action/<group>/<kind>/<action-name>` format.

For example, a resource customization path `resource_customizations/extensions/DaemonSet/actions/restart/action.lua`
corresponds to the `action` path `action/extensions/DaemonSet/restart`. If the resource is not under a group (for examples, Pods or ConfigMaps),
then the path will be `action//Pod/action-name`.

The following permission allows the user to perform any action on the DaemonSet resources, as well as the `maintenance-off` action on a Pod:

```csv
p, example-user, applications, action//Pod/maintenance-off, default/*, allow
p, example-user, applications, action/extensions/DaemonSet/*, default/*, allow
```

To allow the user to perform any actions:

```csv
p, example-user, applications, action/*, default/*, allow
```

### The `applicationsets` resource

The `applicationsets` resource is an [Application-Specific permission](#application-specific-permissions).
[ApplicationSets](applicationset/index.md) provide a declarative way to automatically create/update/delete Applications.

Allowing the `create` action on the resource effectively grants the ability to create Applications. While it doesn't allow the
user to create Applications directly, they can create Applications via an ApplicationSet.

!!! note
In v2.5, it is not possible to create an ApplicationSet with a templated Project field (e.g. `project: {{path.basename}}`)
via the API (or, by extension, the CLI). Disallowing templated projects makes project restrictions via RBAC safe:

With the resource being application-specifc, the `<object>` of the applicationsets permissions will have the format `<app-project>/<app-name>`.
However, since an ApplicationSet does belong to any project, the `<app-project>` value represents

With the following permission, a `dev-group` user will be unable to create an ApplicationSet capable of creating Applications
outside the `dev-project` project.

```csv
p, dev-group, applicationsets, *, dev-project/*, allow
```

### The `logs` resource

The `logs` resource is an [Application-Specific permission](#application-specific-permissions). When enabled with the `get` action, this permission allows a user to see Pod's logs of an application via
the Argo CD UI. The functionality is similar to `kubectl logs`.

### The `exec` resource

The `exec` resource is an [Application-Specific permission](#application-specific-permissions). When enabled with the `create` action, this permission allows a user to `exec` into Pods of an application via
the Argo CD UI. The functionality is similar to `kubectl exec`.

See [Web-based Terminal](web_based_terminal.md) for more info.

### The `extensions` resource

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

- _line1_: defines the group `role:extension` associated with the
  subject `ext`.
- _line2_: defines a policy allowing this role to read (`get`) the
  `httpbin-app` application in the `default` project.
- _line3_: defines another policy allowing this role to `invoke` the
  `httpbin` extension.

**Note 1**: that for extensions requests to be allowed, the policy defined
in the _line2_ is also required.

**Note 2**: `invoke` is a new action introduced specifically to be used
with the `extensions` resource. The current actions for `extensions`
are `*` or `invoke`.

## Tying It All Together

Additional roles and groups can be configured in `argocd-rbac-cm` ConfigMap. The example below
configures a custom role, named `org-admin`. The role is assigned to any user which belongs to
`your-github-org:your-team` group. All other users get the default policy of `role:readonly`,
which cannot modify Argo CD settings.

_ArgoCD ConfigMap `argocd-rbac-cm` Example:_

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

---

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

This example defines a _role_ called `staging-db-admin` with nine _permissions_ that allow users with that role to perform the following _actions_:

- `create`, `delete`, `get`, `override`, `sync` and `update` for applications in the `staging-db-project` project,
- `get` logs for objects in the `staging-db-project` project,
- `create` exec for objects in the `staging-db-project` project, and
- `get` for the project named `staging-db-project`.

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

## Local Users/Accounts

[Local users](user-management/index.md#local-usersaccounts) are assigned access by either grouping them with a role or by assigning policies directly
to them.

The example below shows how to assign a policy directly to a local user.

```yaml
p, my-local-user, applications, sync, my-project/*, allow
```

This example shows how to assign a role to a local user.

```yaml
g, my-local-user, role:admin
```

!!!warning "Ambiguous Group Assignments"
If you have [enabled SSO](user-management/index.md#sso), any SSO user with a scope that matches a local user will be
added to the same roles as the local user. For example, if local user `sally` is assigned to `role:admin`, and if an
SSO user has a scope which happens to be named `sally`, that SSO user will also be assigned to `role:admin`.

    An example of where this may be a problem is if your SSO provider is an SCM, and org members are automatically
    granted scopes named after the orgs. If a user can create or add themselves to an org in the SCM, they can gain the
    permissions of the local user with the same name.

    To avoid ambiguity, if you are using local users and SSO, it is recommended to assign permissions directly to local
    users, and not to assign roles to local users. In other words, instead of using `g, my-local-user, role:admin`, you
    should explicitly assign permissions to `my-local-user`:

    ```yaml
    p, my-local-user, *, *, *, allow
    ```

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

Another example, given the policy above from `policy.csv`, which defines the
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
