---
title: Fix Out-of-Order Syncing in ApplicationSet Progressive Sync
authors:
  - "@ranakan19" # Authors' github accounts here.
sponsors:
  - TBD        # List all interested parties here.
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2025-09-25
last-updated: 2025-09-25
---

# Fix Out-of-Order Syncing in ApplicationSet Progressive Sync

This proposal addresses the issue of out-of-order syncing in ArgoCD's ApplicationSet progressive sync feature with RollingSync strategy, which undermines the core ordering guarantees that users depend on.

## Open Questions [optional]

- What is the acceptable performance impact of additional API calls for staleness detection?
- Should we implement both options (requeue vs direct API calls) with a configuration flag?
- Are there other edge cases in the eventual consistency model that we should address?

## Summary

The progressive sync feature of ArgoCD's ApplicationSets with RollingSync strategy is designed to define an order for deploying and updating applications. However, users have reported observing out-of-order syncing of applications. This discrepancy arises because the ApplicationSet controller, relies on eventual consistency, while real-time status information is needed for accurate ordering decisions.

This proposal presents solutions to address the root cause of informer cache staleness affecting progressive sync ordering decisions.

## Motivation

Progressive sync is a critical feature for users who need to enforce strict deployment ordering. The current out-of-order syncing behavior breaks these use cases and reduces confidence in the progressive sync feature.

### Goals

- Maintain acceptable performance characteristics
- Provide reliable ordering guarantees for RollingSync strategy

### Non-Goals

- Optimizing for scenarios where ordering is not important

## Proposal

This proposal presents three options to address the cache staleness issue that causes out-of-order syncing.

### Use cases

Users of ApplicationSet's progressive sync should not encounter situations where applications of step 'x' are progressing before applications of step 'y' where x > y.

The following scenarios describe the expected progressive sync behavior:

**Scenario 1: Single Source Repository**
- **Given**: All applications in an ApplicationSet pull from the same Git repository and targetRevision
- **When**: A change is pushed to that targetRevision
- **Then**: Applications should be synced progressively through the defined stages in order

**Scenario 2: ApplicationSet Template Changes**
- **Given**: A change is made to the ApplicationSet's `spec.template` field
- **When**: This change causes two or more Applications to go OutOfSync
- **Then**: Those changes should be progressively synced through the defined stages in order

**Scenario 3: Multiple Source Repositories**
- **Given**: An ApplicationSet's Applications have different sources (repos, branches, etc.)
- **When**: Changes are made to the sources of more than one application simultaneously
- **Then**: Those applications should be progressively synced through the defined stages in order  


### Implementation Details/Notes/Constraints

#### Root Cause Analysis

The issue stems from several factors:
1. ApplicationSet controller uses cache's eventual consistency model
2. Progressive sync decisions are made based on potentially stale informer cache data
3. The logic to make ordering decisions gets the owned application's list from cache
4. Applications from the same ApplicationSet are reconciled and detect changes in an unordered way

#### Additional Context

- Reconcile requests contain only NamespacedName of ApplicationSet, requiring cache lookups for owned applications
- Not every resourceVersion update in applications triggers an ApplicationSet reconcile (while setting up manager, we filter out changes in applications to be relevant to ApplicationSet - as per design)
```go
if enableProgressiveSyncs {
	if appOld.Status.Health.Status != appNew.Status.Health.Status || 
	   appOld.Status.Sync.Status != appNew.Status.Sync.Status {
		return true
	}

	if appOld.Status.OperationState != nil && appNew.Status.OperationState != nil {
		if appOld.Status.OperationState.Phase != appNew.Status.OperationState.Phase ||
		   appOld.Status.OperationState.StartedAt != appNew.Status.OperationState.StartedAt {
			return true
		}
	}
}
```

#### Option 1: ResourceVersion-based Staleness Detection with Requeue

