# ArgoCD Cache Invalidation Benchmark With Kind

This benchmark demonstrates the performance difference between full cache
invalidation and incremental namespace sync in ArgoCD. It install argocd in
namespaced mode watching 100 namespaces and 1 deployed app which should be
enough to demonstrate that a full cluster cache sync lasts around 50 seconds.

## Quick Start

The following tools are required:
- `kind`
- `kubectl`
- `docker/podman`
- `jq`

```bash
# 0. Build the image
make IMAGE
kind load docker-image localhost/argocd:latest -n argocd-perf-test

# 1. Then update benchmark/base/kustomization.yaml with your image location

# 2. Setup with feature DISABLED (slow cache invalidation)
./benchmark/setup-benchmark.sh

# 3. Access ArgoCD UI
# Open https://localhost:8080 (credentials shown in script output)

# 4. Trigger cache invalidation manually
# Go to Settings → Clusters → Click cluster → Invalidate Cache

# 5. Watch the logs and calcuate the sync duration (in another terminal)
kubectl logs -n argocd statefulset/argocd-application-controller -f

# 6. Add a new namespace which triggers the full cache invalidation again
# Wait 90 seconds for the results
./benchmark/add-namespace.sh

# 7. Compare with feature ENABLED
FEATURE_ENABLED=true ./benchmark/setup-benchmark.sh
# Since a controller restart triggers a full cluster cache sync as well you
# need to wait until is finished, then you can add a namespace
# This now should be fast because it incrementally adds a namespace and does
# not trigger a full cache invalidation
./benchmark/add-namespace.sh
```
