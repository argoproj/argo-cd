# Cluster Decision Resource Generator

The cluster decision resource generates a list of Argo CD clusters. This is done using [duck-typing](https://pkg.go.dev/knative.dev/pkg/apis/duck), which does not require knowledge of the full shape of the referenced Kubernetes resource. The following is an example of a cluster-decision-resource-based ApplicationSet generator:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
 name: guestbook
 namespace: argocd
spec:
 goTemplate: true
 goTemplateOptions: ["missingkey=error"]
 generators:
 - clusterDecisionResource:
    # ConfigMap with GVK information for the duck type resource
    configMapRef: my-configmap  
    name: quak           # Choose either "name" of the resource or "labelSelector"
    labelSelector:
      matchLabels:       # OPTIONAL
        duck: spotted
      matchExpressions:  # OPTIONAL
      - key: duck
        operator: In
        values:
        - "spotted"
        - "canvasback"   
    # OPTIONAL: Checks for changes every 60sec (default 3min)
    requeueAfterSeconds: 60
 template:
   metadata:
     name: '{{.name}}-guestbook'
   spec:
      project: "default"
      source:
        repoURL: https://github.com/argoproj/argocd-example-apps/
        targetRevision: HEAD
        path: guestbook
      destination:
        server: '{{.clusterName}}' # 'server' field of the secret
        namespace: guestbook
```
The `quak` resource, referenced by the ApplicationSet `clusterDecisionResource` generator:
```yaml
apiVersion: mallard.io/v1beta1
kind: Duck
metadata:
  name: quak
spec: {}
status:
  # Duck-typing ignores all other aspects of the resource except 
  # the "decisions" list
  decisions:
  - clusterName: cluster-01
  - clusterName: cluster-02
```
The `ApplicationSet` resource references a `ConfigMap` that defines the resource to be used in this duck-typing. Only one ConfigMap is required per `ArgoCD` instance, to identify a resource. You can support multiple resource types by creating a `ConfigMap` for each.
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-configmap
data:
  # apiVersion of the target resource
  apiVersion: mallard.io/v1beta1  
  # kind of the target resource
  kind: ducks
  # status key name that holds the list of Argo CD clusters
  statusListKey: decisions
  # The key in the status list whose value is the cluster name found in Argo CD
  matchKey: clusterName
```

(*The full example can be found [here](https://github.com/argoproj/argo-cd/tree/master/applicationset/examples/clusterDecisionResource).*)

This example leverages the cluster management capabilities of the [open-cluster-management.io community](https://open-cluster-management.io/). By creating a `ConfigMap` with the GVK for the `open-cluster-management.io` Placement rule, your ApplicationSet can provision to different clusters in a number of novel ways. One example is to have the ApplicationSet maintain only two Argo CD Applications across 3 or more clusters. Then as maintenance or outages occur, the ApplicationSet will always maintain two Applications, moving the application to available clusters under the Placement rule's direction. 

## How it works
The ApplicationSet needs to be created in the Argo CD namespace, placing the `ConfigMap` in the same namespace allows the ClusterDecisionResource generator to read it. The `ConfigMap` stores the GVK information as well as the status key definitions.  In the open-cluster-management example, the ApplicationSet generator will read the kind `placementrules` with an apiVersion of `apps.open-cluster-management.io/v1`. It will attempt to extract the **list** of clusters from the key `decisions`. It then validates the actual cluster name as defined in Argo CD against the **value** from the key `clusterName` in each of the elements in the list.

The ClusterDecisionResource generator passes the 'name', 'server' and any other key/value in the duck-type resource's status list as parameters into the ApplicationSet template. In this example, the decision array contained an additional key `clusterName`, which is now available to the ApplicationSet template.

!!! note "Clusters listed as `Status.Decisions` must be predefined in Argo CD"
    The cluster names listed in the `Status.Decisions` *must* be defined within Argo CD, in order to generate applications for these values. The ApplicationSet controller does not create clusters within Argo CD.

    The Default Cluster list key is `clusters`.