Add a new field `ResourceVersion` to `ApplicationSetApplicationStatus` which copies the value of ResourceVersion from Application when updating ApplicationStatus. Implement cache staleness detection for progressive sync decisions by comparing current app ResourceVersion with the stored one. If staleness is detected, do not proceed with progression (`syncEnabled = false`) and reconcile later.
This may not be an absolute way of detecting staleness due to reasons mentioned above, but combined with multiple gets while performing progressive sync avoids the single point of failure. Currently progressive sync works with a list of owned applications, which are fetched before performing progressive - if the initial getCurrentApplications() call returns stale data, the entire progressive sync decision-making process operates on stale data. 

```go
type ApplicationSetApplicationStatus struct {
	// Application contains the name of the Application resource
	Application string `json:"application" protobuf:"bytes,1,opt,name=application"`
	// LastTransitionTime is the time the status was last updated
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,2,opt,name=lastTransitionTime"`
	// Message contains human-readable message indicating details about the status
	Message string `json:"message" protobuf:"bytes,3,opt,name=message"`
	// Status contains the AppSet's perceived status of the managed Application resource: (Waiting, Pending, Progressing, Healthy)
	Status string `json:"status" protobuf:"bytes,4,opt,name=status"`
	// Step tracks which step this Application should be updated in
	Step string `json:"step" protobuf:"bytes,5,opt,name=step"`
	// TargetRevision tracks the desired revisions the Application should be synced to.
	TargetRevisions []string `json:"targetRevisions" protobuf:"bytes,6,opt,name=targetrevisions"`
	// ResourceVersion tracks the resourceVersion of the Application when the status was last assessed
	// This helps detect when progressive sync decisions may be based on stale informer cache data
	ResourceVersion string `json:"resourceVersion,omitempty" protobuf:"bytes,7,opt,name=resourceVersion"`
}
```

**Implementation:** 
Compare current Application resourceVersion with stored version in `getAppsToSync` before making progressive sync decisions.
Get applications again before updating to progressing
If potentially stale - requeue 

**Pros:**
- Simple
- Addresses identifying cache lag
- Leverages existing Kubernetes resourceVersion mechanism

**Cons:**
- May cause sync delays
- Requeue could also be expensive if the AppSet makes external API calls on reconcile.

#### Option 2: ResourceVersion-based Staleness Detection with Direct API Calls

Same as Option 1, but if staleness is detected, make direct API calls.  
If there is a way to identify critical Applications, making direct API calls can be selective. Direct API calls for all applications become prohibitive at scale (Don't have an exact value of the number of applications for breaking point). Additionally, filtering by cached status creates circular dependency - we can't determine "critical" applications using the same stale data we're trying to refresh.

**Pros:**
- no stale status

**Cons:**
- Need to determine appropriate thresholds
- Direct API calls are expensive

#### Option 3: Separate ApplicationSetApplicationStatus Controller 

Create a dedicated controller specifically for managing `ApplicationSetApplicationStatus`.
`ApplicationSetApplicationStatus` is only relevant when progressive sync is enabled and strategy is rolling sync, thus can easily be separate from the main ApplicationSet controller. 

The new controller can either work:
- with separation of concern - Status Field Ownership

```go
// ApplicationSet.Status field separation
type ApplicationSetStatus struct {
// Managed by Main ApplicationSet Controller
Conditions        []ApplicationSetCondition         `json:"conditions,omitempty" protobuf:"bytes,1,name=conditions"`
Resources []ResourceStatus `json:"resources,omitempty" protobuf:"bytes,3,opt,name=resources"`
ResourcesCount int64 `json:"resourcesCount,omitempty" protobuf:"varint,4,opt,name=resourcesCount"`

// Managed by ApplicationSetApplicationStatus Controller  
ApplicationStatus []ApplicationSetApplicationStatus `json:"applicationStatus,omitempty" protobuf:"bytes,2,name=applicationStatus"` // Progressive sync RollingSync status
}
```
- OR create a new CRD - such that primary resource of this new controller is a new CRD and ApplicationSet watches and updates ApplicationSetApplicationStatus from this new CRD

Both controllers use the same manager for efficiency:
- Shared cache reduces API server load and memory usage
- Consistent data view between controllers
- Single process deployment with unified leader election


```go
// New dedicated controller within same manager
type ApplicationSetStatusController struct {
    client.Client
}

