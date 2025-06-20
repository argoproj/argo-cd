# How the Cluster Decision Resource generator works for clusterDecisionResource
1. The Cluster Decision Resource generator reads a configurable status format:
```yaml
status:
  clusters:
  - name: cluster-01
  - name: cluster-02
```
This is a common status format.  Another format that could be read looks like this:
```yaml
status:
  decisions:
  - clusterName: cluster-01
    namespace: cluster-01
  - clusterName: cluster-02
    namespace: cluster-02
```
2. Any resource that has a list of key / value pairs, where the value matches ArgoCD cluster names can be used.
3. The key / value pairs found in each element of the list will be available to the template. As well, `name` and `server` will still be available to the template.
4. The Service Account used by the ApplicationSet controller must have access to `Get` the resource you want to retrieve the duck type definition from
5. A configMap is used to identify the resource to read status of generated ArgoCD clusters from. You can use multiple resources by creating a ConfigMap for each one in the ArgoCD namespace.
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-configmap
data:
  apiVersion: group.io/v1
  kind: mykinds
  statusListKey: clusters
  matchKey: name
```
  * `apiVersion`    - This is the apiVersion of your resource
  * `kind`          - This is the plural kind of your resource
  * `statusListKey` - Default is 'clusters', this is the key found in your resource's status that is a list of ArgoCD clusters.
  * `matchKey`      - Is the key name found in the cluster list, `name` and `clusterName` are the keys in the examples above.

# Applying the example
1. Connect to a cluster with the ApplicationSet controller running
2. Edit the Role for the ApplicationSet service account, and grant it permission to `list` the `placementdecisions` resources, from apiGroups `cluster.open-cluster-management.io/v1alpha1`
```yaml
- apiGroups:
  - "cluster.open-cluster-management.io/v1alpha1"
  resources:
  - placementdecisions
  verbs:
  - list
```
3. Apply the following controller and associated ManagedCluster CRD's:
https://github.com/open-cluster-management/placement
4. Now apply the PlacementDecision and an ApplicationSet:
```bash
kubectl apply -f ./placementdecision.yaml
kubectl apply -f ./configMap.yaml
kubectl apply -f ./ducktype-example.yaml
```
5. For now this won't do anything until you create a controller that populates the `Status.Decisions` array.