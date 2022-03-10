---
title:  Merging of ApplicationSet codebase into Argo CD
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
last-updated: 2022-03-10
---

# Merging of ApplicationSet codebase into Argo CD

This is the proposal to merge codebase of ApplicationSet into Argo CD. ApplicationSet and Argo CD are closely related but yet maintained in different repositories, adding to [issues](https://github.com/argoproj/applicationset/issues/528) wrt releases and support. Creating this proposal to finalize the approach on merging the codebase

## Open Questions [optional]

Before starting need to close open PR's of application set and freeze for no more PR's?

1) merge applicationset v0.4.0 into argo cd
2) make sure it works
3) freeze PRs
4) merge changes made to applicationset controller after v0.4.0
5) close PRs with a message indicating they can be re-opened in argo cd repo


While merging need to preserve commit history? 

## Summary

Since ApplicationSet is matured enough and has been graduated from argoprojlab. It has been included with Argo CD install.yaml. As we work out the releasing of new Argo CD and ApplicationSet, we see the growing need to tightly couple ApplicationSet with Argo CD. Merging ApplicationSet code into argocd will pave the way to introduce backend support of ApplicationSset into Argo CD and eventually CLI/UI. We eed to finalize on an approach on how ApplicationSet to be merged with Argo CD.

## Motivation

Motivation is to have a tighter integration of ApplicationSet into Argo CD. Going forward, we can have first-class ApplicationSet support in Argo CD.

Merging will solve this two issues
- Currently have to Deal with two seperate docker images.After merge will have one single image
- Circular dependency.Since Application set will not vendor argocd after merge,circular dependency will be solved.

### Goals

- Finalizing on how ApplicationSet to run as part of Argo CD
- To have all tests of ApplicationSet pass when merged into Argo CD
- Removal or refactoring of repeated code between Argo CD and ApplicationSet
- Introduction of new tests if required 

### Non-Goals

 - Merging of repository of ApplicationSet so that ApplicationSet is maintained into Argo CD
 - Pave a way to add backend support for ApplicationSet into Argo CD

## Proposal

Below are the possible approaches to merge ApplicationSet into Argo CD.

#### Option 1 Merging ApplicationSet Controller to the same process of Argo CD Application Controller

#### Merge application-set-controller into existing application-controller 

Since the job of the Application Controller is to watch Applications and generate corresponding Kubernetes resources declared in Git, ApplicationSet Controller performs the same task for applications with more powerful generators. Also, it's an alternative to apps of apps pattern.

Since both controllers deal with creaion/deletion of applications one should consider merging the controllers together.
### Pros 
-  Running ApplicationSet controller as sts (?) into same os (?) no seperate deployment required
-  ApplicationSet will be more tightly coupled 
-  Can include managing of Applicationset objects into argocd api server. No need to delegate request from Argo CD to ApplicationSet (Controller?) seperately.
-  RBAC of applications that appset is managing can be inherited for ApplicatonSet RBAC

### Cons
- A lot of work. Technically difficult to merge. A lot of things in Argo CD can break.
- Concerns wrt HA (please elaborate, ha != scaling), since scaling of ApplicationSet is based on managed clusters

If we consider scaling beyond number of managed clusters, we can consider sharding based on number of applications and ApplicationSet CRD and each replica to manage shards with some kind of election. Concerns wrt HA can be overcome but need to consider it as part of merging


#### Option 2 Run ApplicationSet as a seperate process/service
Kubernetes have different controlllers shipped in a single daemon kube-controller-manager

### Pros
- Lot easier to merge
- A seperate backend support can be easily added for dealing with ApplicationSet resources and will leave most of argocd server intac.
- Can include ApplicationSet as a submodule in Git, retaining commit history.

### Cons
- Need to have seperate Service and Deployment
- Development of CLI for ApplicationSet into Argo CD will require delegation of grpc requests to this new microservice.
- Apart from controller, ApplicationSet to have a grpc server,http server.


 ### Implementation Details/Notes/Constraints [optional]

For Option1  from above

### Merging of os process
1) Create an invocker that invokes application-controller and application-set controller seperately and both controller run in same os.

2) application-set controller to follow network policies, service account same as of application-controller

3) Exposing via service for appset components like webhook

### Merging of controller code

1) Creating a new Informer and Lister for Applicationset CRD and Registering the Handler functions copying from application set code.

2) Creating new service for webhooks in applicationset

For Option 2 from above

Running as Microservice.

1) Implementing or removal of repeated codes
like ClusterUtils in applicationset
2) Exposing controller via service


### Security Considerations


Security improvements need to be taken care while merge
- Examine Logging of application set 
- May be Make Webhook events to be authenticated
- Application Set to emit kubernetes events same as argocd

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
