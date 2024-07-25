# Cluster Generator

In Argo CD, managed clusters [are stored within Secrets](../../declarative-setup/#clusters) in the Argo CD namespace. The ApplicationSet controller uses those same Secrets to generate parameters to identify and target available clusters.

For each cluster registered with Argo CD, the Cluster generator produces parameters based on the list of items found within the cluster secret.

It automatically provides the following parameter values to the Application template for each cluster:

- `name`
- `nameNormalized` *('name' but normalized to contain only lowercase alphanumeric characters, '-' or '.')*
- `server`
- `metadata.labels.<key>` *(for each label in the Secret)*
- `metadata.annotations.<key>` *(for each annotation in the Secret)*

!!! note
    Use the `nameNormalized` parameter if your cluster name contains characters (such as underscores) that are not valid for Kubernetes resource names. This prevents rendering invalid Kubernetes resources with names like `my_cluster-app1`, and instead would convert them to `my-cluster-app1`.


Within [Argo CD cluster Secrets](../../declarative-setup/#clusters) are data fields describing the cluster:
```yaml
kind: Secret
data:
  # Within Kubernetes these fields are actually encoded in Base64; they are decoded here for convenience.
  # (They are likewise decoded when passed as parameters by the Cluster generator)
  config: "{'tlsClientConfig':{'insecure':false}}"
  name: "in-cluster2"
  server: "https://kubernetes.default.svc"
metadata:
  labels:
    argocd.argoproj.io/secret-type: cluster
# (...)
```

The Cluster generator will automatically identify clusters defined with Argo CD, and extract the cluster data as parameters:
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
  - clusters: {} # Automatically use all clusters defined within Argo CD
  template:
    metadata:
      name: '{{.name}}-guestbook' # 'name' field of the Secret
    spec:
      project: "my-project"
      source:
        repoURL: https://github.com/argoproj/argocd-example-apps/
        targetRevision: HEAD
        path: guestbook
      destination:
        server: '{{.server}}' # 'server' field of the secret
        namespace: guestbook
```
(*The full example can be found [here](https://github.com/argoproj/argo-cd/tree/master/applicationset/examples/cluster).*)

In this example, the cluster secret's `name` and `server` fields are used to populate the `Application` resource `name` and `server` (which are then used to target that same cluster).

### Label selector

A label selector may be used to narrow the scope of targeted clusters to only those matching a specific label:
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
  - clusters:
      selector:
        matchLabels:
          staging: "true"
        # The cluster generator also supports matchExpressions.
        #matchExpressions:
        #  - key: staging
        #    operator: In
        #    values:
        #      - "true"
  template:
  # (...)
```

This would match an Argo CD cluster secret containing:
```yaml
apiVersion: v1
kind: Secret
data:
  # (... fields as above ...)
metadata:
  labels:
    argocd.argoproj.io/secret-type: cluster
    staging: "true"
# (...)
```

The cluster selector also supports set-based requirements, as used by [several core Kubernetes resources](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#resources-that-support-set-based-requirements).

### Deploying to the local cluster

In Argo CD, the 'local cluster' is the cluster upon which Argo CD (and the ApplicationSet controller) is installed. This is to distinguish it from 'remote clusters', which are those that are added to Argo CD [declaratively](../../declarative-setup/#clusters) or via the [Argo CD CLI](../../getting_started.md/#5-register-a-cluster-to-deploy-apps-to-optional).
 
The cluster generator will automatically target both local and non-local clusters, for every cluster that matches the cluster selector.

If you wish to target only remote clusters with your Applications (e.g. you want to exclude the local cluster), then use a cluster selector with labels, for example:
```yaml
spec:
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  generators:
  - clusters:
      selector:
        matchLabels:
          argocd.argoproj.io/secret-type: cluster
        # The cluster generator also supports matchExpressions.
        #matchExpressions:
        #  - key: staging
        #    operator: In
        #    values:
        #      - "true"
```

This selector will not match the default local cluster, since the default local cluster does not have a Secret (and thus does not have the `argocd.argoproj.io/secret-type` label on that secret). Any cluster selector that selects on that label will automatically exclude the default local cluster.

However, if you do wish to target both local and non-local clusters, while also using label matching, you can create a secret for the local cluster within the Argo CD web UI:

1. Within the Argo CD web UI, select *Settings*, then *Clusters*.
2. Select your local cluster, usually named `in-cluster`.
3. Click the *Edit* button, and change the *NAME* of the cluster to another value, for example `in-cluster-local`. Any other value here is fine.
4. Leave all other fields unchanged.
5. Click *Save*.

These steps might seem counterintuitive, but the act of changing one of the default values for the local cluster causes the Argo CD Web UI to create a new secret for this cluster. In the Argo CD namespace, you should now see a Secret resource named `cluster-(cluster suffix)` with label `argocd.argoproj.io/secret-type": "cluster"`. You may also create a local [cluster secret declaratively](../../declarative-setup/#clusters), or with the CLI using `argocd cluster add "(context name)" --in-cluster`, rather than through the Web UI.

### Fetch clusters based on their K8s version

There is also the possibility to fetch clusters based upon their Kubernetes version. To do this, the label `argocd.argoproj.io/auto-label-cluster-info` needs to be set to `true` on the cluster secret. 
Once that has been set, the controller will dynamically label the cluster secret with the Kubernetes version it is running on. To retrieve that value, you need to use the
`argocd.argoproj.io/kubernetes-version`, as the example below demonstrates:

```yaml
spec:
  goTemplate: true
  generators:
  - clusters:
      selector:
        matchLabels:
          argocd.argoproj.io/kubernetes-version: 1.28
        # matchExpressions are also supported.
        #matchExpressions:
        #  - key: argocd.argoproj.io/kubernetes-version
        #    operator: In
        #    values:
        #      - "1.27"
        #      - "1.28"
```

### Pass additional key-value pairs via `values` field

You may pass additional, arbitrary string key-value pairs via the `values` field of the cluster generator. Values added via the `values` field are added as `values.(field)`

In this example, a `revision` parameter value is passed, based on matching labels on the cluster secret:
```yaml
spec:
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  generators:
  - clusters:
      selector:
        matchLabels:
          type: 'staging'
      # A key-value map for arbitrary parameters
      values:
        revision: HEAD # staging clusters use HEAD branch
  - clusters:
      selector:
        matchLabels:
          type: 'production'
      values:
        # production uses a different revision value, for 'stable' branch
        revision: stable
  template:
    metadata:
      name: '{{.name}}-guestbook'
    spec:
      project: "my-project"
      source:
        repoURL: https://github.com/argoproj/argocd-example-apps/
        # The cluster values field for each generator will be substituted here:
        targetRevision: '{{.values.revision}}'
        path: guestbook
      destination:
        server: '{{.server}}'
        namespace: guestbook
```

In this example the `revision` value from the `generators.clusters` fields is passed into the template as `values.revision`, containing either `HEAD` or `stable` (based on which generator generated the set of parameters).

!!! note
    The `values.` prefix is always prepended to values provided via `generators.clusters.values` field. Ensure you include this prefix in the parameter name within the `template` when using it.

In `values` we can also interpolate the following parameter values (i.e. the same values as presented in the beginning of this page)

- `name`
- `nameNormalized` *('name' but normalized to contain only lowercase alphanumeric characters, '-' or '.')*
- `server`
- `metadata.labels.<key>` *(for each label in the Secret)*
- `metadata.annotations.<key>` *(for each annotation in the Secret)*

Extending the example above, we could do something like this:

```yaml
spec:
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  generators:
  - clusters:
      selector:
        matchLabels:
          type: 'staging'
      # A key-value map for arbitrary parameters
      values:
        # If `my-custom-annotation` is in your cluster secret, `revision` will be substituted with it.
        revision: '{{index .metadata.annotations "my-custom-annotation"}}' 
        clusterName: '{{.name}}'
  - clusters:
      selector:
        matchLabels:
          type: 'production'
      values:
        # production uses a different revision value, for 'stable' branch
        revision: stable
        clusterName: '{{.name}}'
  template:
    metadata:
      name: '{{.name}}-guestbook'
    spec:
      project: "my-project"
      source:
        repoURL: https://github.com/argoproj/argocd-example-apps/
        # The cluster values field for each generator will be substituted here:
        targetRevision: '{{.values.revision}}'
        path: guestbook
      destination:
        # In this case this is equivalent to just using {{name}}
        server: '{{.values.clusterName}}'
        namespace: guestbook
```
