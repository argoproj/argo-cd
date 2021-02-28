# Application state & health

## Sync Status

The *Sync Status* represents the current state of reconciliation between the
*Source* and the *Destination*. The *Sync Status* can take one of the values:

* `SYNCED` - All resources of the *Application* are in the desired state on the
  destination. There is no deviation between the desired and the actual state.

* `OUT OF SYNC` - Argo CD has determined a deviation between the desired state
  and the actual state. When an *Application* transitions to this state, the
  [Automated Sync Policy](../syncing.md)
  (if enabled) will trigger a sync for the *Application*.

* `UNKNOWN` - Argo CD currently cannot determine the desired state from the
  *Application's* source or the actual state on the *Application's* destination.
  This state usually occurs when a non-transient error occurs while comparing
  actual and desired states. Argo CD will also let you know about the error.

Argo CD determines the *Sync Status* by performing a *diff* between the
resources defined by the *Application Source* and the resources that actually
exist in the *Application Destination*.

In some cases, resources on the target cluster get modified by other actors,
such as an operator or a controller, after they have been reconciled into the
target cluster. In such cases, the *Sync Status* would be constantly `OUT OF
SYNC`.

The diffing behaviour can be changed to ignore such expected deviations, so that
they won't affect the *Sync Status*. You can read more about this in the
[Diffing Customization documentation](../../advanced/diffing.md)

## Application Health

The *Application Health* is an aggregate representation of the health of your
*Application's* resources. Whereas the *Sync Status* determines whether all of
the *Application's* resource manifests have been successfully reconciled into
the target Kubernetes cluster, the *Application Health* is an indicator whether
all of the resources also have been succesfully brought into a usable state by
Kubernetes.

The *Application Health* can have one of the following states:

* `HEALTHY` - all of the *Application's* resources *Application* are considered
  healthy

* `PROGRESSING` - at least one of the *Application's* resources is still in the
  process of being brought to a healthy state

* `DEGRADED` - at least one of the *Application's* resources is marked as being
  in an erroneous state or is otherwise unhealthy.

* `UNKNOWN` - the health state of the *Application's* resources could not be
  determined. Argo CD will let you know about the reason for this.

* `MISSING` - the *Application's* resources are missing, and Argo CD cannot
  reliably determine the health status. This usually happens when *Application*
  has not been synced, or when there is an error with the cache.

* `SUSPENDED` - to be written

To illustrate this a little, imagine a `Service` resource in your cluster of
type `LoadBalancer`.

## History
