# Redis HA Manifests

The Redis HA manifests are taken from the upstream helm chart, and tweaked slightly to add
Argo CD labels.
* `chart` is a helm chart that references the upstream redis-ha chart. To update redis, update the
  version in `chart/requirements.yaml` with a later version of the chart.
* `overlays` is a directory containing kustomize overlays for Argo CD, namely label modifications
* `generate.sh` is a script to regenerate the final kustomize 