```

```go
// In cmd/argocd-applicationset-controller/commands/applicationset_controller.go

// Setup main ApplicationSet controller (existing)
if err = (&controllers.ApplicationSetReconciler{
    Generators:                 topLevelGenerators,
    Client:                     mgr.GetClient(),
    Scheme:                     mgr.GetScheme(),
    Recorder:                   mgr.GetEventRecorderFor("applicationset-controller"),
    // ... other fields
}).SetupWithManager(mgr, enableProgressiveSyncs, maxConcurrentReconciliations); err != nil {
    log.Error(err, "unable to create controller", "controller", "ApplicationSet")
    os.Exit(1)
}

// Setup new ApplicationSetStatus controller
 if err = (&controllers.ApplicationSetStatusController{
     Client:        mgr.GetClient(),
     APIReader:     mgr.GetAPIReader(), // For direct API calls, if needed
     Scheme:        mgr.GetScheme(),
     Recorder:      mgr.GetEventRecorderFor("applicationset-status-controller"),
     Metrics:       &metrics,
 }).SetupWithManager(mgr, enableProgressiveSyncs, maxConcurrentReconciliations); err != nil {
     log.Error(err, "unable to create controller", "controller", "ApplicationSetStatus")
     os.Exit(1)
 }
```

```go
// ApplicationSetStatusController.SetupWithManager()
func (r *ApplicationSetStatusController) SetupWithManager(mgr ctrl.Manager, maxConcurrentReconciliations int) error {
    // Index Applications by their owner ApplicationSet for efficient lookup
    if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &argov1alpha1.Application{}, 
        ".metadata.controller", appControllerIndexer); err != nil {
        return fmt.Errorf("error setting up application indexer: %w", err)
    }

    return ctrl.NewControllerManagedBy(mgr).WithOptions(controller.Options{
        MaxConcurrentReconciles: maxConcurrentReconciliations,
    }).For(&argov1alpha1.Application{}).  // Primary: Watch Applications directly or the new CRD
        Watches(
            &argov1alpha1.ApplicationSet{},
            handler.EnqueueRequestsFromMapFunc(r.findApplicationsForApplicationSet),
        ).
        WithEventFilter(applicationSetProgressiveSyncPredicate()).
        Complete(r)
}

// Map ApplicationSet changes to all its owned Applications
func (r *ApplicationSetStatusController) findApplicationsForApplicationSet(ctx context.Context, obj client.Object) []reconcile.Request {
    appSet := obj.(*argov1alpha1.ApplicationSet)
    
    // Only process if progressive sync is enabled
    if !isProgressiveSyncEnabled(appSet) {
        return nil
    }
    
    // Find all Applications owned by this ApplicationSet
    var appList argov1alpha1.ApplicationList
    if err := r.List(ctx, &appList, client.MatchingFields{
        ".metadata.controller": appSet.Name,
    }, client.InNamespace(appSet.Namespace)); err != nil {
        return nil
    }
    
    // Create reconcile requests for each Application
    requests := make([]reconcile.Request, len(appList.Items))
    for i, app := range appList.Items {
        requests[i] = reconcile.Request{
            NamespacedName: types.NamespacedName{
                Name:      app.Name,
                Namespace: app.Namespace,
            },
        }
    }
    
    return requests
}

// Only reconcile Applications owned by ApplicationSets with progressive sync enabled
func applicationSetProgressiveSyncPredicate() predicate.Predicate {
    return predicate.Funcs{
        CreateFunc: func(e event.CreateEvent) bool {
            return isOwnedByProgressiveSyncApplicationSet(e.Object)
        },
        UpdateFunc: func(e event.UpdateEvent) bool {
            return isOwnedByProgressiveSyncApplicationSet(e.ObjectNew)
        },
        DeleteFunc: func(e event.DeleteEvent) bool {
            return isOwnedByProgressiveSyncApplicationSet(e.Object)
        },
    }
}

