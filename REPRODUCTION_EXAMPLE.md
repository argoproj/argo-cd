# SharedResourceWarning Bug Reproduction Example

## Issue #24477 - Working Example

### Problem Description
ArgoCD incorrectly triggers `SharedResourceWarning` for resources with the same name deployed in different clusters.

### Reproduction Steps

#### 1. Setup Multi-Cluster Environment

**Cluster A Configuration:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
data:
  application.resourceTrackingMethod: annotation
  application.instanceLabelKey: argocd.argoproj.io/instance
```

**Cluster B Configuration:**
```yaml
apiVersion: v1
kind: ConfigMap  
metadata:
  name: argocd-cm
data:
  application.resourceTrackingMethod: annotation
  application.instanceLabelKey: argocd.argoproj.io/instance
```

#### 2. Deploy Same Resource to Both Clusters

**Application A (Cluster A):**
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app-cluster-a
  namespace: argocd
spec:
  destination:
    server: https://cluster-a.example.com
    namespace: default
  source:
    repoURL: https://github.com/example/test-app
    path: manifests
    targetRevision: main
```

**Application B (Cluster B):**
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app-cluster-b
  namespace: argocd
spec:
  destination:
    server: https://cluster-b.example.com  
    namespace: default
  source:
    repoURL: https://github.com/example/test-app
    path: manifests
    targetRevision: main
```

**Deployed Resource (same in both clusters):**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.20
```

#### 3. Observe the Bug

**Before Fix:**
- Both applications show `SharedResourceWarning`
- Warning message: "apps/Deployment nginx-deployment is part of applications test-app-cluster-a and test-app-cluster-b"
- This is incorrect because they are in different clusters

**Resource State in Cluster A:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  namespace: default
  uid: 62e7a834-97c6-4a99-8abf-8bbcb1dec995
  annotations:
    argocd.argoproj.io/tracking-id: test-app-cluster-a:apps/Deployment:default/nginx-deployment
```

**Resource State in Cluster B:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  namespace: default
  uid: 39399317-0fef-4770-beda-516d9c62b24d
  annotations:
    argocd.argoproj.io/tracking-id: test-app-cluster-b:apps/Deployment:default/nginx-deployment
```

#### 4. Verification

**Key Differences (proving they are different resources):**
- Different UIDs: `62e7a834-97c6-4a99-8abf-8bbcb1dec995` vs `39399317-0fef-4770-beda-516d9c62b24d`
- Different tracking IDs: `test-app-cluster-a:...` vs `test-app-cluster-b:...`
- Different cluster contexts

### After Fix

**Expected Behavior:**
- No `SharedResourceWarning` for cross-cluster resources
- Warning only triggered for genuinely shared resources within the same cluster
- All tracking methods supported (annotation, annotation+label, label)

### Test Commands

```bash
# Check application status
kubectl get applications -n argocd

# Check for SharedResourceWarning conditions
kubectl get application test-app-cluster-a -n argocd -o jsonpath='{.status.conditions[?(@.type=="SharedResourceWarning")]}'

# Verify resource UIDs in different clusters
kubectl get deployment nginx-deployment -o jsonpath='{.metadata.uid}' --context=cluster-a
kubectl get deployment nginx-deployment -o jsonpath='{.metadata.uid}' --context=cluster-b
```

This example demonstrates the exact scenario where the bug occurs and can be used to verify the fix.
