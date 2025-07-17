---
title: Server Side Pagination for Applications List and Watch APIs
authors:
  - "@alexmt"
sponsors:
  - "@jessesuen"
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2024-02-14
last-updated: 2024-02-14
---

# Introduce Server Side Pagination for Applications List and Watch APIs

Improve Argo CD performance by introducing server side pagination for Applications List and Watch APIs.

## Open Questions [optional]

This is where to call out areas of the design that require closure before deciding to implement the
design.


## Summary

The Argo CD API server currently returns all applications in a single response. This can be a performance
bottleneck when there are a large number of applications. This proposal is to introduce server side pagination
for the Applications List and Watch APIs.

## Motivation

The main motivation for this proposal it to improve the Argo CD UI responsiveness when there are a large number
of applications. The API server memory usage increases with the number of applications however this is not critical
and can be mitigated by increasing memory limits for the API server deployment. The UI however becames unresponsive
even on a powerful machine when the number of applications increases 2000. The server side pagination will allow
to reduce amount of data returned by the API server and improve the UI responsiveness.

### Goals

* Support server side pagination for Applications List and Watch APIs

* Leverage pagination in the Argo CD UI to improve responsiveness

* Leverage pagination in the Argo CD CLI to improve performance and reduce load on the API server

### Non-Goals

* The API Server is known to use a lot of CPU while applying very large RBAC policies to a large number of applications.
  Even with pagination API still need to apply RBAC policies to return "last page" response. So the issueis not addressed by this proposal.

## Proposal

**Pagination Cursor**

It is proposed to add `offset` and `limit` fields for pagination support in Application List API.
The The Watch API is a bit more complex. Both Argo CD user interface and CLI are relying on the Watch API to display real time updates of Argo CD applications.
The Watch API currently supports filtering by a project and an application name. In order to effectively
implement server side pagination for the Watch API we cannot rely on the order of the applications returned by the API server. Instead of
relying on the order it is proposed to rely on the application name and use it as a cursor for pagination. Both the Applications List and Watch
APIs will be extended with the optional `minName` and `maxName` fields. The `minName` field will be used to specify the application name to start from
and the `maxName` field will be used to specify the application name to end at. The fields should be added to the `ApplicationQuery` message
which is used as a request payload for the Applications List and Watch APIs.

```proto
message ApplicationQuery { 
  // ... existing fields
  // New proto fields for server side pagination
	// the application name to start from (app with min name is included in response)
	optional string minName = 9;
	// the application name to end at (app with max name is included in response)
	optional string maxName = 10;
  	// offset
	optional int64 offset = 18;
	// limit
	optional int64 limit = 19;
}
```

**Server Side Filtering**

In order to support server side pagination the filtering has to be moved to the server side as well. `ApplicationQuery` message needs to be extended with the following fields:

```proto
message ApplicationQuery { 
  // ... existing fields
  // New proto fields for server side filtering
	// the repos filter
	repeated string repos = 11;
	// the clusters filter
	repeated string clusters = 12;
	// the namespaces filter
	repeated string namespaces = 13;
	// the auth sync filter
	optional bool autoSyncEnabled = 14;
	// the sync status filter
	repeated string syncStatuses = 15;
	// the health status filter
	repeated string healthStatuses = 16;
	// search
	optional string search = 17;
}
```

The Argo CD UI should be updated to populate fields in the List and Watch API requests instead of performing filtering on the client side.

**Applications Stats**

The Argo CD UI displays the breakdown of the applications by the sync status, health status etc. Stats numbers are calculated on the client side
and rely on the full list of applications returned by the API server. The server side pagination will break the stats calculation. The proposal is to
intoduce a new `stats` field to the Applications List API response. The field will contain the breakdown of the applications by various statuses.

```golang
type ApplicationLabelStats struct {
	Key    string   `json:"key" protobuf:"bytes,1,opt,name=key"`
	Values []string `json:"values" protobuf:"bytes,2,opt,name=values"`
}

// ApplicationListStats holds additional information about the list of applications
type ApplicationListStats struct {
	Total                int64                             `json:"total" protobuf:"bytes,1,opt,name=total"`
	TotalBySyncStatus    map[SyncStatusCode]int64          `json:"totalBySyncStatus,omitempty" protobuf:"bytes,2,opt,name=totalBySyncStatus"`
	TotalByHealthStatus  map[health.HealthStatusCode]int64 `json:"totalByHealthStatus,omitempty" protobuf:"bytes,3,opt,name=totalByHealthStatus"`
	AutoSyncEnabledCount int64                             `json:"autoSyncEnabledCount" protobuf:"bytes,4,opt,name=autoSyncEnabledCount"`
	Destinations         []ApplicationDestination          `json:"destinations" protobuf:"bytes,5,opt,name=destinations"`
	Namespaces           []string                          `json:"namespaces" protobuf:"bytes,6,opt,name=namespaces"`
	Labels               []ApplicationLabelStats           `json:"labels,omitempty" protobuf:"bytes,7,opt,name=labels"`
}
```

The `stats` filter should be populated with information about all applications returned by the API server even when single page is loaded.
The Argo CD UI should be updated to use the stats returned by the API server instead of calculating the stats on the client side.

**Argo CD CLI**

The Argo CD CLI should be updated to support server side pagination. The `argocd app list` command should be updated to support `--offset` and `--limit` flags.
If the `--offset` and `--limit` flags are not specified the CLI should use pagination to load all applications in batches of 500 applications.

### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

#### User Server Side Pagination in Argo CD User Interface to improve responsiveness:
As a user, I would like to be able to navigate through the list of applications using the pagination controls.

#### User Server Side Pagination in Argo CD CLI to reduce load on the API server:
As a user, I would like to use Argo CD CLI to list applications while leveraging the pagination without overloading the API server.

### Implementation Details/Notes/Constraints [optional]

**Application Stats**

**CLI Backward Compatibility**

Typically we bump minimal supported API version when we introduce a new feature in CLI. In this case I
suggest to support gracefully missing pagination support in CLI. If the API server returns more applications than
specified in `limit` the CLI should assume pagination is not supported and response has full list of applications.
This way the user can downgrade API server without downgrading CLI.

### Security Considerations

The proposal does not introduce any new security risks.

### Risks and Mitigations

We might need to bump minimal supported API version in CLI to support pagination. The `Implementation Details` section
contains the proposal to avoid doing it.

### Upgrade / Downgrade Strategy

The proposal does not introduce any breaking changes. The API server should gracefully handle requests without pagination fields.

## Drawbacks

None.

## Alternatives

****