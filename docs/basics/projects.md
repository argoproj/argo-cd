# Projects

## What is a Project (AppProject)?

The so-called *Projects* (or, *AppProject* alternatively) play a vital role in
the multi-tenancy and governance model of Argo CD. It is important to understand
how *Projects* work and how they impact *Applications* and permissions.

You can think of a *Project* as a way to group specific *Applications* together
to enforce a common set of governance rules and settings on those Applications,
with the settings being defined in the *Project*. For example, you can restrict
the kind of resources allowed in an *Application*, or restrict the *Application*
to source its manifests only from a certain repository, etc etc. Furthermore,
projects can issue *access tokens* scoped to applications within the given
project. These tokens can be used to access the Argo CD API for manipulation
of *Applications* associated with the project, and their permissions can be
configured using *Project* specific RBAC configuration.

*Projects* and Applications have a *1:n* relationship, that is, multiple
*Applications* can belong to the same *Project*, while each *Application* can
only belong to one *Project*. Furthermore, the association of an *Application*
to a *Project* is mandatory. It is not possible to have an *Application* that
is not associated to a *Project*.

An Argo CD *Project* is implemented as a Custom Resource `AppProject` in the
`argoproj.io/v1alpha1` API. 

All `AppProject` resources must exist in Argo CD's installation namespace
(`argocd` by default) in the cluster Argo CD is installed to in order to be
used by Argo CD. They cannot be installed in other clusters or namespaces.

!!! tip "The default project"
    Argo CD installs a default *Project* which permits everything and restricts
    nothing. The default *Project* is called, well, `default`.

## Project level settings

A project can enforce restrictions on the following entities:

### Destinations

A *Project* can define allowed *destinations* for any of the *Applications*
associated with the *Project*. A *Project's* destination restriction is a
tuple of a target cluster and a namespace, with wildcard pattern matching
supported.

|Cluster|Namespace|Outcome|
|-|-|-|
|*|*|Allow all namespaces in all clusters|
|https://kubernetes.default.svc|*|Allow all namespaces in the local cluster (`kubernetes.default.svc`)|
|https://prod-*|*|Allow all namespaces in target clusters with URL starting with `https://prod-*`|
|*|dev-apps-*|Allow namespaces starting with `dev-apps-*` in all target clusters|

When an *Application's* destination or one of the *Application's* resources
with a hardcoded target namespace do not match an allowed destination of the
*Project*, any sync operation will not be allowed.

### Sources

A *Project* can define allowed *sources* for any of the *Applications*
associated with the *Project*. A *Project's* allowed sources is a list of one
or more URL patterns that must match an *Application's* source repository.

The corresponding CLI commands for adding or removing constraints on
project sources are:

* `argocd proj add-source` - adds a source
* `argocd proj remove-source` - removes a source

### Cluster resources

A *Project* must define what kind of cluster-scoped resources *Applications* are
allowed to deploy. If an *Application's* resources contain any cluster-scoped
resources not allowed by the *Project*, any sync operation will not be allowed.

Allowance of cluster-scoped resources is evaluated from two lists:

* A positive-list to allow specific resources
* A negative-list to deny specific resources

In order to decide if a resource is allowed, it is first matched against the
positive list. If it matches the positive-list, and is not found in the
negative-list, the resource is allowed. If it doesn't match the positive-list,
or is matched in the negative-list, the resource - and therefore the sync
operation - is not allowed.

Each list is using tuples of Kubernetes' API `Group` and `Kind` to match the
resources of the *Application* against. Wildcard patterns are supported. Each
resource **must** match against the positive-list, and **must not** match
against the negative-list.

The following table shows matching for a cluster-wide resource of Group/Kind
`rbac.authorization.k8s.io/ClusterRole` (the dash `-` means, not configured)

|Positive Group| Positive Kind|Negative Group|Negative Kind|Allowed|
|-|-|-|-|-|
|`*`|`*`|-|-|Yes|
|`*`|`*`|`*`|`*`|No|
|`rbac*`|`*`|-|-|Yes|
|`*`|`*`|`rbac.authorization.k8s.io`|`ClusterRoleBinding`|Yes|

A newly created *Project* without further configuration will forbid all
cluster-scoped resources to be managed. The `default` *Project* allows all
cluster-scoped resources to be managed.

The corresponding CLI commands for adding or removing constraints on
cluster-scoped resources are:

* `argocd proj allow-cluster-resource` - adds a cluster-scoped resource to the
  positive-list
* `argocd proj deny-cluster-resource` - adds a cluster-scoped resource to the
  negative-list

### Namespaced resources

