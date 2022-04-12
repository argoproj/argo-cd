# GitOps Engine Design - Top Down [WIP]

## Summary

During the elaboration of [Bottom Up](./design-bottom-up.md) option, we've discovered that this
approach is challenging. Although components are similar at a high-level there is
still a lot of differences in design and implementation, so it is hard to re-use either of them in
both projects without redesign. This is not surprising because code was developed
by different teams and with a focus on different use-cases. To speed-up the process it is
proposed to take a whole sub-system of one project and make it customizable enough to
be suitable for both Argo CD and Flux.

## Proposal

It is proposed to extract of Argo CD application controller subsystem, that is responsible for
interacting with Kubernetes and resources reconciliation and syncing. The sub-system should be refactored
and used in both Argo CD and Flux. During refactoring we should make sure that it supports all 
customizations that are required to keep Flux behavior untouched.There are two main reasons to try using
Argo CD as a base for reconciliation engine:
- Argo CD uses the _Application_ abstraction to represent the desired state and target the Kubernetes cluster.
This abstraction works for both Argo CD and Flux.
- The Argo CD controller leverages Kubernetes watch APIs instead of polling. This enables Argo CD features
such as Health assessment, UI and could provide better performance to Flux as well.

### Manifest Generation

The manifest generation logic is very different in Argo CD and Flux: Argo CD focuses on providing first class
support for existing config management tools while Flux provides a flexible way to connect any config
management tool using .flux.yaml files. I think we should merge manifest generation step by step, after core
of GitOps engine is established.

### Repository Access

Flux has much more Git related features than Argo CD. I think we might do the same exercise as a next step:
generalize Flux's Git access, commit verification, write back features and contribute to the engine as
a whole sub-system.

### Hypothesis and assumptions

The proposed solution is based on the assumption that despite implementation differences the core functionality of Argo CD and Flux behaves in the same way. Both projects
ultimately extract the set of manifests from Git and use "kubectl apply" to change the cluster state. The minor differences are expected but we can resolve them by introducing new
knobs.

Also, the proposed approach is based on the assumption that Argo CD engine is flexible enough to cover all Flux use-cases, reproduce Flux's behavior with minor differences and can
be easily integrated into Argo CD.

However, there is a risk that there will be too many differences and it might be not feasible to support all of them. To get early feedback, we will start with a Proof-of-Concept 
(PoC from now on) implementation which will serve as an experiment to assess the feasibility of the approach.

### Acceptance criteria

To consider the PoC successful (and with the exception of features excluded from the PoC to save time), 
all the following must hold true:
1. All the Flux unit and end-to-end tests must pass. The existing tests are limited, so we may decide to
include additional ones.
2. The UX of Flux must remain unchanged. That includes:
   - The flags of `fluxd` and `fluxctl` must  be respected and can be used in the same way as before
     resulting in the same configuration behavioural changes.
   - Flux's API must remain unchanged. In particular, the web API (used by fluxctl) and the websocket API (e.g. used to 
     communicate with Weave Cloud) must work without changes.
3. Flux's writing behaviour on Git and Kubernetes must be identical. In particular:
   - Flux+GitEngine should make changes in Git if and only if Flux without GitEngine would had done it,
     in the same way (same content) and in the same situations
   - Flux+GitEngine should add and update Kubernetes resources if and only if Flux without GitEngine would had done, 
     in the same way (same content) and in the same situations

Unfortunately, there isn't a straightforward way to decidedly check for (3).

Additionally, there must be a clear way forward (in the shape of well-defined steps) 
for the features not covered by the PoC to work (complying with the points above) int he final GitOps  
Engine.

### GitOps Engine PoC

The PoC deliverables are:

- All PoC changes are in separate branches.
- Argo CD controller will be moved to https://github.com/argoproj/gitops-engine.
- Flux will import GitOps engine component from the https://github.com/argoproj/gitops-engine repository and use it to perform cluster state syncing.
- The flux installation and fluxctl behavior will remain the same other than using GitOps engine internally. That means there will be no creation of Application CRD or Argo CD
specific ConfigMaps/Secrets.
- For the sake of saving time POC does not include implementing features mentioned before. So no commit verification, only plain .yaml files support, and full cluster mode.

## Design Details

