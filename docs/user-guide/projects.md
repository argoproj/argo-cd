## Projects

Projects provide a logical grouping of applications, which is useful when Argo CD is used by multiple teams. Projects provide the following features:

* restrict what may be deployed (trusted Git source repositories)
* restrict where apps may be deployed to (destination clusters and namespaces)
* restrict what kinds of objects may or may not be deployed (e.g. RBAC, CRDs, DaemonSets, NetworkPolicy etc...)
* defining project roles to provide application RBAC (bound to OIDC groups and/or JWT tokens)

### The Default Project

Every application belongs to a single project. If unspecified, an application belongs to the `default` project, which is created automatically and by default, permits deployments from any source repo, to any cluster, and all resource Kinds. The default project can be modified, but not deleted. When initially created, it's specification is configured to be the most permissive:

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

Additional projects can be created to give separate teams different levels of access to namespaces. The following command creates a new project `myproject` which can deploy applications to namespace `mynamespace` of cluster `https://kubernetes.default.svc`. The permitted Git source repository is set to `https://github.com/argoproj/argocd-example-apps.git` repository.

```bash
argocd proj create myproject -d https://kubernetes.default.svc,mynamespace -s https://github.com/argoproj/argocd-example-apps.git
```

### Managing Projects

Permitted source Git repositories are managed using commands:

```bash
argocd proj add-source <PROJECT> <REPO>
argocd proj remove-source <PROJECT> <REPO>
```

We can also do negations of sources (i.e. do _not_ use this repo).

```bash
argocd proj add-source <PROJECT> !<REPO>
argocd proj remove-source <PROJECT> !<REPO>
```

Declaratively we can do something like this:

```yaml
spec:
  sourceRepos:
    # Do not use the test repo in argoproj
    - '!ssh://git@GITHUB.com:argoproj/test'
    # Nor any Gitlab repo under group/ 
    - '!https://gitlab.com/group/**'
    # Any other repo is fine though
    - '*'
```

A source repository is considered valid if the following conditions hold:

