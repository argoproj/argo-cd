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

## Open Questions [optional]

Before starting need to close open PR's of application set and freeze for no more PR's?

1) merge applicationset v0.4.0 into argo cd
2) make sure it works
3) freeze PRs
4) merge changes made to applicationset controller after v0.4.0
5) close PRs with a message indicating they can be re-opened in argo cd repo


While merging need to preserve commit history? 

## Summary

Since Application set is matured enough and bundled with Argocd , need to merge application set with argocd. Merging application set code into argocd will pave way to introduce backend support of application set into argocd and eventually cli/UI.Before merging need to finalize on approach on how application set to be clubed with argocd  

## Motivation

Motivation is to have Tighter integration of application set into argocd, so that going forward we can have first-class ApplicationSet support in Argo CD.

Merging will solve this two issues
- Currently have to Deal with two seperate docker images.After merge will have one single image
- Circular dependency.Since Application set will not vendor argocd after merge,circular dependency will be solved.

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

Kubernetes have different controlllers shipped in a single daemon kube-controller-manager


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


 ### Implementation Details/Notes/Constraints [optional]

For Option1  from above

### Merging of os process
1) Create an invocker that invokes application-controller and application-set controller seperately and both controller run in same os.

2) application-set controller to follow network policies, service account same as of application-controller

3) Exposing via service for appset components like webhook

### Merging of controller code

1)  Creating a new Informer and Lister for Applicationset CRD and Registering the Handler functions copying from application set code.

2) Creating new service for webhooks in applicationset

Option 2 

Running as Microservice.

1) Implementing or removal of repeated codes
like ClusterUtils in applicationset
2) Exposing controller via service


<!-- What are the caveats to the implementation? What are some important details that didn't come across
above. Go in to as much detail as necessary here. This might be a good place to talk about core
concepts and how they relate.

You may have a work-in-progress Pull Request to demonstrate the functioning of the enhancement you are proposing. -->

### Detailed examples

### Security Considerations

Security improvements need to be taken care while merge
- Examine Logging of application set 
- May be Make Webhook events to be authenticated
- Application Set to emit kubernetes events same as argocd

<!-- * How does this proposal impact the security aspects of Argo CD workloads ?
* Are there any unresolved follow-ups that need to be done to make the enhancement more robust ?  -->

<!-- ### Risks and Mitigations

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