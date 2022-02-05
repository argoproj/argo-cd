# Redis HA Manifests

The Redis HA manifests are taken from the upstream helm chart, and tweaked slightly to add
Argo CD labels.  We also add roles to redis-ha service accounts to enable run-as non-root users
in OpenShift. The `overlays` is a directory containing kustomize overlays for Argo CD, namely label
modifications and role additions. To update redis version, update the kustomization.yaml with the
new version in `helmCharts`.
