# Cluster Profile Controller

The ApplicationSet controller includes a ClusterProfile reconciler which automates the creation of cluster secrets from ClusterProfile objects. This allows for integration with the [Cluster Inventory API](https://github.com/kubernetes-sigs/cluster-inventory-api) to simplify management multiple clusters.

## Examples

When reading the information below, it may be helpful to have concrete examples. See the [kind](cluster-profiles-kind-example.md) or [GCP](cluster-profiles-gcp-example.md) examples for specific instructions on how you can use Cluster Profiles.

## How it works

### Cluster Profiles

The `ClusterProfileReconciler` is a controller that watches for `ClusterProfile` custom resources. For each `ClusterProfile` it finds, it creates a corresponding `Secret` object in the Argo CD namespace that can be used to connect to a remote cluster.

A `ClusterProfile` object looks like this:
```yaml
apiVersion: "multicluster.x-k8s.io/v1alpha1"
kind: ClusterProfile
metadata:
  name: my-cluster
  namespace: argocd
spec:
  clusterManager:
    name: my-cluster-profile-controller
  displayName: "My Cluster"
status:
  accessProviders:
  - name: my-provider
    cluster:
      server: https://my-cluster.example.com
```

You will need to have the `ClusterProfile` CRD from the [Cluster Inventory API](https://github.com/kubernetes-sigs/cluster-inventory-api) installed in your cluster:
```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-inventory-api/main/config/crd/bases/multicluster.x-k8s.io_clusterprofiles.yaml
```

The generated `Secret` will be labeled with `argocd.argoproj.io/secret-type: cluster` to identify it as a cluster secret, and `argocd.argoproj.io/cluster-profile-origin` with the `ClusterProfile` that it was generated from.

### Authentication

The Cluster Profile API is designed so that authentication information is kept separate from the ClusterProfile itself, in a file listing access providers with `execConfig`s specifying a command to run for authentication.

The access providers file would look something like this:
```json
{
  "providers": [
    {
      "name": "secretreader",
      "execConfig": {
          "apiVersion": "client.authentication.k8s.io/v1",
          "args": null,
          "command": "./bin/secretreader-plugin",
          "env": null,
          "provideClusterInfo": true
      }
    }
  ]
}
```

The ClusterProfile reconciler reads this file, finds an access provider whose name has a match in `ClusterProfile.Status.AccessProviders`, and uses it for the cluster connection config in the `Secret`.

To provide this file, the ApplicationSet controller should be configured with `--clusterProfileProvidersFile` and the file should be mounted through a Secret or ConfigMap ([example](cluster-profiles-kind-example.md)).

### Deletion

A finalizer (`argoproj.io/cluster-profile-finalizer`) is added to the `ClusterProfile` object. When it is deleted, the controller will clean up the corresponding `Secret`.

## Configuration

To enable the ClusterProfile controller, you need to start or patch the `argocd-applicationset-controller` with the `--cluster-profile-providers-file` argument. This argument should point to a file that contains the configuration for the access providers.

### Disabling the feature

The ClusterProfile controller is disabled by default. If you have enabled it and want to disable it, you need to remove the `--cluster-profile-providers-file` argument from the `argocd-applicationset-controller` command.