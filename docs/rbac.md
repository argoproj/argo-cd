# RBAC

## Overview

The feature RBAC allows restricting access to ArgoCD resources. ArgoCD does not have own user management system and has only one built-in user `admin`. The `admin` user is a
superuser and it has full access. RBAC requires configuring [SSO](./sso.md) integration. Once [SSO](./sso.md) is connected you can define RBAC roles and map roles to groups.

## Configure RBAC

RBAC configuration allows defining roles and groups. ArgoCD has two pre-defined roles: role `role:readonly` which provides read-only access to all resources and role `role:admin`
which provides full access. Role definitions are available in [builtin-policy.csv](../util/rbac/builtin-policy.csv) file.

Additional roles and groups can be configured in `argocd-rbac-cm` ConfigMap. The example below custom role `org-admin`. The role is assigned to any user which belongs to
`your-github-org:your-team` group. All other users get `role:readonly` and cannot modify ArgoCD settings.

*ConfigMap `argocd-rbac-cm` example:*

```yaml
apiVersion: v1
data:
  policy.default: role:readonly
  policy.csv: |
    p, role:org-admin, applications, *, */*, allow
    p, role:org-admin, applications/*, *, */*, allow

    p, role:org-admin, clusters, get, *, allow
    p, role:org-admin, repositories, get, *, allow
    p, role:org-admin, repositories/apps, get, *, allow

    p, role:org-admin, repositories, create, *, allow
    p, role:org-admin, repositories, update, *, allow
    p, role:org-admin, repositories, delete, *, allow

    g, your-github-org:your-team, role:org-admin
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
```

## Configure Projects

Argo projects allow grouping applications which is useful if ArgoCD is used by multiple teams. Additionally, projects restrict source repositories and destination
Kubernetes clusters which can be used by applications belonging to the project.

### 1. Create new project

Following command creates project `myproject` which can deploy applications to namespace `default` of cluster `https://kubernetes.default.svc`. The valid application source is defined in the `https://github.com/argoproj/argocd-example-apps.git` repository.

```
argocd proj create myproject -d https://kubernetes.default.svc,default -s https://github.com/argoproj/argocd-example-apps.git
```

Project sources and destinations can be managed using commands
```
argocd project add-destination
argocd project remove-destination
argocd project add-source
argocd project remove-source
```

### 2. Assign application to a project

Each application belongs to a project. By default, all application belongs to the default project which provides access to any source repo/cluster. The application project can be
changes using `app set` command:

```
argocd app set guestbook-default --project myproject
```

### 3. Update RBAC rules

Following example configure admin access for two teams. Each team has access only two application of one project (`team1` can access `default` project and `team2` can access
`myproject` project).

*ConfigMap `argocd-rbac-cm` example:*

```yaml
apiVersion: v1
data:
  policy.default: ""
  policy.csv: |
    p, role:team1-admin, applications, *, default/*, allow
    p, role:team1-admin, applications/*, *, default/*, allow

    p, role:team1-admin, applications, *, myproject/*, allow
    p, role:team1-admin, applications/*, *, myproject/*, allow

    p, role:org-admin, clusters, get, *, allow
    p, role:org-admin, repositories, get, *, allow
    p, role:org-admin, repositories/apps, get, *, allow

    p, role:org-admin, repositories, create, *, allow
    p, role:org-admin, repositories, update, *, allow
    p, role:org-admin, repositories, delete, *, allow

    g, role:team1-admin, org-admin
    g, role:team2-admin, org-admin
    g, your-github-org:your-team1, role:team1-admin
    g, your-github-org:your-team2, role:team2-admin
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
```
## Project Roles
Projects include a feature called roles that allow users to define access to project's applications.  A project can have multiple roles, and those roles can have different access granted to them.  These permissions are called policies, and they are stored within the role as a list of casbin strings.  A role's policy can only grant access to that role and are limited to applications within the role's project.  However, the policies have an option for granting wildcard access to any application within a project.

In order to create roles in a project and add policies to a role, a user will need permission to update a project.  The following commands can be used to manage a role.
```
argoproj proj role list
argoproj proj role get
argoproj proj role create
argoproj proj role delete
argoproj proj role add-policy
argoproj proj role remove-policy
```

Project roles can not be used unless a user creates a entity that is associated with that project role.  ArgoCD supports creating JWT tokens with a role associated with it.  Since the JWT token is associated with a role's policies, any changes to the role's policies will immediately take effect for that JWT token.

A user will need permission to update a project in order to create a JWT token for a role, and they can use the following commands to manage the JWT tokens. 

```
argoproj proj role create-token
argoproj proj role delete-token
```
Since the JWT tokens aren't stored in ArgoCD, they can only be retrieved when they are created.  A user can leverage them in the cli by either passing them in using the `--auth-token` flag or setting the ARGOCD_AUTH_TOKEN environment variable. The JWT tokens can be used until they expire or are revoked.  The JWT tokens can created with or without an expiration, but the default on the cli is creates them without an expirations date.  Even if a token has not expired, it can not be used if the token has been revoke.

Below is an example of leveraging a JWT token to access the guestbook application.  It makes the assumption that the user already has a project named myproject and an application called guestbook-default.
```
PROJ=myproject
APP=guestbook-default
ROLE=get-role
argocd proj role create $PROJ $ROLE
argocd proj role create-token $PROJ $ROLE -e 10m
JWT=<value from command above>
argocd proj role list $PROJ
argocd proj role get $PROJ $ROLE

#This command will fail because the JWT Token associated with the project role does not have a policy to allow access to the application
argocd app get $APP --auth-token $JWT
# Adding a policy to grant access to the application for the new role
argocd proj role add-policy $PROJ $ROLE --action get --permission allow --object $APP
argocd app get $PROJ-$ROLE --auth-token $JWT

# Removing the policy we added and adding one with a wildcard.  
argocd proj role remove-policy $PROJ $TOKEN -a get -o $PROJ-$TOKEN
argocd proj role remove-policy $PROJ $TOKEN -a get -o '*'
# The wildcard allows us to access the application due to the wildcard.
argocd app get $PROJ-$TOKEN --auth-token $JWT
argocd proj role get $PROJ


argocd proj role get $PROJ $ROLE
# Revoking the JWT token
argocd proj role delete-token $PROJ $ROLE <id field from the last command>
# This will fail since the JWT Token was deleted for the project role.
argocd app get $APP --auth-token $JWT
```

