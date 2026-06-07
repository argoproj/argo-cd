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
last-updated: 2026-06-06
---

# Attach Debug Containers via ArgoCD UI

This Proposal describes a feature to allow attaching debug containers to workloads via ArgoCD UI, to allow easier debugging of workloads with distroless images
or workloads whose images don't contain debugging tools


## Summary

ArgoCD currently lacks a built-in way to attach debugging tools to running workloads. Users operating pods with distroless images, or images that don't bundle debugging utilities, must rely on external tooling (e.g., raw kubectl commands, ssh into the k8s node) to troubleshoot live issues — a workflow that is cumbersome and not accessible through the ArgoCD UI.
This proposal adds a Debug Containers tab to the ArgoCD pod view that lets users attach Kubernetes ephemeral containers to existing pods directly from the UI. Users can select a debug image and optionally share the target container's process — enabling tools like busybox, netshoot, or dlv to inspect a running workload. This closes the gap between ArgoCD's deployment visibility and the day-2 debugging experience, removing the need to drop out to kubectl debug for routine troubleshooting.


## Motivation

Allowing users to attach debug containers to pods is highly useful for debugging basic network/io problems in situations where a shell those tools are not available, or in some cases, where the workload cannot be exec'ed into.

### Goals

* Users can debug workloads that use distroless images.

### Non-Goals

This proposal does not cover managing the lifecycle of already-attached ephemeral containers (Kubernetes does not support removing them)

## Proposal

Rather than introducing a new RBAC resource, this proposal uses on the existing `exec` resource by adding a new action `debug`. The feature is gated behind a `exec.debug.enabled` flag in the `argocd-cm` ConfigMap (disabled by default). When enabled, a user with the `debug` action on the pod's project sees a new `Debug` tab on the pod view, with a dropdown of allowed debug images and a dropdown of the pod's containers for optional process-namespace sharing — enabling tools like busybox, netshoot, or dlv to inspect a running workload.

A `exec.debug.images` field is also exposed in `argocd-cm` as the cluster-wide default allowlist of images that may be used. The same field is exposed on the AppProject CRD as `debugImages`; when set, it overrides `debug.images` for applications in that project.

### RBAC

The simple form grants the user the ability to attach any image permitted by `exec.debug.images` / `debugImages`:

```
p, role:debugger, exec, debug, */*, allow
```

The action can be extended with an image pattern to further narrow which images a specific subject is allowed to attach. Glob matching is applied against the fully-qualified image reference, and the result is intersected with the configured allowlist:

```
p, role:net-debug, exec, debug/docker.io/library/busybox:*, */*, allow
p, role:go-debug,  exec, debug/gcr.io/example/dlv:1.22, prod/*, allow
```

A bare `debug` action (no image suffix) is equivalent to `debug/*` for that subject. The image dropdown shown in the UI is the intersection of the configured allowlist and the patterns the user's RBAC grants.

When a workload has a debug container attached, its Application shows a note near the Sync OK text: `Sync OK (Debug Container Attached)`.

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
