---
title:  Merging of Application set codebase into Argocd
authors:
  - "@rishabh625" # Authors' github accounts here.
sponsors:
  - TBD        # List all intereste parties here.
reviewers:
  - TBD
  - TBD
approvers:
  - TBD
  - TBD

creation-date: 2022-03-09
last-updated: 2022-03-09
---

# Merging of Application set codebase into Argocd

This is the proposal to merge codebase of application set into argocd. Application Set and Argocd are tightly coupled but yet maintained in different repository, adding to [issues](https://github.com/argoproj/applicationset/issues/528) wrt releases and support .Creating this proposal to finalize approach on merging the codebase


## Summary

Since Application set is matured enough and bundled with Argocd , need to merge application set with argocd. Merging application set code into argocd will pave way to introduce backend support of application set into argocd and eventually cli/UI.Before merging need to finalize on approach on how application set to be clubed with argocd  

## Motivation

Motivation is to have Tighter integration of application set into argocd, so that going forward we can have first-class ApplicationSet support in Argo CD.

### Goals

- Finalizing on how applicationset to run as part of argocd
- To have all tests of application set pass when merged into argocd
- Removal or merging of repeated code between argocd and applicationset
- Introduction of new tests if required 

### Non-Goals

 - Merging of repository of applicationset so that application set is maintained into argocd
 - Pave a way to add backend support for appset into argocd

## Proposal

Below are the following options to merge

#### Option 1

#### Merge application-set-controller into existing application-controller 

Since job of the Application Controller is to watch Applications and generate corresponding kubernetes resources declared in git,application set does the same for applications with more powerful generators, also it's an alternative to  apps of apps pattern

since both deals with creaion/deletion of applications one should consider merging the controllers together
### Pros 
-  Running appset controller as sts into same os no seperate deployment required
-  ApplicationSet will be more tightly coupled 
-  Can Include Managing of Appset objects into argocd api server, no need to delegate request from argocd to appset seperately.
-  RBAC of applications that appset is managing can be inherited for appsets RBAC

### Cons
- Lot of work,Technically difficult to merge, lot of things in argocd can break.
- Concerns wrt HA, since application set scales based on managed clusters

If we consider scaling beyond number of managed clusters, we can consider sharding based on number of applications and applicationset CRD and each replica to manage shards with some kind of election.Concerns wrt HA can be overcome but need to consider it as part of merging

#### Option 2

Run application set as seperate microservices

### Pros
- Lot easier to merge
- A seperate Backend support can be easily added for dealing with appset resources and will not touch most part of existing argocd server.
- Can include most part of appset as submodule in git, retaining commit history.

### Cons
- Need to have seperate Deployment,expose via seperate service
- Development of cli for appset into argocd will require delegation of grpc requests to this new microservice.
- Apart from controller, appset to have a grpc server,http server.


<!-- ### Implementation Details/Notes/Constraints [optional]

What are the caveats to the implementation? What are some important details that didn't come across
above. Go in to as much detail as necessary here. This might be a good place to talk about core
concepts and how they relate.

You may have a work-in-progress Pull Request to demonstrate the functioning of the enhancement you are proposing.

### Detailed examples

### Security Considerations

* How does this proposal impact the security aspects of Argo CD workloads ?
* Are there any unresolved follow-ups that need to be done to make the enhancement more robust ?  

### Risks and Mitigations

What are the risks of this proposal and how do we mitigate. Think broadly. 

For example, consider
both security and how this will impact the larger Kubernetes ecosystem.

Consider including folks that also work outside your immediate sub-project.


### Upgrade / Downgrade Strategy

If applicable, how will the component be upgraded and downgraded? Make sure this is in the test
plan.

Consider the following in developing an upgrade/downgrade strategy for this enhancement:

- What changes (in invocations, configurations, API use, etc.) is an existing cluster required to
  make on upgrade in order to keep previous behavior?
- What changes (in invocations, configurations, API use, etc.) is an existing cluster required to
  make on upgrade in order to make use of the enhancement?

## Drawbacks

The idea is to find the best form of an argument why this enhancement should _not_ be implemented.

## Alternatives

Similar to the `Drawbacks` section the `Alternatives` section is used to highlight and record other
possible approaches to delivering the value proposed by an enhancement. -->