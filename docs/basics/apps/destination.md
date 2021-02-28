# Application destination

The *Application destination* defines where the *Application* should be synced
to. The *Destination* is defined in the `.spec.destination` part of the
*Application* Custom Resource.

A *Destination* consists of a tuple of the *target cluster* and the target
*namespace*.

A *Destination* must be permitted in the *Application's* parent
[Project](../projects/).

## Target cluster

The *target cluster*, as the name implies, defines the cluster where the
application's resource manifests should be deployed to. The target cluster is
specified using the `spec.destination.server` field, which contains either the
URL to the Kubernetes API of the cluster, or its
[symbolic name](../clusters/).

There are two distinct types of values you can use here:

* Either the local cluster where Argo CD is installed to, which is usually
  `https://kubernetes.default.svc` with a symbolic name of `in-cluster`, or

* A remote cluster, referenced by its API URL. Be aware that before you can
  specify a remote cluster as a target cluster, it needs to be
  [added to Argo CD's configuration properly](../clusters/).

## Target namespace

Depending on your Argo CD
[installation type](../../getting_started/install.md#installation-types),
your
[target cluster's configuration](../clusters/)
and your
[project settings](../projects/#cluster-resources),
your *Application* resource manifests may consist of cluster-scoped and
namespace-scoped resources.

Cluster-scoped resources obviously won't need a target namespace, but Argo CD
needs to know to which target namespace the namespace-scoped resources shall
be deployed to. This is set via the `.spec.destination.namespace` field.

The target namespace has to exist in the target cluster unless the
[sync option](../../syncing/)
[namespace auto-creation](../../syncing/)
has been set in the *Application* or an appropriate `Namespace` resource is part
of the *Application's* resource manifests.

Argo CD will not overwrite existing namespace configuration in any resource,
so the final decision about a resource's target namespace will be made according
to these rules:

* If a resource has set `.metadata.namespace`, its value will be used as the
  target namespace for that resource. In this case, the namespace has either to
  exist in the target cluster, or an appropriate `Namespace` resource has to
  be delivered together with the application's resource manifests.

* Otherwise, the *target namespace* as defined in the *Application's*
  `.spec.destination.namespace` field will be used as the target namespace for
  the resource.
