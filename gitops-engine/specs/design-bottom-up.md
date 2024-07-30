# GitOps Engine Design - Bottom Up [On Hold - see [Top Down](./design-top-down.md)]

## Summary

The Bottom Up approach assumes that teams identify similar components in Argo CD and Flux and merge them one by one. The following five components were identified so far:

* Access to Git repositories
* Kubernetes resource cache
* Manifest Generation
* Resources reconciliation
* Sync Planning

The rest of the document describes separate proposals of how exactly components could be merged.

## Kubernetes Resource Cache

Both Argo CD and Flux have to load target cluster resources. This is required to enable the following main use cases:
- Collect resources that are no longer in Git and delete/warn the user.
- Present information about cluster state to the user: Argo CD UI, fluxctl `list-images`, `list-workloads` commands.
- Compare the state of the cluster with the configuration in Git.

Projects use different approaches to collect cluster state information. Argo CD leverages Kubernetes watch APIs to maintain
lightweight cluster state cache. Flux fetches required resources when information is needed.

The problem is that Kubernetes does not provide an SQL like API which allows to effectively find required resources and in
some cases, Flux has to load whole cluster/namespace state into memory and go through the in-memory resources list. This is
a time and memory consuming approach, which also puts pressure on Kubernetes' API server.

### Goals

Extract Argo CD caching logic into a reusable component that maintains a lightweight cluster state cache. argoproj/argo-cd/controller/cache

### Non-Goals
Support multi-cluster caching. The ability to maintain a cache of multiple-cluster is implemented in Argo CD code but it is tightly coupled
to how Argo CD stores cluster credentials and add too much complexity.

### Proposal.

The cluster cache component encapsulates interaction with Kubernetes APIs and allows to quickly inspect Kubernetes resources in a thread-safe
manner. The component is responsible for the following tasks:

- Identify resource APIs supported by the target cluster and provide  API’s metadata (e.g. if API is namespaced or cluster scope).
- Notifying about changes in the resource APIs supported by the target cluster (e.g. added CRDs, removed CRDs ...).
- Loads initial state and watch for changes in every supported resource API.
- Handles available changes  APIs:  start/stops watches; removes obsolete APIs from the cache.

The component does not cache the whole resource manifest because it would require too much memory.  Instead, it stores only 
resource identifiers and relationships between resources.  The whole resource manifest or any other resource metadata should
be cached by the component user using event handlers.

The component watches only the preferred version of each resource API. So resource object passed to the event handlers has the
structure of the preferred version.

The component is responsible for the handling of following Kubernetes API edge cases:

Resources of the deprecated extensions API group have duplicates in groups apps, networking.k8s.io, policy.
* The ReplicaSet from apps group might reference Deployment from the extensions group as a parent.
* The relationship between Service and Endpoint is not explicit: [kubernetes/#28483](https://github.com/kubernetes/kubernetes/issues/28483)
* The relationship between ServiceAccount and Token is not explicit.
* Resources of OpenShift deprecated groups authorization.openshift.io and project.openshift.io create
duplicates in rbac.authorization.k8s.io and core groups.

#### Top-Level Component APIs

The listing below represents top-level API exposed by cluster cache component:

```golang
// ResourceID is a unique resource identifier.
type ResourceID struct {
  // Namespace is empty for cluster-scoped resources.
  Namespace string
  Name      string
  Group     string
  Kind      string
}

type ListOptions struct {
  // A selector to restrict the list of returned objects by their labels.
  Selector metav1.LabelSelector
  // Restricts list of returned objects by namespace. If not empty the only namespaced resources are returned.
  Namespaces []string
  // If set to true then only namespaced object are returned.
  NamespacedOnly bool
  // If set to true then only cluster level object are returned.
  ClusterLevelOnly bool
  // If set to true then only objects without owners are returned.
  TopLevelOnly bool
}

// Cache provides a set of methods to access the cached cluster's state. All methods are thread safe.
type Cache interface {
  // List returns a list of resource ids which match the specified list options
  List(options ListOptions) ([]ResourceID, error)
  // GetResourceAPIMetadata returns API ( metav1.APIResource includes information about supported verb, namespaced/cluster level etc)
  GetResourceAPIMetadata(gk schema.GroupKind) (metav1.APIResource, error)
  // IterateChildTree builds a DAG using parent-child relationships based on ownerReferences resource field and 
  // traverse resources in a topological order starting from specified root ids
  IterateChildTree(roots []ResourceID, action func(key ResourceID) error) error
}
```

The Cache interface methods are serving following use cases:

List:
* Returns resources managed by Argo CD/Flux. Typically top-level resources labeled with a special label.
* Returns orphaned namespace resources. This will enable the Argo CD feature of warning the user if a namespace has any unmanaged resources.

GetResourceAPIMetadata:
* Answers whether a  resource namespace-scoped or cluster-scoped. This is useful in two cases:
    * to gracefully handle user errors when cluster-level resource Git have namespace. This is incorrect, but kubectl gracefully handles such errors.
    * set fallback namespace to namespaced resources without namespace
* Helps to create a dynamic K8s client and specify the resource/kind.

IterateChildTree:
* The method allows Argo CD to get information about resources tree which is used to visualize cluster state in the UI

#### Customizations
The listing below contains a set of data structures that allows customizing caching behavior.

```golang
type ResourceFilter struct {
  APIGroups []string
  Kinds     []string
}

type ResourcesAPIFilter struct {
  // ResourceExclusions holds the api groups, kinds which should be excluded
  ResourceExclusions []ResourceFilter
  // ResourceInclusions holds the only api groups that should be included. Assumes that everything is included in empty.
  ResourceInclusions []ResourceFilter
}

// ResourceEventHandlers is a set of handlers which are executed when resources updated/created/deleted.
type ResourceEventHandlers struct {
  OnCreated func(obj *unstructured.Unstructured)
  OnUpdated func(updated *unstructured.Unstructured)
  OnDeleted func(key ResourceID)
}

// Settings contains list of parameters which customize caching behavior
type Settings struct {
  Filter        ResourcesAPIFilter
  EventHandlers ResourceEventHandlers
  Namespaces []string
  ResyncPeriod time.Duration
}

func NewClusterCache(config *rest.Config, settings Settings) (Cache, error)
```

ResourceEventHandlers:
A set of callbacks that are executed when cache changes. Useful to collect and cache additional information about resources, for example:
* Cache whole manifest of a managed resource to use it later for reconciliation
* Cache resource metadata such as a list of images or health status.

ResourceAPIFilter:
Enables limiting the set of monitored resource APIs. 

Namespaces:
Allows switching component into namespace only mode. If one or more namespaces are specified then component ignore cluster level resources and watch only resources in the specified namespaces. 

NOTE: Kubernetes API allows to list/watch resources only in one namespace or the whole cluster. So if more than one namespace is specified then component have to start separate set of watches for each namespace.

ResyncPeriod:
Specifies interval after which cluster cache should be automatically invalidated.

#### Health Assessment (optionally)

The health assessment subcomponent provides the ability to get health information about a given resource. The health assessment package is not directly related to caching but helps to leverage functionality provided by caching and thus its proposed for inclusion into the caching component..

The health assessment logic is available in package argoproj/argo-cd/util/health and includes the following features:
* Support for several Kubernetes built-in resources such as Pod, ReplicaSet, Pod, Ingress and few others
* A framework that allows customizing health assessment logic using Lua script. Framework includes testing infrastructure.

The health information is represented by the following data structure:

```golang
type HealthStatus struct {
  Status  HealthStatusCode
  Message string
}
```

The health status might take one of the following values:
* Healthy/Degraded - self explanatory
* Progressing - the resource is not healthy yet but there is still a chance to become Healthy. 
* Unknown - the health assessment failed. The error message is in the `Message` field.
* Suspended - the resource is neither progressing nor degraded. For example Deployment is considered suspended if `spec.paused` field is set to true.
* Missing - the expected resource is missing

The library API is represented by a single method:

```golang
type HealthAssessor interface {
  GetResourceHealth(obj *unstructured.Unstructured) (*HealthStatus, error)
}
```

#### Additional Considerations

The live state cache could be useful for the docker-registry monitoring feature: the `OnUpdated` resource event handler can be used to maintain a images pull secrets. However, if the docker registry part is extracted into a separate binary we would have to run a separate instance of a cluster cache which means 2x more Kubernetes API calls. The workaround would be to optionally point Docker Registry Monitor to Flux?

## Reconciliation [WIP]
## Access to Git repositories [WIP]
## Manifest Generation [WIP]
## Resources reconciliation  [WIP]
## Sync Planning [WIP]
