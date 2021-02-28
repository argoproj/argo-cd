# Project level settings

A project can enforce restrictions on the following entities:

## Destinations

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

## Sources

A *Project* can define allowed *sources* for any of the *Applications*
associated with the *Project*. A *Project's* allowed sources is a list of one
or more URL patterns that must match an *Application's* source repository.

The corresponding CLI commands for adding or removing constraints on
project sources are:

* `argocd proj add-source` - adds a source
* `argocd proj remove-source` - removes a source

## Cluster resources

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

## Namespaced resources

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

## GnuPG keys used for signature verification

An advanced feature of Argo CD is to only allow syncs from Git revisions that
are signed using GnuPG (e.g. commited using `git commit -S`). You can read more
about this feature in its
[documentation](/advanced/gnupg.md).

You can configure the GnuPG key IDs that commits need to be signed by for all
applications belonging to a certain project. Once at least one key ID is added,
signature verification will be enforced and any sync operation to a non-signed
revision, or a revision that is signed with a GnuPG key not in the allow-list
will be denied.

The corresponding CLI commands for adding and removing GnuPG key IDs are:

* `argocd proj add-signature-key`
* `argocd proj remove-signature-key`

By default, GnuPG commit verification is disabled.

## Sync windows

A *Project* can define time windows that determine when an *Application* is
allowed to be synced to a cluster. You can read more about this feature in the
[Sync Windows documentation](/advanced/sync_windows.md).

By default, a *Project* does not restrict syncing to any time windows and the
sync is allowed at all times.

To manage sync windows, you can use the `argocd proj windows` command.
