---
title: Cluster Name as ID
authors:
  - "@denysvitali" # Authors' github accounts here.
sponsors:
  - TBD        # List all interested parties here.
reviewers:
  - "@alexmt"
  - TBD
approvers:
  - "@alexmt"
  - TBD

creation-date: 2023-03-07
last-updated: 2023-03-07
---

# Cluster Name as unique identifier


<!-- ## Open Questions [optional] -->


## Summary

Currently, the clusters are identified by their API URL. Pretty much everywhere in the codebase, we assume that the API
URL is unique within an ArgoCD instance, and thus this is used as an identifier.  
With this proposal, we suggest to switch to a (Namespaced) Cluster Name oriented approach, so that one can use the same API URL multiple times
but with different service accounts / different namespaces.  
  
The result could be similar to [Traefik Service Names](https://doc.traefik.io/traefik/routing/services/) (e.g: `cluster-1@argo-cd`) to achieve namespaced unique identifiers / clusters.

## Motivation

There are [many different ways to achieve multi-tenancy in Kubernetes](https://kubernetes.io/blog/2021/04/15/three-tenancy-models-for-kubernetes/), mainly divided into "Cluster/Control Plane as a Service" and "Namespaces as a Service".  
  
For clarification purposes, we'll consider the following examples:
- One cluster per team, per environment (e.g: `https://team1-dev.kubernetes.example.com`, `default`)
- One cluster per team, namespaced (e.g: `https://team1.kubernetes.example.com`, with namespaces `dev`, `staging` and `prod`)
- One huge cluster, divided by namespaces (e.g: `https://kubernetes.example.com`, `team1-dev`, `team1-staging`, ...)
  
For this proposal, we're going to tackle an issue with the last two cases, assuming different credentials for each namespace. These last two cases can be seen as specific implementations of the "Namespace as a Service "case.
  
Consider the situation where a team is given three (or more) namespaces, for
example:

- `team1-dev`
- `team1-staging`
- `team1-prod`

Assume that for security purposes, Service Accounts are only allowed to act on one namespace.
This effectively means that in the example above `team-1` would have 3 service accounts to deploy to Kubernetes, one for each environment.  
  
With the current implementation of ArgoCD (2.6.3), it is not possible
without ugly workarounds (e.g: reverse proxy, subdomains), to support this use-case.

The problem is due to the fact that, in the case of multi-environment and multi-tenant clusters with different service accounts per environment, 
ArgoCD won't work reliably.  
  
Due to the current indexing by API URL (e.g: `https://shared-cluster.kubernetes.example.com`), one cannot use the following clusters:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: "cluster-a"
  namespace: argo-cd
  labels:
      argocd.argoproj.io/secret-type: cluster
type: Opaque
stringData:
  name: "cluster-a"
  namespaces: "team1-dev"
  server: "https://kubernetes.example.com"
  config: "{}" # tlsClientConfig for team1-dev
---
apiVersion: v1
kind: Secret
metadata:
  name: "cluster-b"
  namespace: argo-cd
  labels:
      argocd.argoproj.io/secret-type: cluster
type: Opaque
stringData:
  name: "cluster-b"
  namespaces: "team1-staging"
  server: "https://kubernetes.example.com"
  config: "{}" # tlsClientConfig for team1-staging
```

The issue that the user will experience is non-deterministic. Since the list of clusters
is indexed by the cluster API URL (`https://kubernetes.example.com`), some times ArgoCD will try
to deploy / verify the resources on the correct cluster (e.g: checking `team1-dev` with `cluster-a`), but other times it might use the correct credentials (e.g: checking `team1-dev` with `cluster-b`).

### Goals

- Users are able to use multiple "clusters" with the same API URL

### Non-Goals

<!-- TODO: Define Non-Goals ? -->

## Proposal

- Indexing is performed on `name@namespace` (for example `cluster-a@argo-cd`)
- The code only assumes the `name@namespace` to be unique
- The code assumes an API URL can be repeated

### Use cases


#### Use case 1:

As a user, I would use multiple clusters with the same API URL and different credentials

### Implementation Details/Notes/Constraints [optional]



### Detailed examples

### Security Considerations

N/A

### Risks and Mitigations


**Using the name as the identifier might cause problems in case a cluster with the same name is specified in another namespace.**

Mitigation: do what Traefik is doing, generate unique identifiers in the form `name@namespace`


### Upgrade / Downgrade Strategy

As we assume the `name@namespace` are unique within a cluster, and they're enforced by Kubernetes already (you can't have two resources of the same type in the same namespace), the upgrade strategy is pretty straight-forward.  
  
Since we're not breaking any functionality for the exsiting users,
downgrading would not cause any issues for those who are using ArgoCD
as it currently is as of v2.6.3.

## Drawbacks

None

## Alternatives

- Users can use a reverse proxy so that their API URL effectively becomes unique. This is an ugly solution but would work.