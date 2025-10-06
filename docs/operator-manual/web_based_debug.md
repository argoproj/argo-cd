# Web-based Pod Debug

![Argo CD Debug](../assets/debug.png)

Since v2.13, Argo CD has a web-based pod debug feature that allows you to create debug containers inside running pods just like you would with `kubectl debug`. This feature uses Kubernetes ephemeral containers to provide debugging capabilities without affecting the running application.

This is a powerful debugging tool. It allows users to run debugging containers with additional tools and utilities inside application pods for troubleshooting purposes.

## Enabling the debug feature

The debug feature is enabled by default and doesn't require additional configuration like the terminal feature. However, you need to ensure proper RBAC permissions are in place.

### RBAC Configuration

1. Patch the `argocd-server` Role (if using namespaced Argo) or ClusterRole (if using clustered Argo) to allow `argocd-server` to update ephemeral containers:

   ```yaml
   - apiGroups:
     - ""
     resources:
     - pods/ephemeralcontainers
     verbs:
     - update
   ```

   If you'd like to perform the patch imperatively, you can use the following command:

   - For namespaced Argo:
     ```
     kubectl patch role <argocd-server-role-name> -n argocd --type='json' -p='[{"op": "add", "path": "/rules/-", "value": {"apiGroups": [""], "resources": ["pods/ephemeralcontainers"], "verbs": ["update"]}}]'
     ```
   - For clustered Argo:
     ```
     kubectl patch clusterrole <argocd-server-clusterrole-name> --type='json' -p='[{"op": "add", "path": "/rules/-", "value": {"apiGroups": [""], "resources": ["pods/ephemeralcontainers"], "verbs": ["update"]}}]'
     ```

2. Add RBAC rules to allow your users to `create` the `exec` resource (debug uses the same permissions as terminal for now):

   ```csv
   p, <your-user-or-role>, exec, create, <project>/<application>, allow
   ```

   Or grant blanket access to all applications:

   ```csv
   p, <your-user-or-role>, exec, create, *, allow
   ```

## Using the debug feature

1. Navigate to your application in the Argo CD UI
2. Click on a pod resource
3. Click on the "DEBUG" tab (🐛 icon)
4. Configure your debug session:
   - **Debug Image**: Container image to use for debugging (e.g., `busybox:1.28`, `ubuntu:20.04`, `nicolaka/netshoot`)
   - **Command**: Command to run in the debug container (default: `sh`)
   - **Share Process Namespace**: Enable to see processes from the target container
5. Click "Start Debug Session"
6. Use the terminal interface to debug your application

## Popular debug images

- **`busybox:1.28`**: Minimal Unix utilities
- **`ubuntu:20.04`**: Full Ubuntu environment with package manager
- **`nicolaka/netshoot`**: Network debugging tools (tcpdump, netstat, nslookup, etc.)
- **`alpine:latest`**: Minimal Alpine Linux with package manager

## Debug session features

- **Process namespace sharing**: When enabled, debugging tools can see and interact with processes from the target container
- **Full terminal support**: Complete xterm.js terminal with ANSI color support
- **Session management**: Start, stop, and reconfigure debug sessions
- **No application impact**: Debug containers run alongside your application without affecting it

## Security considerations

- Debug containers have the same network and storage access as the target pod
- When process namespace sharing is enabled, debug containers can see all processes in the pod
- Debug containers can potentially access sensitive data mounted in the pod
- Consider using minimal debug images and limiting debug access through RBAC

## Limitations

- Requires Kubernetes 1.23+ (ephemeral containers feature)
- Debug containers cannot be removed until the pod is deleted
- Some container runtimes may have limitations with ephemeral containers
- Process namespace sharing requires appropriate security context capabilities

## Troubleshooting

### Debug container fails to start

- Ensure the debug image exists and is accessible
- Check that the pod is in Running state
- Verify RBAC permissions for `pods/ephemeralcontainers`
- Check Kubernetes version (1.23+ required)

### Cannot see target container processes

- Enable "Share process namespace" option
- Ensure the debug image has process inspection tools (`ps`, `top`, etc.)
- Some security contexts may prevent process namespace sharing

### Connection errors

- Check network connectivity to the Argo CD server
- Verify WebSocket connections are allowed through firewalls/proxies
- Ensure proper authentication and session management
