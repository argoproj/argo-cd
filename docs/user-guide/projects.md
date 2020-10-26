## Projects

Projects provide a logical grouping of applications, which is useful when Argo CD is used by multiple
teams. Projects provide the following features:

* restrict *what* may be deployed (trusted Git source repositories)
* restrict *where* apps may be deployed to (destination clusters and namespaces)
* restrict what kinds of objects may or may not be deployed (e.g. RBAC, CRDs, DaemonSets, NetworkPolicy etc...)
* defining project roles to provide application RBAC (bound to OIDC groups and/or JWT tokens)

### The Default Project

Every application belongs to a single project. If unspecified, an application belongs to the
`default` project, which is created automatically and by default, permits deployments from any
source repo, to any cluster, and all resource Kinds. The default project can be modified, but not
deleted. When initially created, it's specification is configured to be the most permissive:

```yaml
spec:
  sourceRepos:
  - '*'
  destinations:
  - namespace: '*'
    server: '*'
  clusterResourceWhitelist:
  - group: '*'
    kind: '*'
```

### Creating Projects

Additional projects can be created to give separate teams different levels of access to namespaces.
The following command creates a new project `myproject` which can deploy applications to namespace
`mynamespace` of cluster `https://kubernetes.default.svc`. The permitted Git source repository is
set to `https://github.com/argoproj/argocd-example-apps.git` repository.

```bash
argocd proj create myproject -d https://kubernetes.default.svc,mynamespace -s https://github.com/argoproj/argocd-example-apps.git
```

### Managing Projects

Permitted source Git repositories are managed using commands:

```bash
argocd proj add-source <PROJECT> <REPO>
argocd proj remove-source <PROJECT> <REPO>
```

Permitted destination clusters and namespaces are managed with the commands (for clusters always provide server, the name is not used for matching):

```bash
argocd proj add-destination <PROJECT> <CLUSTER>,<NAMESPACE>
argocd proj remove-destination <PROJECT> <CLUSTER>,<NAMESPACE>
```

Permitted destination K8s resource kinds are managed with the commands. Note that namespaced-scoped
resources are restricted via a blacklist, whereas cluster-scoped resources are restricted via
whitelist.

```bash
argocd proj allow-cluster-resource <PROJECT> <GROUP> <KIND>
argocd proj allow-namespace-resource <PROJECT> <GROUP> <KIND>
argocd proj deny-cluster-resource <PROJECT> <GROUP> <KIND>
argocd proj deny-namespace-resource <PROJECT> <GROUP> <KIND>
```

### Assign Application To A Project

The application project can be changed using `app set` command. In order to change the project of
an app, the user must have permissions to access the new project.

```
argocd app set guestbook-default --project myproject
```

## Project Roles

Projects include a feature called roles that enable automated access to a project's applications.
These can be used to give a CI pipeline a restricted set of permissions. For example, a CI system
may only be able to sync a single app (but not change its source or destination).

Projects can have multiple roles, and those roles can have different access granted to them. These
permissions are called policies, and they are stored within the role as a list of policy strings.
A role's policy can only grant access to that role and are limited to applications within the role's
project.  However, the policies have an option for granting wildcard access to any application
within a project.

In order to create roles in a project and add policies to a role, a user will need permission to
update a project.  The following commands can be used to manage a role.

```bash
argocd proj role list
argocd proj role get
argocd proj role create
argocd proj role delete
argocd proj role add-policy
argocd proj role remove-policy
```

Project roles in itself are not useful without generating a token to associate to that role. Argo CD
supports JWT tokens as the means to authenticate to a role. Since the JWT token is
associated with a role's policies, any changes to the role's policies will immediately take effect
for that JWT token.

The following commands are used to manage the JWT tokens.

```bash
argocd proj role create-token PROJECT ROLE-NAME
argocd proj role delete-token PROJECT ROLE-NAME ISSUED-AT
```

Since the JWT tokens aren't stored in Argo CD, they can only be retrieved when they are created. A
user can leverage them in the cli by either passing them in using the `--auth-token` flag or setting
the ARGOCD_AUTH_TOKEN environment variable. The JWT tokens can be used until they expire or are
revoked.  The JWT tokens can created with or without an expiration, but the default on the cli is
creates them without an expirations date.  Even if a token has not expired, it cannot be used if
the token has been revoked.

Below is an example of leveraging a JWT token to access a guestbook application.  It makes the
assumption that the user already has a project named myproject and an application called
guestbook-default.

```bash
PROJ=myproject
APP=guestbook-default
ROLE=get-role
argocd proj role create $PROJ $ROLE
argocd proj role create-token $PROJ $ROLE -e 10m
JWT=<value from command above>
argocd proj role list $PROJ
argocd proj role get $PROJ $ROLE

# This command will fail because the JWT Token associated with the project role does not have a policy to allow access to the application
argocd app get $APP --auth-token $JWT
# Adding a policy to grant access to the application for the new role
argocd proj role add-policy $PROJ $ROLE --action get --permission allow --object $APP
argocd app get $PROJ-$ROLE --auth-token $JWT

# Removing the policy we added and adding one with a wildcard.
argocd proj role remove-policy $PROJ $TOKEN -a get -o $PROJ-$TOKEN
argocd proj role add-policy $PROJ $ROLE -a get --permission allow -o '*'
# The wildcard allows us to access the application due to the wildcard.
argocd app get $PROJ-$TOKEN --auth-token $JWT
argocd proj role get $PROJ


argocd proj role get $PROJ $ROLE
# Revoking the JWT token
argocd proj role delete-token $PROJ $ROLE <id field from the last command>
# This will fail since the JWT Token was deleted for the project role.
argocd app get $APP --auth-token $JWT
```

## Configuring RBAC With Projects

The project Roles allows configuring RBAC rules scoped to the project. The following sample
project provides read-only permissions on project applications to any member of `my-oidc-group` group.

*AppProject example:*

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: my-project
  namespace: argocd
spec:
  roles:
  # A role which provides read-only access to all applications in the project
  - name: read-only
    description: Read-only privileges to my-project
    policies:
    - p, proj:my-project:read-only, applications, get, my-project/*, allow
    groups:
    - my-oidc-group
```

You can use `argocd proj role` CLI commands or project details page in the user interface to configure the policy.
Note that each project role policy rule must be scoped to that project only. Use the `argocd-rbac-cm` ConfigMap described in
[RBAC](../operator-manual/rbac.md) documentation if you want to configure cross project RBAC rules.

## Configuring Global Projects (v1.8)

Global projects can be configured to provide configurations that other projects can inherit from. 

Projects, which match `matchExpressions` specified in `argocd-cm` ConfigMap, inherit the following fields from the global project:
* namespaceResourceBlacklist
* namespaceResourceWhitelist
* clusterResourceBlacklist
* clusterResourceWhitelist
* SyncWindows

Configure global projects in `argocd-cm` ConfigMap:
```yaml
data:
  globalProjects: |-
    - labelSelector:
        matchExpressions:
          - key: opt
            operator: In
            values:
              - prod
      projectName: proj-global-test
kind: ConfigMap
``` 

Valid operators you can use are: In, NotIn, Exists, DoesNotExist. Gt, and Lt.

projectName: `proj-global-test` should be replaced with your own global project name.
