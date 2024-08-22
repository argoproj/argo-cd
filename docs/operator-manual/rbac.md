# RBAC Configuration

The RBAC feature enables restrictions of access to Argo CD resources. Argo CD does not have its own
user management system and has only one built-in user, `admin`. The `admin` user is a superuser and
it has unrestricted access to the system. RBAC requires [SSO configuration](user-management/index.md) or [one or more local users setup](user-management/index.md).
Once SSO or local users are configured, additional RBAC roles can be defined, and SSO groups or local users can then be mapped to roles.

There are two main components where RBAC configuration can be defined:

- The global RBAC config map (see [argo-rbac-cm.yaml](argocd-rbac-cm-yaml.md))
- The [AppProject's roles](../user-guide/projects.md#project-roles)

## Basic Built-in Roles

Argo CD has two pre-defined roles but RBAC configuration allows defining roles and groups (see below).

- `role:readonly`: read-only access to all resources
- `role:admin`: unrestricted access to all resources

These default built-in role definitions can be seen in [builtin-policy.csv](https://github.com/argoproj/argo-cd/blob/master/assets/builtin-policy.csv)

## Default Policy for Authenticated Users

When a user is authenticated in Argo CD, it will be granted the role specified in `policy.default`.

!!! warning "Restricting Default Permissions"

    **All authenticated users get _at least_ the permissions granted by the default policies. This access cannot be blocked
    by a `deny` rule.** It is recommended to create a new `role:authenticated` with the minimum set of permissions possible,
    then grant permissions to individual roles as needed.

## Anonymous Access

Enabling anonymous access to the Argo CD instance allows users to assume the default role permissions specified by `policy.default` **without being authenticated**.

The anonymous access to Argo CD can be enabled using the `users.anonymous.enabled` field in `argocd-cm` (see [argocd-cm.yaml](argocd-cm-yaml.md)).

!!! warning

    When enabling anonymous access, consider creating a new default role and assigning it to the default policies
    with `policy.default: role:unauthenticated`.

## RBAC Model Structure

The model syntax is based on [Casbin](https://casbin.org/docs/overview). There are two different types of syntax: one for assigning policies, and another one for assigning users to internal roles.

**Group**: Allows to assign authenticated users/groups to internal roles.

Syntax: `g, <user/group>, <role>`

- `<user/group>`: The entity to whom the role will be assigned. It can be a local user or a user authenticated with SSO.
  When SSO is used, the `user` will be based on the `sub` claims, while the group is one of the values returned by the `scopes` configuration.
- `<role>`: The internal role to which the entity will be assigned.

**Policy**: Allows to assign permissions to an entity.

Syntax: `p, <role/user/group>, <resource>, <action>, <object>, <effect>`

- `<role/user/group>`: The entity to whom the policy will be assigned
- `<resource>`: The type of resource on which the action is performed.
- `<action>`: The operation that is being performed on the resource.
- `<object>`: The object identifier representing the resource on which the action is performed. Depending on the resource, the object's format will vary.
- `<effect>`: Whether this policy should grant or restrict the operation on the target object. One of `allow` or `deny`.

Below is a table that summarizes all possible resources and which actions are valid for each of them.

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

### Application-Specific Policy

Some policy only have meaning within an application. It is the case with the following resources:

- `applications`
- `applicationsets`
- `logs`
- `exec`

While they can be set in the global configuration, they can also be configured in [AppProject's roles](../user-guide/projects.md#project-roles).
The expected `<object>` value in the policy structure is replaced by `<app-project>/<app-name>`.

For instance, these policies would grant `example-user` access to get any applications,
but only be able to see logs in `my-app` application part of the `example-project` project.

```csv
p, example-user, applications, get, *, allow
p, example-user, logs, get, example-project/my-app, allow
```

#### Application in Any Namespaces

When [application in any namespace](app-any-namespace.md) is enabled, the expected `<object>` value in the policy structure is replaced by `<app-project>/<app-ns>/<app-name>`.
Since multiple applications could have the same name in the same project, the policy below makes sure to restrict access only to `app-namespace`.

```csv
p, example-user, applications, get, */app-namespace/*, allow
p, example-user, logs, get, example-project/app-namespace/my-app, allow
```

### The `applications` resource

The `applications` resource is an [Application-Specific Policy](#application-specific-policy).

#### Fine-grained Permissions for `update`/`delete` action

The `update` and `delete` actions, when granted on an application, will allow the user to perform the operation on the application itself **and** all of its resources.
It can be desirable to only allow `update` or `delete` on specific resources within an application.

To do so, when the action if performed on an application's resource, the `<action>` will have the `<action>/<group>/<kind>/<ns>/<name>` format.

For instance, to grant access to `example-user` to only delete Pods in the `prod-app` Application, the policy could be:

```csv
p, example-user, applications, delete/*/Pod/*, default/prod-app, allow
```

If we want to grant access to the user to update all resources of an application, but not the application itself:

```csv
p, example-user, applications, update/*, default/prod-app, allow
```

If we want to explicitly deny delete of the application, but allow the user to delete Pods:

```csv
p, example-user, applications, delete, default/prod-app, deny
p, example-user, applications, delete/*/Pod/*, default/prod-app, allow
```

!!! note

    It is not possible to deny fine-grained permissions for a sub-resource if the action was **explicitly allowed on the application**.
    For instance, the following policies will **allow** a user to delete the Pod and any other resources in the application:

    ```csv
    p, example-user, applications, delete, default/prod-app, allow
    p, example-user, applications, delete/*/Pod/*, default/prod-app, deny
    ```

#### The `action` action

The `action` action corresponds to either built-in resource customizations defined
[in the Argo CD repository](https://github.com/argoproj/argo-cd/tree/master/resource_customizations),
or to [custom resource actions](resource_actions.md#custom-resource-actions) defined by you.

See the [resource actions documentation](resource_actions.md#built-in-actions) for a list of built-in actions.

The `<action>` has the `action/<group>/<kind>/<action-name>` format.

For example, a resource customization path `resource_customizations/extensions/DaemonSet/actions/restart/action.lua`
corresponds to the `action` path `action/extensions/DaemonSet/restart`. If the resource is not under a group (for example, Pods or ConfigMaps),
then the path will be `action//Pod/action-name`.

The following policies allows the user to perform any action on the DaemonSet resources, as well as the `maintenance-off` action on a Pod:

```csv
p, example-user, applications, action//Pod/maintenance-off, default/*, allow
p, example-user, applications, action/extensions/DaemonSet/*, default/*, allow
```

To allow the user to perform any actions:

```csv
p, example-user, applications, action/*, default/*, allow
```

#### The `override` action

When granted along with the `sync` action, the override action will allow a user to synchronize local manifests to the Application.
These manifests will be used instead of the configured source, until the next sync is performed.

### The `applicationsets` resource

The `applicationsets` resource is an [Application-Specific policy](#application-specific-policy).

[ApplicationSets](applicationset/index.md) provide a declarative way to automatically create/update/delete Applications.

Allowing the `create` action on the resource effectively grants the ability to create Applications. While it doesn't allow the
user to create Applications directly, they can create Applications via an ApplicationSet.

!!! note

    In v2.5, it is not possible to create an ApplicationSet with a templated Project field (e.g. `project: {{path.basename}}`)
    via the API (or, by extension, the CLI). Disallowing templated projects makes project restrictions via RBAC safe:

With the resource being application-specific, the `<object>` of the applicationsets policy will have the format `<app-project>/<app-name>`.
However, since an ApplicationSet does belong to any project, the `<app-project>` value represents the projects in which the ApplicationSet will be able to create Applications.

With the following policy, a `dev-group` user will be unable to create an ApplicationSet capable of creating Applications
outside the `dev-project` project.

```csv
p, dev-group, applicationsets, *, dev-project/*, allow
```

### The `logs` resource

The `logs` resource is an [Application-Specific Policy](#application-specific-policy).

When granted with the `get` action, this policy allows a user to see Pod's logs of an application via
the Argo CD UI. The functionality is similar to `kubectl logs`.

### The `exec` resource

The `exec` resource is an [Application-Specific Policy](#application-specific-policy).

When granted with the `create` action, this policy allows a user to `exec` into Pods of an application via
the Argo CD UI. The functionality is similar to `kubectl exec`.

See [Web-based Terminal](web_based_terminal.md) for more info.

### The `extensions` resource

With the `extensions` resource, it is possible to configure permissions to invoke [proxy extensions](../developer-guide/extensions/proxy-extensions.md).
The `extensions` RBAC validation works in conjunction with the `applications` resource.
A user **needs to have read permission on the application** where the request is originated from.

Consider the example below, it will allow the `example-user` to invoke the `httpbin` extensions in all
applications under the `default` project.

```csv
p, example-user, applications, get, default/*, allow
p, example-user, extensions, invoke, httpbin, allow
```

### The `deny` effect

When `deny` is used as an effect in a policy, it will be effective if the policy matches.
Even if more specific policies with the `allow` effect match as well, the `deny` will have priority.

The order in which the policies appears in the policy file configuration has no impact, and the result is deterministic.

## Policies Evaluation and Matching

The evaluation of access is done in two parts: validating against the default policy configuration, then validating against the policies for the current user.

**If an action is allowed or denied by the default policies, then this effect will be effective without further evaluation**.
When the effect is undefined, the evaluation will continue with subject-specific policies.

The access will be evaluated for the user, then for each configured group that the user is part of.

The matching engine, configured in `policy.matchMode`, can use two different match modes to compare the values of tokens:

- `glob`: based on the [`glob` package](https://pkg.go.dev/github.com/gobwas/glob).
- `regex`: based on the [`regexp` package](https://pkg.go.dev/regexp).

When all tokens match during the evaluation, the effect will be returned. The evaluation will continue until all matching policies are evaluated, or until a policy with the `deny` effect matches.
After all policies are evaluated, if there was at least one `allow` effect and no `deny`, access will be granted.

### Glob matching

When `glob` is used, the policy tokens are treated as single terms, without separators.

Consider the following policy:

```
p, example-user, applications, action/extensions/*, default/*, allow
```

When the `example-user` executes the `extensions/DaemonSet/test` action, the following `glob` matches will happen:

1. The current user `example-user` matches the token `example-user`.
2. The value `applications` matches the token `applications`.
3. The value `action/extensions/DaemonSet/test` matches `action/extensions/*`. Note that `/` is not treated as a separator and the use of `**` is not necessary.
4. The value `default/my-app` matches `default/*`.

## Using SSO Users/Groups

The `scopes` field controls which OIDC scopes to examine during RBAC enforcement (in addition to `sub` scope).
If omitted, it defaults to `'[groups]'`. The scope value can be a string, or a list of strings.

For more information on `scopes` please review the [User Management Documentation](user-management/index.md).

The following example shows targeting `email` as well as `groups` from your OIDC provider.

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

This can be useful to associate users' emails and groups directly in AppProject.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: team-beta-project
  namespace: argocd
spec:
  roles:
    - name: admin
      description: Admin privileges to team-beta
      policies:
        - p, proj:team-beta-project:admin, applications, *, *, allow
      groups:
        - user@example.org # Value from the email scope
        - my-org:team-beta # Value from the groups scope
```

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

!!! warning "Ambiguous Group Assignments"

    If you have [enabled SSO](user-management/index.md#sso), any SSO user with a scope that matches a local user will be
    added to the same roles as the local user. For example, if local user `sally` is assigned to `role:admin`, and if an
    SSO user has a scope which happens to be named `sally`, that SSO user will also be assigned to `role:admin`.

    An example of where this may be a problem is if your SSO provider is an SCM, and org members are automatically
    granted scopes named after the orgs. If a user can create or add themselves to an org in the SCM, they can gain the
    permissions of the local user with the same name.

    To avoid ambiguity, if you are using local users and SSO, it is recommended to assign policies directly to local
    users, and not to assign roles to local users. In other words, instead of using `g, my-local-user, role:admin`, you
    should explicitly assign policies to `my-local-user`:

    ```yaml
    p, my-local-user, *, *, *, allow
    ```

## Policy CSV Composition

It is possible to provide additional entries in the `argocd-rbac-cm` configmap to compose the final policy csv.
In this case, the key must follow the pattern `policy.<any string>.csv`.
Argo CD will concatenate all additional policies it finds with this pattern below the main one ('policy.csv').
The order of additional provided policies are determined by the key string.

Example: if two additional policies are provided with keys `policy.A.csv` and `policy.B.csv`,
it will first concatenate `policy.A.csv` and then `policy.B.csv`.

This is useful to allow composing policies in config management tools like Kustomize, Helm, etc.

The example below shows how a Kustomize patch can be provided in an overlay to add additional configuration to an existing RBAC ConfigMap.

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
use the [`argocd admin settings rbac` command](../user-guide/commands/argocd_admin_settings_rbac.md) to validate them.
This tool allows you to test whether a certain role or subject can perform the requested action with a policy
that's not live yet in the system, i.e. from a local file or config map.
Additionally, it can be used against the live RBAC configuration in the cluster your Argo CD is running in.

### Validating a policy

To check whether your new policy configuration is valid and understood by Argo CD's RBAC implementation,
you can use the [`argocd admin settings rbac validate` command](../user-guide/commands/argocd_admin_settings_rbac_validate.md).

### Testing a policy

To test whether a role or subject (group or local user) has sufficient
permissions to execute certain actions on certain resources, you can
use the [`argocd admin settings rbac can` command](../user-guide/commands/argocd_admin_settings_rbac_can.md).
