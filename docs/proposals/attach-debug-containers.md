---
title: Attach Debug Containers via ArgoCD UI
authors:
  - "@KyriosGN0" # Authors' github accounts here.
sponsors:
  - TBD        # List all interested parties here.
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2026-04-14
last-updated: 2026-04-14
---

# Attach Debug Containers via ArgoCD UI

This Proposal describes a feature to allow attaching debug containers to workloads via ArgoCD UI, to allow easier debugging of workloads with distroless images
or workloads whose images don't contain debugging tools



## Open Questions [optional]

* Should we provide a list of allowed debug images by default, or should we leave it empty and force users to pick their own images

## Summary

ArgoCD currently lacks a built-in way to attach debugging tools to running workloads. Users operating pods with distroless images, or images that don't bundle debugging utilities, must rely on external tooling (e.g., raw kubectl commands, ssh into the k8s node) to troubleshoot live issues — a workflow that is cumbersome and not accessible through the ArgoCD UI.
This proposal adds a Debug Containers tab to the ArgoCD pod view that lets users attach Kubernetes ephemeral containers to existing pods directly from the UI. Users can select a debug image and optionally share the target container's process — enabling tools like busybox, netshoot, or dlv to inspect a running workload. This closes the gap between ArgoCD's deployment visibility and the day-2 debugging experience, removing the need to drop out to kubectl debug for routine troubleshooting.


## Motivation

Allowing users to attach debug containers to pods is highly useful for debugging basic network/io problems in situations where a shell those tools are not available, or in some cases, where the workload cannot be exec'ed into.

### Goals

* Users can debug workloads that use distroless images.

### Non-Goals

.

## Proposal

ArgoCD will have new action `debug`, this action will be gated behind permission, and a flag in `argocd-cm` ConfigMap `debug.enabled`, another flag will be exposed `debug.images` this will be an array of images that will be allowed to be used in the debug container. when the `debug.enabled` flag is enabled and a user with the `debug` permission will click on a pod, it will have another tab called `debug` which will present him with a dropdown list of debug images, and a dropdown list for which (if any) container to target (for process sharing)

### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

#### Use case 1:
As a user, I would like to be able to debug workloads that are using distroless images


### Implementation Details/Notes/Constraints [optional]

see [PR](https://github.com/argoproj/argo-cd/pull/27124/changes)

### Detailed examples

### Security Considerations

* This Proposal and PR use the same terminal code as the exec feature
* users who configure allow images with `latest` as their tag in `debug.images` can be at risk for bad actors taking over popular images

### Risks and Mitigations

.


### Upgrade / Downgrade Strategy

.

## Drawbacks

ArgoCD might consider limiting attack surface that can be presented with with debug container (which are more powerful then exec'ing into a pod)

## Alternatives
.
