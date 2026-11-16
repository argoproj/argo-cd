# Web-based Debug (Ephemeral Containers)

Argo CD supports launching ephemeral debug containers on running pods directly from the UI, similar to `kubectl debug`. This is useful for troubleshooting distroless/scratch containers that have no shell, or workloads whose images don't bundle debugging tools.

## Requirements

- Kubernetes 1.25+ (ephemeral containers are GA since 1.25)
- The feature must be enabled in the `argocd-cm` ConfigMap
- Users must have an image-scoped `debug/<image>` action on the `exec` resource
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

  # Comma-separated allowlist of debug images. Required when the feature is enabled:
  # there is no default, and the feature is unavailable until this is set (example values below).
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
    p, role:debugger, exec, debug/*, */*, allow

    # Restrict a subject to specific images using a glob suffix on the action.
    # The pattern is matched against the image reference and intersected with
    # the exec.debug.images allowlist (you cannot attach images not in the allowlist).
    p, role:net-debug, exec, debug/docker.io/library/busybox:*, */*, allow
    p, role:go-debug,  exec, debug/gcr.io/example/dlv:1.22, prod/*, allow
```

The action must always carry an image glob: `debug/*` allows any allowlisted image, and
`debug/<image-glob>` only matches the requested image. A bare `debug` action grants nothing, and
`argocd admin settings rbac validate` rejects it. Because each request is checked against a single
`debug/<image>` action, an image-scoped `deny` overrides a broader `allow`:

```csv
p, role:dev, exec, debug/*, */*, allow
p, role:dev, exec, debug/docker.io/nicolaka/netshoot:*, */*, deny
```

> [!WARNING]
> A wildcard action on `exec` (for example `p, role:x, exec, *, */*, allow`, including in
> AppProject roles) also matches `debug/<image>`, so it grants debug access once
> `exec.debug.enabled` is `true`. Grant `exec, create` explicitly if a role should only get the
> web terminal.

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
5. Optionally select a **Target Container** — this enables process namespace sharing so the debug container can see the target container's processes. With no target, the debug container shares no process namespace (unless the pod already sets `shareProcessNamespace`)
6. Click **Start Debug Session**
7. An ephemeral container is created on the pod and a terminal session opens automatically
8. Click **End Session** when finished

While a debug container is running in a workload's pod, its Application shows a note near the
sync status: `Sync OK (Debug Container Attached)`. The note clears once the debug container exits.

## How it works

When a debug session is started:

1. The browser opens a WebSocket session with the Argo CD API server
2. An ephemeral container with a unique name (e.g. `argocd-debug-abc123`) is added to the pod via the Kubernetes `pods/ephemeralcontainers` subresource
3. Argo CD waits for the container to reach the `Running` state (up to 2 minutes), failing early if the image cannot be pulled or the container cannot be created
4. The WebSocket session attaches stdin/stdout/stderr to the ephemeral container
5. The session is terminated when the user clicks "End Session" or closes the browser tab. The container is started with `stdinOnce`, so a shell-based debug image exits once the session detaches

## Known limitations

- Ephemeral containers cannot be removed once added. If a session fails after the container was
  added (for example the image pull is slow and the 2-minute wait times out, or the attach fails),
  the container may still start later and remain idle in the pod until the pod is replaced. Each
  new session adds another container.
- A debug session cannot be resumed. When the Argo CD session token is refreshed or fails
  verification during a debug session, the session ends and a new one must be started, which adds
  a new ephemeral container.

## Security considerations

- Debug access is a `debug/<image>` action on the `exec` resource, so it can be granted independently of `exec, create`. Treat it as at least as privileged as `exec` — a debug container ships its own binaries (shells, package managers, network tools) into a live pod.
- Only images in the `exec.debug.images` allowlist can be used, and RBAC image globs can scope a subject tighter than the allowlist.
- Ephemeral containers share the pod's network namespace by default; selecting a target container also enables process namespace sharing, which exposes the target's memory and file descriptors (including in-memory secrets).
- Ephemeral containers cannot be removed once added; they remain until the pod is deleted.
- Debug session logs include the client source IP when source IP logging is enabled for the API server, as for the web terminal.
