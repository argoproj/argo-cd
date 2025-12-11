# Managed By URL Annotation

## Overview

The `argocd.argoproj.io/managed-by-url` annotation allows an Application resource to specify which Argo CD instance manages it. This is useful when you have multiple Argo CD instances and need application links in the UI to point to the correct managing instance.

## Use Case

When using multiple Argo CD instances with the [app-of-apps pattern](cluster-bootstrapping.md):

- A primary Argo CD instance creates a parent Application
- The parent Application deploys child Applications that are managed by a secondary Argo CD instance
- Without the annotation, clicking on child Applications in the primary instance's UI tries to open them in the primary instance (incorrect)
- With the annotation, child Applications correctly open in the secondary instance

The `managed-by-url` annotation ensures application links redirect to the correct Argo CD instance.

> [!NOTE]
> This annotation is particularly useful in multi-tenant setups where different teams have their own Argo CD instances, or in hub-and-spoke architectures where a central instance manages multiple edge instances.

## Example

This example demonstrates the [app-of-apps pattern](cluster-bootstrapping.md) where a parent Application deploys child Applications from a Git repository.

### Step 1: Create Parent Application

Create a parent Application in your primary Argo CD instance:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: parent-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/YOUR-ORG/my-apps-repo.git
    targetRevision: main
    path: path-to-child-app
  destination:
    server: https://kubernetes.default.svc
    namespace: namespace-b
  syncPolicy:
    automated:
      selfHeal: true
      prune: true
```

### Step 2: Create Child Application in Git Repository

In your Git repository at `apps/child-apps/child-app.yaml`, add the `managed-by-url` annotation:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: child-app
  namespace: namespace-b
  annotations:
    argocd.argoproj.io/managed-by-url: "http://localhost:8081" # replace with actual secondary ArgoCD URL in real setup
spec:
  project: default
  source:
    repoURL: https://github.com/YOUR-ORG/my-apps-repo.git
    targetRevision: HEAD
    path: path-to-child-app
  destination:
    server: https://kubernetes.default.svc
    namespace: namespace-b
  syncPolicy:
    automated:
      selfHeal: true
      prune: true
```

### Result

When viewing the parent Application in the primary instance's UI:
- The parent Application syncs from Git and deploys the child Application
- Clicking on `child-app` in the resource tree navigates to `https://secondary-argocd.example.com/applications/namespace-b/child-app`
- The link opens the child Application in the correct Argo CD instance that actually manages it

## Configuration

### Annotation Format

| Field | Value |
|-------|-------|
| **Annotation** | `argocd.argoproj.io/managed-by-url` |
| **Target** | Application |
| **Value** | Valid HTTP(S) URL |
| **Required** | No |

### URL Validation

The annotation value **must** be a valid HTTP(S) URL:

- ✅ `https://argocd.example.com`
- ✅ `https://argocd.example.com:8080`
- ✅ `http://localhost:8080` (for development)
- ❌ `argocd.example.com` (missing protocol)
- ❌ `javascript:alert(1)` (invalid protocol)

Invalid URLs will prevent the Application from being created or updated.

### Behavior

When generating application links, Argo CD:
- **Without annotation**: Uses the current instance's base URL
- **With annotation**: Uses the URL from the annotation
- **Invalid annotation**: Falls back to the current instance's base URL and logs a warning

> [!WARNING]
> Ensure the URL in the annotation is accessible from users' browsers. For internal deployments, use internal DNS names or configure appropriate network access.

## Testing Locally

To test the annotation with two local Argo CD instances:

```bash
# Install primary instance
kubectl create namespace argocd
kubectl apply -n argocd --server-side -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# Install secondary instance
kubectl create namespace namespace-b
kubectl apply -n namespace-b --server-side -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# Port forward both instances
kubectl port-forward -n argocd svc/argocd-server 8080:443 &
kubectl port-forward -n namespace-b svc/argocd-server 8081:443 &

# Wait for Argo CD to be ready
kubectl wait --for=condition=available --timeout=300s deployment/argocd-server -n argocd

# Get the admin password for primary instance
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d && echo

```

Then:
1. Open `http://localhost:8080` in your browser
2. Login with username `admin` and the password from the command above
3. Navigate to the `parent-app` Application
4. Click on the `child-app` in the resource tree
5. It should redirect to `http://localhost:8081/applications/namespace-b/child-app`

You will need to repeat the command to get the password for the secondary instance to login and access the child-app

```bash
# Get the admin password for secondary instance
kubectl -n namespace-b get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d && echo
```

## Troubleshooting

### Links Still Point to Wrong Instance

**Check if the annotation is present:**

```bash
kubectl get application child-app -n instance-b -o jsonpath='{.metadata.annotations.argocd\.argoproj\.io/managed-by-url}'
```

Expected output: A complete URL like `http://localhost:8081` or the url that has been set 
i.e `https://secondary-argocd.example.com`

**If the annotation is present but links still don't work:**
- Verify the URL is accessible from your browser
- Check browser console for errors
- Ensure the URL format is correct (includes `http://` or `https://`)

### Application Creation Fails

If Application creation fails with "invalid managed-by URL" error:

- ✅ URL includes protocol (`https://` or `http://`)
- ✅ URL contains no typos
- ✅ URL uses only valid characters
- ✅ URL is not a potentially malicious scheme (e.g., `javascript:`)

### Nested Applications Not Working

For app-of-apps patterns, ensure:
1. The child Application YAML in Git includes the annotation
2. The parent Application has synced successfully
3. The child Application has been created in the cluster

Verify the child Application exists:

```bash
kubectl get application CHILD-APP-NAME -n NAMESPACE
```

## See Also

- [Application Annotations](../user-guide/annotations-and-labels.md)
- [App of Apps Pattern](cluster-bootstrapping.md)
- [Deep Links](deep_links.md)
