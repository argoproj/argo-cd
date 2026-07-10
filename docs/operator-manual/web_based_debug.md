# Web-based Debug (Ephemeral Containers)

Argo CD supports launching ephemeral debug containers on running pods directly from the UI, similar to `kubectl debug`. This is useful for troubleshooting distroless/scratch containers that have no shell, or workloads whose images don't bundle debugging tools.

## Requirements

- Kubernetes 1.25+ (ephemeral containers are GA since 1.25)
- The feature must be enabled in the `argocd-cm` ConfigMap
- Users must have the `debug` action on the `exec` resource
- The service account Argo CD uses for the destination cluster must be allowed to
  `patch` the `pods/ephemeralcontainers` subresource and `create` `pods/attach`. For the
  local (in-cluster) destination this is the `argocd-server` ClusterRole: `patch` is already
  covered by its cluster-wide rule, and the bundled install manifests add `pods/attach`.
  For externally-registered clusters, ensure the cluster's Argo CD credentials include these
  permissions.

## Configuration

### Enable the feature

In your `argocd-cm` ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  # Enable the web-based debug feature (disabled by default)
  exec.debug.enabled: "true"

  # Comma-separated list of allowed debug images (optional, defaults shown)
  exec.debug.images: "busybox:latest,alpine:latest,nicolaka/netshoot:latest"
```

> [!WARNING]
> Avoid mutable image tags (e.g. `:latest`) in `exec.debug.images`. The image is pulled at
> attach time, so a compromised upstream tag would land in a live pod without any Argo CD
> approval. Prefer pinning to a digest or an immutable tag.

### Configure RBAC

Debug access reuses the existing `exec` resource with a new `debug` action (rather than a
separate resource). Grant it to users or roles in `argocd-rbac-cm`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
  namespace: argocd
data:
  policy.csv: |
    # Allow a role to attach any allowlisted image to any application
    p, role:debugger, exec, debug, */*, allow

    # Restrict a subject to specific images using a glob suffix on the action.
    # The pattern is matched against the image reference and intersected with
    # the exec.debug.images allowlist (you cannot attach images not in the allowlist).
    p, role:net-debug, exec, debug/docker.io/library/busybox:*, */*, allow
    p, role:go-debug,  exec, debug/gcr.io/example/dlv:1.22, prod/*, allow
```

A bare `debug` action (no image suffix) is equivalent to `debug/*` for that subject. An
image-scoped grant of the form `debug/<image-glob>` only matches the requested image.

> [!NOTE]
> The image glob in the RBAC action is matched against the image reference exactly as it
> appears in `exec.debug.images`. Use the same reference form in both places — for example,
> a policy of `debug/docker.io/library/busybox:*` will not match an allowlist entry of
> `busybox:latest`.

The image dropdown shown in the UI is the intersection of the configured allowlist and the
images the user's RBAC grants.

## Usage

1. Navigate to an application in the Argo CD UI
2. Click on a running Pod in the resource tree
3. Select the **Debug** tab
4. Choose a debug image from the dropdown (images are restricted to the allowlist configured by the operator and to what your RBAC grants)
5. Optionally select a **Target Container** — this enables process namespace sharing so the debug container can see the target container's processes
6. Click **Start Debug Session**
7. An ephemeral container is created on the pod and a terminal session opens automatically
8. Click **End Session** when finished

When a workload has a debug container attached, its Application shows a note near the sync
status: `Sync OK (Debug Container Attached)`.

## How it works

When a debug session is started:

1. An ephemeral container with a unique name (e.g. `debug-abc123`) is added to the pod via the Kubernetes `pods/ephemeralcontainers` subresource
2. Argo CD waits for the container to reach the `Running` state (up to 2 minutes)
3. A WebSocket connection attaches stdin/stdout/stderr to the ephemeral container
4. The session is terminated when the user clicks "End Session" or closes the browser tab

## Security considerations

- Debug access is a `debug` action on the `exec` resource, so it can be granted independently of `exec, create`. Treat it as at least as privileged as `exec` — a debug container ships its own binaries (shells, package managers, network tools) into a live pod.
- Only images in the `exec.debug.images` allowlist can be used, and RBAC image globs can scope a subject tighter than the allowlist.
- Ephemeral containers share the pod's network namespace by default; selecting a target container also enables process namespace sharing, which exposes the target's memory and file descriptors (including in-memory secrets).
- Ephemeral containers cannot be removed once added; they remain until the pod is deleted.
