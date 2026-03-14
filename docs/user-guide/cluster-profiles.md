# Cluster Profile Controller

The Cluster Profile controller automates the creation of cluster secrets from ClusterProfile` objects. This allows for integration with the [Cluster Inventory API](https://github.com/kubernetes-sigs/cluster-inventory-api) to simplify management multiple clusters.

## Examples

When reading the information below, it may be helpful to have concrete examples. See the [kind](cluster-profiles-kind-example.md) or [GCP](cluster-profiles-gcp-example.md) examples for specifics on how the controller can be used.

## How it works

### Cluster Profiles

The Cluster Profile controller watches for `ClusterProfile` custom resources. For each `ClusterProfile` it finds, it creates a corresponding `Secret` object in the Argo CD namespace that can be used to connect to a remote cluster. The controller will also update the `ClusterProfile`'s status with the `Secret`'s name and namespace.

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

They may be synced automatically by a cluster manager or created manually. To manually create Cluster Profiles, the `ClusterProfile` CRD from the [Cluster Inventory API](https://github.com/kubernetes-sigs/cluster-inventory-api) must be installed in your cluster:
```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-inventory-api/main/config/crd/bases/multicluster.x-k8s.io_clusterprofiles.yaml
```

To enable the Cluster Profile controller, you must set `clusterprofilecontroller.enabled: "true"` in the `argocd-cmd-params-cm` ConfigMap. Otherwise, the controller will only log "ClusterProfile controller is disabled." If enabled, it will start watching for `ClusterProfile` objects and generating `Secret`s. The generated `Secret`s will be labeled with `argocd.argoproj.io/secret-type: cluster` to identify them as cluster secrets, and `argocd.argoproj.io/cluster-profile-origin` with the name of the `ClusterProfile` that they were generated from.

### Authentication

These secrets are used by the Argo CD Application controller to authenticate to remote clusters. The Cluster Profile controller can generate these secrets with one of two authentication methods: using built-in cloud provider authentication, or using a custom access providers file.

#### Built-in Cloud Provider Authentication

If you are using a supported cloud provider (such as GCP), the Cluster Profile controller can generate a secret that uses the `argocd-k8s-auth` command to authenticate to the remote cluster.

To use this feature, the access provider name in the `ClusterProfile`'s status must start with `argo-cd-builtin-` followed by the provider's name (e.g., `argo-cd-builtin-gcp`). When the controller encounters an access provider with this prefix, it will automatically configure the generated Argo CD secret to use the `argocd-k8s-auth <provider>` command for authentication. The supported provider names are `gcp`, `aws`, and `azure`, corresponding to commands in `cmd/argocd-k8s-auth/commands/`. See the [GCP example](cluster-profiles-gcp-example.md) for more.

#### Custom Access Providers File

For other environments or custom authentication, part of the design of Cluster Profiles (unlike Secrets) is to keep authentication information separate from the ClusterProfile itself. This is achieved using an "access providers" file, which lists named access providers with `execConfig`s that specify how to authenticate to a cluster.

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
    // ... more providers
  ]
}
```

The Cluster Profile controller reads this file, finds an access provider whose name matches one in the Cluster Profile object's `Status.AccessProviders` field, and generates the Secret to use the provider's `execConfig` for the cluster connection.

To provide this file to the controller, configure the `argocd-clusterprofile-controller` with the `--cluster-profile-providers-file` argument (or `ARGOCD_CLUSTERPROFILE_CONTROLLER_CLUSTER_PROFILE_PROVIDERS_FILE` environment variable) and mount the file through a Secret or ConfigMap (see the [kind cluster example](cluster-profiles-kind-example.md)).

### Deletion

The controller adds a finalizer (`argoproj.io/cluster-profile-finalizer`) to the `ClusterProfile` object. When the `ClusterProfile` is deleted, the controller will clean up the corresponding `Secret`.

## Configuration

The Cluster Profile controller is included in the default Argo CD installation, but its functionality is disabled by default. 

To enable the Cluster Profile controller, you must scale its deployment to 1 replica:
```bash
kubectl scale deployment argocd-clusterprofile-controller --replicas=1 -n argocd
```

To provide an access providers file to the controller, you should configure the `argocd-clusterprofile-controller` with the `--cluster-profile-providers-file` argument (or `ARGOCD_CLUSTERPROFILE_CONTROLLER_CLUSTER_PROFILE_PROVIDERS_FILE` environment variable). This should point to a mounted file that contains the configuration for the access providers.

### Disabling the feature

If you have enabled the controller and want to disable it, you can scale its deployment back to 0 replicas (default):
```bash
kubectl scale deployment argocd-clusterprofile-controller --replicas=0 -n argocd
```