The proposed design is based on the PoC that contains draft implementation and provides the base idea of a target design. PoC is available in
[argo-cd#fargo/engine](https://github.com/alexmt/argo-cd/tree/fargo/engine) and [flux#fargo](https://github.com/alexmt/flux/tree/fargo) 

The GitOps engine API consists of three main packages:
* The `utils` package. It consist of loosely coupled packages that implements K8S resource diffing, health assessment, etc
* The `sync<Term>` package that contains Sync<Term> data structure definition and CRUD client interface.
* The `engine` package that leverages `utils` and uses provided set of Applications as a configuration source.

```
gitops-engine
|-- pkg
|   |-- utils
|   |   |-- diff        # provides Kubernetes resource diffing functionality
|   |   |-- kube        # provides utility methods to interact with Kubernetes
|   |   |-- lua         # provides utility methods to run Lua scripts
|   |   |-- sync<Term>   # provides utility methods to manipulate Sync<Term> (e.g. start sync operation, wait for sync operation, force reconciliation)
|   |   `-- health      # provides Kubernetes resources health assessment
|   |-- sync<Term>
|   |   |-- apis       # contains data structures that describe Sync<Term>
|   |   `-- client     # contains client that provides Sync<Term> CRUD operations
|   `-- engine         # the engine implementation
```

The engine is responsible for the reconciliation of Kubernetes resources, which includes:
- Interacting with the Kubernetes API: load and maintain the cluster state; pushing required changes to the Kubernetes API.
- Reconciliation logic: match target K8S cluster resources with the resources stored in Git and decide which resources should be updated/deleted/created.
- Syncing logic: determine the order in which resources should be modified; features like sync hooks, waves, etc.

The manifests generation is out of scope and should be implemented by the Engine consumer.

### Engine API

> `Sync<Term>` is a place holder to a real name. The name is still under discussion

The engine API includes three main parts:
- `Engine` golang interface
- `Sync<Term>` data structure and `Sync<Term>Store` interface which provides access to the units.
- Set of data structures which allows configuring reconciliation process.

**Engine interface** - provides set of features that allows updating reconciliation settings and subscribe to engine events.
```golang
type Engine interface {
	// Run starts reconciliation loop using specified number of processors for reconciliation and operation execution.
	Run(ctx context.Context, statusProcessors int, operationProcessors int)

	// SetReconciliationSettings updates reconciliation settings
	SetReconciliationSettings(settings ReconciliationSettings)

	// OnBeforeSync registers callback that is executed before each sync operation.
	OnBeforeSync(callback func(appName string, tasks []SyncTaskInfo) ([]SyncTaskInfo, error)) Unsubscribe

	// OnSyncCompleted registers callback that is executed after each sync operation.
	OnSyncCompleted(callback func(appName string, state OperationState) error) Unsubscribe

    // OnClusterCacheInitialized registers a callback that is executed when cluster cache initialization is completed.
    // Callback is useful to wait until cluster cache is fully initialized before starting to use cached data.
	OnClusterCacheInitialized(callback func(server string)) Unsubscribe

	// OnResourceUpdated registers a callback that is executed when cluster resource got updated.
	OnResourceUpdated(callback func(cluster string, un *unstructured.Unstructured)) Unsubscribe

	// OnResourceRemoved registers a callback that is executed when a cluster resource gets removed.
	OnResourceRemoved(callback func(cluster string, key kube.ResourceKey)) Unsubscribe

	// On<Term>Event registers callback that is executed on every sync <Term> event.
	On<Term>Event(callback func(id string, info EventInfo, message string)) Unsubscribe
}

type Unsubscribe func()
```

**Sync<Term> data structure** - allows engine accessing sync <Term> and update status.

```golang
type Sync<Term> struct {
	ID         string
	Status      Status
	Source      ManifestSource
	Destination ManifestDestination
	Operation   *Operation
}

type Sync<Term>Store interface {
	List() ([]Sync<Term>, error)
	Get(id string) (Sync<Term>, error)
	SetStatus(id string, status Status) (Sync<Term>, error)
	SetOperation(id string, operation *Operation) error
}
```

**Settings data structures** - holds reconciliation settings and cluster credentials.
```golang
type ReconciliationSettings struct {
	// AppInstanceLabelKey holds label key which is automatically added to every resource managed by the application
	AppInstanceLabelKey string
	// ResourcesFilter holds settings which allows to configure list of managed resource APIs
	ResourcesFilter resource.ResourcesFilter
	// ResourceOverrides holds settings which customize resource diffing logic
	ResourceOverrides map[scheme.GroupKind]appv1.ResourceOverride
}

// provides access to cluster credentials
type CredentialsStore interface {
	GetCluster(ctx context.Context, name string) (*appv1.Cluster, error)
	WatchClusters(ctx context.Context, callback func(event *ClusterEvent)) error
}
```

**ManifestGenerator** a Golang interface that must be implemented by the GitOps engine consumer.
```golang
type ManifestResponse struct {
    // Generated manifests
    Manifests  []string
    // Resolved Git revision
    Revision   string
}

type ManifestGenerator interface {
	Generate(ctx context.Context, app *appv1.Application, revision string, noCache bool) (*ManifestResponse, error)
}
```

**Engine instantiation** in order to create engine the consumer must provide the cluster credentials store and manifest generator as well as reconciliation settings.
The code snippets below contains `NewEngine` function definition and usage example:

```golang
func NewEngine(settings ReconciliationSettings, creds CredentialsStore, manifests ManifestGenerator)
```

```golang
myManifests := manifests{}
myClusters := clusters{}
engine := NewEngine(ReconciliationSettings{}, myManifests, myClusters)
engine.OnBeforeSync(func(appName string, tasks []SyncTaskInfo) ([]SyncTaskInfo, error) {
    // customize syncing process by returning update list of sync tasks or fail to prevent syncing
    return tasks, nil
})
```

## Alternatives

[Bottom-up](./design-bottom-up.md)