1. _Any_ allow source rule (i.e. a rule which isn't prefixed with `!`) permits the source
2. AND *no* deny source (i.e. a rule which is prefixed with `!`) rejects the source

Keep in mind that `!*` is an invalid rule, since it doesn't make any sense to disallow everything.

Permitted destination clusters and namespaces are managed with the commands (for clusters always provide server, the name is not used for matching):

```bash
argocd proj add-destination <PROJECT> <CLUSTER>,<NAMESPACE>
argocd proj remove-destination <PROJECT> <CLUSTER>,<NAMESPACE>
```

As with sources, we can also do negations of destinations (i.e. install anywhere _apart from_).

```bash
argocd proj add-destination <PROJECT> !<CLUSTER>,!<NAMESPACE>
argocd proj remove-destination <PROJECT> !<CLUSTER>,!<NAMESPACE>
```

Declaratively we can do something like this:

```yaml
spec:
  destinations:
  # Do not allow any app to be installed in `kube-system`  
  - namespace: '!kube-system'
    server: '*'
  # Or any cluster that has a URL of `team1-*`   
  - namespace: '*'
    server: '!https://team1-*'
    # Any other namespace or server is fine though.
  - namespace: '*'
    server: '*'
```

As with sources, a destination is considered valid if the following conditions hold:

1. _Any_ allow destination rule (i.e. a rule which isn't prefixed with `!`) permits the destination
2. AND *no* deny destination (i.e. a rule which is prefixed with `!`) rejects the destination

Keep in mind that `!*` is an invalid rule, since it doesn't make any sense to disallow everything.

Permitted destination K8s resource kinds are managed with the commands. Note that namespaced-scoped resources are restricted via a deny list, whereas cluster-scoped resources are restricted via allow list.

```bash
argocd proj allow-cluster-resource <PROJECT> <GROUP> <KIND>
argocd proj allow-namespace-resource <PROJECT> <GROUP> <KIND>
argocd proj deny-cluster-resource <PROJECT> <GROUP> <KIND>
argocd proj deny-namespace-resource <PROJECT> <GROUP> <KIND>
```

### Assign Application To A Project

The application project can be changed using `app set` command. In order to change the project of an app, the user must have permissions to access the new project.

```
argocd app set guestbook-default --project myproject
```

## Project Roles

Projects include a feature called roles that can be used to determine who and what can be done applications associated with the project. As an example, it can be used to give a CI pipeline a restricted set of permissions allowing sync operations on a single app (but not change its source or destination).

Projects can have multiple roles, and those roles can have different access granted to them. These permissions are called policies which follows the same [RBAC pattern used in Argo CD configuration](../operator-manual/rbac.md). They are stored within the role as a list of policy strings. A role's policy can only grant access to that role. Users are associated with roles based on the groups list. Consider the hypothetical AppProject definition below:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: sample-test-project
spec:
  ...
  roles:
  - description: some-role
    groups:
    - some-user
    name: admin
    policies:
    - p, proj:sample-test-project:some-role, applications, *, *, allow
  ...
```

Argo CD will use the policies defined in the AppProject roles while authorizing users actions. To determine which role a given users is associated with, it will dynamically create groups based on the role name in runtime. The project definition above will generate the following Casbin RBAC rules:

```
    p, proj:sample-test-project:some-role, applications, *, *, allow
    g, some-user, proj:sample-test-project:some-role
```

_Note 1_: It is very important that policy roles follow the pattern `proj:<project-name>:<role-name>` or they won't be effective during the Argo CD authorization process.

_Note 2_: The example above used `applications` as the resource for the policy definition. However other types of resources can also be used: `repositories`, `clusters`, `logs`, `exec` and `projects`. See the [RBAC documentation](../operator-manual/rbac.md) for more details about those resources.

In order to create roles in a project and add policies to a role, a user will need permission to update a project.  The following commands can be used to manage a role.

```bash
argocd proj role list
argocd proj role get
argocd proj role create
argocd proj role delete
argocd proj role add-policy
argocd proj role remove-policy
```

Project roles in itself are not useful without generating a token to associate to that role. Argo CD supports JWT tokens as the means to authenticate to a role. Since the JWT token is associated with a role's policies, any changes to the role's policies will immediately take effect for that JWT token.

The following commands are used to manage the JWT tokens.

```bash
argocd proj role create-token PROJECT ROLE-NAME
argocd proj role delete-token PROJECT ROLE-NAME ISSUED-AT
```

Since the JWT tokens aren't stored in Argo CD, they can only be retrieved when they are created. A user can leverage them in the cli by either passing them in using the `--auth-token` flag or setting the ARGOCD_AUTH_TOKEN environment variable. The JWT tokens can be used until they expire or are revoked.  The JWT tokens can created with or without an expiration, but the default on the cli is creates them without an expirations date.  Even if a token has not expired, it cannot be used if the token has been revoked.

Below is an example of leveraging a JWT token to access a guestbook application.  It makes the assumption that the user already has a project named myproject and an application called guestbook-default.

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
argocd app get $APP --auth-token $JWT

# Removing the policy we added and adding one with a wildcard.
argocd proj role remove-policy $PROJ $ROLE -a get -o $APP
argocd proj role add-policy $PROJ $ROLE -a get --permission allow -o '*'
# The wildcard allows us to access the application due to the wildcard.
argocd app get $APP --auth-token $JWT
argocd proj role get $PROJ $ROLE


argocd proj role get $PROJ $ROLE
# Revoking the JWT token
argocd proj role delete-token $PROJ $ROLE <id field from the last command>
# This will fail since the JWT Token was deleted for the project role.
argocd app get $APP --auth-token $JWT
```

## Configuring RBAC With Projects

The project Roles allows configuring RBAC rules scoped to the project. The following sample project provides read-only permissions on project applications to any member of `my-oidc-group` group.

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
Note that each project role policy rule must be scoped to that project only. Use the `argocd-rbac-cm` ConfigMap described in [RBAC](../operator-manual/rbac.md) documentation if you want to configure cross project RBAC rules.

## Configuring Global Projects (v1.8)

Global projects can be configured to provide configurations that other projects can inherit from.

Projects, which match `matchExpressions` specified in `argocd-cm` ConfigMap, inherit the following fields from the global project:

* namespaceResourceBlacklist
* namespaceResourceWhitelist
* clusterResourceBlacklist
* clusterResourceWhitelist
* SyncWindows
* SourceRepos
* Destinations

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

## Project scoped Repositories and Clusters

Normally, an Argo CD admin creates a project and decides in advance which clusters and Git repositories it defines. However, this creates a problem in scenarios where a developer wants to add a repository or cluster after the initial creation of the project. This forces the developer to contact their Argo CD admin again to update the project definition.

It is possible to offer a self-service process for developers so that they can add a repository and/or cluster in a project on their own even after the initial creation of the project.

For this purpose Argo CD supports project-scoped repositories and clusters.

To begin the process, Argo CD admins must configure RBAC security to allow this self-service behavior.
For example, to allow users to add project scoped repositories and admin would have to add the following RBAC rules:

```
p, proj:my-project:admin, repositories, create, my-project/*, allow
p, proj:my-project:admin, repositories, delete, my-project/*, allow
p, proj:my-project:admin, repositories, update, my-project/*, allow
```

This provides extra flexibility so that admins can have stricter rules. e.g.:

```
p, proj:my-project:admin, repositories, update, my-project/https://github.example.com/*, allow
```

Once the appropriate RBAC rules are in place, developers can create their own Git repositories and (assuming they have the correct credentials) can add them in an existing project either from the UI or the CLI.
Both the User interface and the CLI have the ability to optionally specify a project. If a project is specified then the respective cluster/repository is considered project scoped:

```argocd repo add --name stable https://charts.helm.sh/stable --type helm --project my-project```

For the declarative setup both repositories and clusters are stored as Kubernetes Secrets, and so a new field is used to denote that this resource is project scoped:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: argocd-example-apps
  labels:
    argocd.argoproj.io/secret-type: repository
type: Opaque
stringData:
  project: my-project1                                     # Project scoped 
  name: argocd-example-apps
  url: https://github.com/argoproj/argocd-example-apps.git
  username: ****
  password: ****
```

!!! warning
Please keep in mind when using a project-scoped repository, only applications from the same project can make use of
it. When using applicationsets with the Git generator, only non-scoped repositories can be used (i.e. repositories that
do _not_ have a `project` set).

All the examples above talk about Git repositories, but the same principles apply to clusters as well.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mycluster-secret
  labels:
    argocd.argoproj.io/secret-type: cluster
type: Opaque
stringData:
  name: mycluster.example.com
  project: my-project1 # Project scoped 
  server: https://mycluster.example.com
  config: |
    {
      "bearerToken": "<authentication token>",
      "tlsClientConfig": {
        "insecure": false,
        "caData": "<base64 encoded certificate>"
      }
    }
```

With project-scoped clusters we can also restrict projects to only allow applications whose destinations belong to the same project. The default behavior allows for applications to be installed onto clusters which are not a part of the same project, as the example below demonstrates:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: "some-ns"
spec:
  destination:
    # This destination might not actually be a cluster which belongs to `foo-project`
    server: https://some-k8s-server/
    namespace: "some-ns"
  project: foo-project
```

To prevent this behavior, we can set the attribute `permitOnlyProjectScopedClusters` on a project.

```yaml
spec:
  permitOnlyProjectScopedClusters: true
```

With this set, the application above would no longer be allowed to be synced to any cluster other than the ones which are a part of the same project.
