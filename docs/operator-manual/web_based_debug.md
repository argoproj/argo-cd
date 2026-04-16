# Web-based Debug (Ephemeral Containers)

Argo CD supports launching ephemeral debug containers on running pods directly from the UI, similar to `kubectl debug`. This is useful for troubleshooting distroless/scratch containers that have no shell.

## Requirements

- Kubernetes 1.25+ (ephemeral containers are GA since 1.25)
- The feature must be enabled in the `argocd-cm` ConfigMap
- Users must have the `debug:create` RBAC permission

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
  # Enable the web-based debug feature
  debug.enabled: "true"

  # Comma-separated list of allowed debug images (optional, defaults shown)
  debug.images: "busybox:latest,alpine:latest,nicolaka/netshoot:latest"
```

### Configure RBAC

Grant the `debug:create` permission to users or roles in `argocd-rbac-cm`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
  namespace: argocd
data:
  policy.csv: |
    # Allow admins to debug any application
    p, role:admin, debug, create, */*, allow

    # Allow specific users to debug in a project
    p, alice, debug, create, my-project/*, allow
```

## Usage

1. Navigate to an application in the Argo CD UI
2. Click on a running Pod in the resource tree
3. Select the **Debug** tab
4. Choose a debug image from the dropdown (images are restricted to the allowlist configured by the operator)
5. Optionally select a **Target Container** — this enables process namespace sharing so the debug container can see the target container's processes
6. Click **Start Debug Session**
7. An ephemeral container is created on the pod and a terminal session opens automatically
8. Click **End Session** when finished

## How it works

When a debug session is started:

1. An ephemeral container with a unique name (e.g. `debug-abc123`) is added to the pod via the Kubernetes `pods/ephemeralcontainers` subresource
2. Argo CD waits for the container to reach the `Running` state (up to 2 minutes)
3. A WebSocket connection attaches stdin/stdout/stderr to the ephemeral container
4. The session is terminated when the user clicks "End Session" or closes the browser tab

## Security considerations

- The `debug` resource requires a separate RBAC permission from `exec`, allowing operators to restrict debug access independently
- Only images in the `debug.images` allowlist can be used, preventing use of arbitrary images
- Ephemeral containers share the pod's network namespace by default; selecting a target container also enables process namespace sharing
- Ephemeral containers cannot be removed once added; they will be cleaned up when the pod is deleted
