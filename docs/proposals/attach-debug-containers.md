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

* Debug distroless pods, or pods missing debug tools, from ArgoCD UI.
* Network debugging from the UI (e.g. attach `netshoot` to check connectivity, DNS, traffic).
* Attach profilers/debuggers (`dlv`, `py-spy`) via shared PID namespace.
* Debug pods where `exec` won't work — distroless without a shell, crash-looping pods, pods whose entrypoint already exited.

### Non-Goals

This proposal does not cover managing the lifecycle of already-attached ephemeral containers (Kubernetes does not support removing them)

## Proposal

Rather than introducing a new RBAC resource, this proposal uses on the existing `exec` resource by adding a new action `debug`. The feature is gated behind a `exec.debug.enabled` flag in the `argocd-cm` ConfigMap (disabled by default). When enabled, a user with the `debug` action on the pod's project sees a new `Debug` tab on the pod view, with a dropdown of allowed debug images and a dropdown of the pod's containers for optional process-namespace sharing — enabling tools like busybox, netshoot, or dlv to inspect a running workload.

A `exec.debug.images` field is also exposed in `argocd-cm` as the cluster-wide default allowlist of images that may be used.

### RBAC

The simple form grants the user the ability to attach any image permitted by `exec.debug.images`:

```
p, role:debugger, exec, debug, */*, allow
```

The action can be extended with an image pattern to further narrow which images a specific subject is allowed to attach (you cannot add images not present in exec.debug.images). Glob matching is applied against the fully-qualified image reference, and the result is intersected with the configured allowlist:

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

* **Bigger attack surface than `exec`.** Debug container ships its own binaries — shells, package managers, net tools the workload image left out. `debug` grant = injecting arbitrary tooling into a live pod. Mitigation: off by default (`exec.debug.enabled`). Images limited to the `exec.debug.images` allowlist, RBAC supports per-image globs so a subject can be scoped tighter than the allowlist.
* **Mutable image tags.** Allowlisting `busybox:latest` means the image pulled at attach time can change without ArgoCD approval — compromised upstream tag lands in prod. Mitigation: No mitigation in Argo itself, but docs will show a warning againts using mutable image tags.
* **Resource exhaustion from a bad debug container.** Ephemeral containers share the pod's cgroup, and Kubernetes does not allow `resources` on ephemeral containers — so a profiler, packet capture, or fork bomb can't be bounded the way a sidecar can, and may starve the workload or fill node ephemeral storage. Mitigation: can't fix at the container level. Treat `exec.debug.images` as a trusted-image list and `debug` as a privileged grant on par with `exec`. Node-level protections (kubelet eviction thresholds, namespace ResourceQuotas on ephemeral storage) stay the operator's job.
* **PID namespace sharing exposes the target's memory and FDs.** With shared PID namespace, the debug container can read `/proc/<pid>/mem`, `ptrace`, and see the workload's file descriptors — including in-memory secrets. Mitigation: opt-in per attach; audit log captures image + target container; same RBAC as `exec`.
* **Debug history sticks around.** Kubernetes won't let you remove an ephemeral container once attached — the pod spec keeps the record until the pod is replaced. Called out in Non-Goals; UI shows `Sync OK (Debug Container Attached)` so the state isn't hidden.

### Upgrade / Downgrade Strategy

* **Upgrade.** Gated by `exec.debug.enabled` in `argocd-cm`, default `false` — upgrade is a no-op until an operator opts in. On opt-in: populate `exec.debug.images` and grant `exec, debug` to the relevant roles. Until then the Debug tab is hidden and the API rejects attach requests.
* **Downgrade.** Older ArgoCD versions drop the Debug tab and the `exec, debug` endpoint. Already-attached ephemeral containers keep running — they're owned by Kubernetes, not ArgoCD — and stay visible via `kubectl`. No ArgoCD-side cleanup needed. The new ConfigMap keys are ignored by older versions; leave them in place for a future re-upgrade or remove them, either is fine. RBAC policies referencing `debug` are inert on older versions.

## Drawbacks

Argo CD might consider limiting the attack surface that can be presented with the debug container (which is more powerful than exec'ing into a pod)

## Alternatives

* **`kubectl debug` against the cluster.** The status quo. Needs direct cluster creds and bypasses ArgoCD RBAC + audit — which is the reason teams that live in the ArgoCD UI want this in-product.
* **`kubectl exec` (already in ArgoCD).** Fine when the image has a shell and the right tools. No good for distroless, crash-looping pods, or attaching a profiler to a running PID from outside.
* **eBPF tooling.** Good for always-on, low-overhead network and syscall introspection across the cluster. Separate platform to run, needs cluster-wide privileges, and doesn't cover interactive use (attach a debugger to a PID, run ad-hoc commands in the pod's namespaces). Complements debug containers, doesn't replace them.
