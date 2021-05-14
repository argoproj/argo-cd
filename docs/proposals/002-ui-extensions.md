---
title: Argo CD UI Extensions
authors:
  - "@rbreeze"
  - "@jsuen"
sponsors:
  - TBD
reviewers:
  - "@alexmt"
  - "@jsuen"
  - TBD
approvers:
  - TBD

creation-date: 2021-05-13
last-updated: 2021-05-13
---

# UI Extensions

## Open Questions

[ ] Extensions with more sophisticated API endpoints (e.g. watch)


## Summary

This proposal is to provide a mechanism in Argo CD to extend the API and UI, such that resource-specific and context-sensitive UI components could be displayed in the user interface. The extension could be configured by operators at runtime (without a feature being built directly into Argo CD).

## Motivation

The Argo CD API server and UI cannot (and will likely never be able to) present information about application resources in a sophisticated manner that would be of most use to the end-user. This is because aside from native Kubernetes kinds, Argo CD simply does not have deep understanding of other resources (i.e. CRDs), what information would be useful to extract from the object, and how to visualize that information.

### Goals

- Enable new visualizations in the UI for Resources that do not have baked-in support
- Extensions should be easy to develop and even easier to install

### Non-Goals

- We do not want to require operators to recompile UI code to enable new Extensions
- We do not want tight coupling between Argo CD and certain versions of Extensions

## Proposal

In the UI, a new tab in the Resource View will be made available. The contents of that tab should be served by the underlying Extension's API service, proxied through the Argo CD API server.

Additional changes are required in the Argo CD API server. Similar to Kubernetes' API Service resource, an ArgoCDExtension Custom Resource will reserve an API path in the Argo CD server to proxy requests to an underlying service. The API service will serve both additional API endpoints, as well as UI assets needed to display the custom Extension.

The remote service proxy will use `kubectl port-forward` style of proxy. The local cluster proxy will use a simple reverse proxy (e.g. using Golang network reverse proxy).

### Use cases

## Use case 1: 
As a user, I would like to see visual information about my Rollout without having to use the CLI or otherwise leave Argo CD.

## Use case 2: 
As a user, I would like to see visual information about my Workflow without having to use the CLI or otherwise leave Argo CD.

### Implementation Details/Notes/Constraints [optional]

The UI will dynamically import an Extension React component from the Argo CD API Server. This is accomplished by specifying the generic Extension component as a Webpack external, and including a `<script>` tag in the `index.html` template that refers to the Argo CD API Server's generic extension endpoint (i.e. `/api/v1/extensions`). The API Server serves a different instantiation of the generic Extension component depending on the Resource being displayed; the generic extensions endpoint will have intelligence that reverse proxies the relevant third-party Extension API. The third-party Extension itself must conform to certain standards for this dynamic import (i.e. it must not bundle React). 

A new Custom Resource must also be introduced to teach the Argo CD API Server where to point its reverse proxy for which Resources. This CR will look similar to a Kubernetes [APIService CR](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-extension-api-server/): 

```
metadata:
  name: rollout-extension
spec:
  group: argoproj.io
  kind: Rollout
  service:
    remote: true  # indicates it will proxy to the remote/managed cluster (vs. the Argo CD cluster)
    namespace: argo-rollouts
    name: rollouts-dashboard
    port: 3100
    base_url: /api/v1

  actions:
    promote.lua: |
      # lua code  
    promote-full.lua: |
      # lua code  

  discovery.lua: |
    # lua code

  health.lua: |
    # lua code goes here

```

### Detailed examples

#### Argo Rollout Extension PoC: 

![Rollout Extension](./rollout-extension.png)

### Security Considerations

- Extensions will only support read operations to the proxy
- Any write operations must be configured as Lua scripts defined in the ArgoCDExtension Custom Resource so that Argo CD RBAC can be enforced when a user invokes an action
- May introduce allow/deny list of endpoint paths which the Argo CD API Server is allowed to proxy (in case there are endpoints which an operator does not want to make available to the proxy)
- Need ability to configure certificate in ArgoCDExtension CR to prevent MiM attacks

### Risks and Mitigations

We will be allowing the Argo CD UI to serve dynamically imported UI assets; while these dynamic imports will only occur from same-origin, malicious Extensions may inject hazardous code. We will mitigate the damage that malicious Extensions could cause by restricting Extensions API proxy calls to read-only, but further consideration here is required. We may also consider publishing a list of "sanctioned" or "approved" Extensions that we believe to be trustworthy (e.g. Argo Rollouts' or Workflows' Extensions).


### Upgrade / Downgrade Strategy

Existing Argo CD instances should be unaffected by this change. Extensions are opt-in only, and ideally none should be installed by default. 

To opt in, operators will need to install services that comply with the Argo CD Extensions API and expose that service such that it is reachable by the Argo CD API Server. To uninstall an extension should be as simple as removing the service that provides the Extension assets and API - the Argo CD API Server should fail gracefully when an ArgoCDExtension CR exists that defines an Extension which is no longer installed.

## Drawbacks

Argo CD was designed to be a GitOps tool, not a cluster visualization dashboard. Extensions open the door to increase Argo CD's scope in a way that may not be desirable.

## Alternatives

We originally considered building native support for resources like a Rollout directly into Argo CD. However, this tightly couples the Argo CD Server to an Argo Rollouts version, which is problematic when Argo CD manages several clusters all running different Rollouts versions.

We additionally considered requiring recompilation of the Argo CD UI (and by extension, the API server) to install Extensions in a similar fashion to Config Management Plugins. However, this is a headache for operators, and given that we are in the process of improving the Config Management Plugin paradigm, we should not go down this path if possible. 