func isOwnedByProgressiveSyncApplicationSet(obj client.Object) bool {
    // Check if this is an Application owned by an ApplicationSet
    if app, ok := obj.(*argov1alpha1.Application); ok {
        owner := metav1.GetControllerOf(app)
        if owner != nil && owner.Kind == "ApplicationSet" {
            // Would need to fetch the ApplicationSet to check if progressive sync is enabled
            // For simplicity, assume all ApplicationSet-owned Applications are relevant
            return true
        }
    }
    return false
}
```

**Controller Coordination and Status Management:**

When an Application changes, **three controllers** may be triggered:
1. **Application Controller** (manages Application lifecycle)
2. **ApplicationSet Controller** (handles generators, templating, Application CRUD)
3. **ApplicationSetApplicationStatus Controller** (manages progressive sync status)

Thus **Status updates use optimistic locking** (resourceVersion) to handle conflicts

**Pros:**
- **Single Responsibility Principle**: Each controller has a focused purpose

**Cons:**
- Introduces additional controller complexity
- Requires coordination between two controllers

#### Option 4: Debouncing Reconciliations 
When multiple owned Applications trigger rapid successive reconciliations of the ApplicationSet, debouncing aggregates these events to reduce succesive processing and also improving odds of getting cache updated meanwhile. 
Have seen the workaround of increasing jitter worked, having a debouncer reset timer for per-object reconciles should help. That is, concurrent ApplicationSet reconcilations continue, but only one reconcile of ApplicationSet proceeds when multiple requests of the same applicationset received in a short period of time.

```go
 type ApplicationSetReconciler struct {
	//... existing fields 
      resourceMutexes sync.Map // map[string]*sync.Mutex
  }

```


### Security Considerations

- The addition of resourceVersion field does not introduce new security concerns
- Direct API calls (Option 2) should use the existing RBAC permissions
- No additional sensitive data is stored or transmitted

### Risks and Mitigations

pros and cons have been identified of each options, once an option is decided by the community, can update this section

### Upgrade / Downgrade Strategy

**Options 1 & 2:**
- The new `ResourceVersion` field should be optional and backward compatible
- Existing ApplicationSets will continue to work, empty values to be handled in code, field will be added the next time applicationStatus is updated with rolling sync.
- During upgrade, the field will be populated on the next status update
- Downgrade will simply ignore the additional field

**Option 3 :**
- **Phase 1**: Deploy new ApplicationSetStatus controller alongside existing ApplicationSet controller
- **Phase 2**: Enable progressive sync status management in new controller via feature flag
- **Phase 3**: Remove progressive sync logic from main ApplicationSet controller


## Drawbacks

**Options 1 & 2:**
- Adds complexity to the ApplicationSet status structure
- May impact performance in high-scale environments (Option 1: requeuing, Option 2: API calls)


**Option 3:**
- Introduces additional operational complexity with separate controller
- Requires coordination between two controllers updating different parts of ApplicationSet status
- More complex deployment model and monitoring requirements?


## Alternatives Considered

### Generation Comparison
Use existing `metadata.generation` field for staleness detection. Applications have `metadata.generation`, and when processed by the controller, `observedGeneration` is populated to match. However, generation only tracks `.spec` changes, not `.status` changes, while progressive sync decisions depend on health/sync status.

### Timestamp Heuristics
Compare `lastTransitionTime` with current time. Unreliable because timestamps only indicate when transitions happened, not what changed, and determining the gap between application updates and ApplicationSet reconciliation is difficult.

### Step-aware Filtering
Modify the `Owns` predicate to check if an Application's step is currently active. This increases complexity without addressing the fundamental cache staleness issue.
