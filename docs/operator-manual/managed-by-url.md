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
    argocd.argoproj.io/managed-by-url: 'http://localhost:8081' # replace with actual secondary ArgoCD URL in real setup
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

| Field          | Value                               |
| -------------- | ----------------------------------- |
| **Annotation** | `argocd.argoproj.io/managed-by-url` |
| **Target**     | Application                         |
| **Value**      | Valid HTTP(S) URL                   |
| **Required**   | No                                  |

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

## Troubleshooting

### Links Still Point to Wrong Instance

**Check if the annotation is present:**

```bash
kubectl get application child-app -n instance-b -o jsonpath='{.metadata.annotations.argocd\.argoproj\.io/managed-by-url}'
```

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

## See Also

- [Application Annotations](../user-guide/annotations-and-labels.md)
- [App of Apps Pattern](cluster-bootstrapping.md)
- [Deep Links](deep_links.md)