A *Project* must define what kind of namespace-scoped resources *Applications*
are allowed to deploy. If an *Application's* resources contain any
namespace-scoped resources not allowed by the *Project*, any sync operation will
not be allowed.

The decision tree for whether allowing a namespaced resource for deployment is
the same as for
[cluster scoped resources](#cluster-resources).

A newly created *Project* without further configuration will forbid all
namespaced-scoped resources to be managed. The `default` *Project* allows all
namespaced-scoped resources to be managed.

The corresponding CLI commands for adding or removing constraints on namespaced
resources are:

* `argocd proj allow-namespace-resource` - adds a namespace-scoped resource to the
  positive-list
* `argocd proj deny-namespace-resource` - adds a namespace-scoped resource to the
  negative-list

!!! tip "Resources in the core API group"
    If you need to add resources from the *Core* API group, i.e. *Secret* or
    *ConfigMap* resources, use the empty string `''` as API group.

### GnuPG keys used for signature verification

An advanced feature of Argo CD is to only allow syncs from Git revisions that
are signed using GnuPG (e.g. commited using `git commit -S`). You can read more
about this feature in its
[documentation](../advanced/gnupg.md).

You can configure the GnuPG key IDs that commits need to be signed by for all
applications belonging to a certain project. Once at least one key ID is added,
signature verification will be enforced and any sync operation to a non-signed
revision, or a revision that is signed with a GnuPG key not in the allow-list
will be denied.

The corresponding CLI commands for adding and removing GnuPG key IDs are:

* `argocd proj add-signature-key`
* `argocd proj remove-signature-key`

By default, GnuPG commit verification is disabled.

### Sync windows

A *Project* can define time windows that determine when an *Application* is
allowed to be synced to a cluster. You can read more about this feature in the
[Sync Windows documentation](../advanced/sync_windows.md).

By default, a *Project* does not restrict syncing to any time windows and the
sync is allowed at all times.

To manage sync windows, you can use the `argocd proj windows` command.

## Project roles

### Access tokens

### Project specific RBAC rules

## Managing projects

Projects can be managed in three distinct ways:

* using the `argocd` CLI,
* using the web UI,
* using the Kubernetes API

### Using the argocd CLI

To create a new Project using the `argocd` CLI, you can use the
`argocd proj create` command. In its most simple form, this command creates
a new project, that allows nothing and restricts everything (which is not very
useful, obviously). For example, the following creates a new Project named
`restrictive`:

```bash
argocd proj create restrictive
```

You can verify this project has been created by using `argocd proj get`
command:

```bash
$ argocd proj get restrictive
Name:                        restrictive
Description:                 
Destinations:                <none>
Repositories:                <none>
Allowed Cluster Resources:   <none>
Denied Namespaced Resources: <none>
Signature keys:              <none>
Orphaned Resources:          disabled
```

### Using the web UI

You can find the Projects overview page by going to *Settings* -> *Projects*:

![Projects overview screen](/assets/screens/projects-01-where.png)

### Using Kubernetes API

*Projects* are implemented as Custom Resource of Kind `AppProject` in the
`argoproj.io` API group.

You can directly create, edit or remove *Project* resources in the cluster
where Argo CD is installed to.

*Project* resources live in Argo CD's installation namespace, which is `argocd`
by default.

To list all configured *Projects*, use `kubectl get`:

```bash
$ kubectl -n argocd get appprojects.argoproj.io
NAME          AGE
default       161d
```

To show more information about a given *Project*, use `kubectl describe`:

```bash
$ kubectl -n argocd describe appprojects.argoproj.io default
Name:         default
Namespace:    argocd
Labels:       <none>
Annotations:  <none>
API Version:  argoproj.io/v1alpha1
Kind:         AppProject
Metadata:
  Creation Timestamp:  2020-07-25T13:39:29Z
  Generation:          1
  Managed Fields:
    API Version:  argoproj.io/v1alpha1
    Fields Type:  FieldsV1
    fieldsV1:
      f:spec:
        .:
        f:clusterResourceWhitelist:
        f:destinations:
        f:sourceRepos:
      f:status:
    Manager:         argocd-server
    Operation:       Update
    Time:            2020-07-25T13:39:29Z
  Resource Version:  7013321
  Self Link:         /apis/argoproj.io/v1alpha1/namespaces/argocd/appprojects/default
  UID:               b06f6dcd-bd80-4119-a721-fa7619a5387f
Spec:
  Cluster Resource Whitelist:
    Group:  *
    Kind:   *
  Destinations:
    Namespace:  *
    Server:     *
  Source Repos:
    *
Status:
Events:  <none>
```

You can find more information in the
[AppProject CRD reference documentation](../reference/crd/appproject.md).