# Diffing Customization

It is possible for an application to be `OutOfSync` even immediately after a successful Sync operation. Some reasons for this might be:

* There is a bug in the manifest, where it contains extra/unknown fields from the actual K8s spec. These extra fields would get dropped when querying Kubernetes for the live state,
resulting in an `OutOfSync` status indicating a missing field was detected.
* The sync was performed (with pruning disabled), and there are resources which need to be deleted.
* A controller or [mutating webhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#mutatingadmissionwebhook) is altering the object after it was
submitted to Kubernetes in a manner which contradicts Git.
* A Helm chart is using a template function such as [`randAlphaNum`](https://github.com/helm/charts/blob/master/stable/redis/templates/secret.yaml#L16),
which generates different data every time `helm template` is invoked.
* For Horizontal Pod Autoscaling (HPA) objects, the HPA controller is known to reorder `spec.metrics`
  in a specific order. See [kubernetes issue #74099](https://github.com/kubernetes/kubernetes/issues/74099).
  To work around this, you can order `spec.metrics` in Git in the same order that the controller
  prefers.

In case it is impossible to fix the upstream issue, Argo CD allows you to optionally ignore differences of problematic resources.
The diffing customization can be configured for single or multiple application resources or at a system level.

## Application Level Configuration

Argo CD allows ignoring differences at a specific JSON path, using [RFC6902 JSON patches](https://tools.ietf.org/html/rfc6902) and [JQ path expressions](https://stedolan.github.io/jq/manual/#path(path_expression)). It is also possible to ignore differences from fields owned by specific managers defined in `metadata.managedFields` in live resources.

The following sample application is configured to ignore differences in `spec.replicas` for all deployments:

```yaml
spec:
  ignoreDifferences:
  - group: apps
    kind: Deployment
    jsonPointers:
    - /spec/replicas
```

Note that the `group` field relates to the [Kubernetes API group](https://kubernetes.io/docs/reference/using-api/#api-groups) without the version.
The above customization could be narrowed to a resource with the specified name and optional namespace:

```yaml
spec:
  ignoreDifferences:
  - group: apps
    kind: Deployment
    name: guestbook
    namespace: default
    jsonPointers:
    - /spec/replicas
```

To ignore elements of a list, you can use JQ path expressions to identify list items based on item content:
```yaml
spec:
  ignoreDifferences:
  - group: apps
    kind: Deployment
    jqPathExpressions:
    - .spec.template.spec.initContainers[] | select(.name == "injected-init-container")
```

To ignore fields owned by specific managers defined in your live resources:
```yaml
spec:
  ignoreDifferences:
  - group: *
    kind: *
    managedFieldsManagers:
    - kube-controller-manager
```

The above configuration will ignore differences from all fields owned by `kube-controller-manager` for all resources belonging to this application.

## System-Level Configuration

The comparison of resources with well-known issues can be customized at a system level. Ignored differences can be configured for a specified group and kind
in `resource.customizations` key of `argocd-cm` ConfigMap. Following is an example of a customization which ignores the `caBundle` field
of a `MutatingWebhookConfiguration` webhooks:

```yaml
data:
  resource.customizations.ignoreDifferences.admissionregistration.k8s.io_MutatingWebhookConfiguration: |
    jqPathExpressions:
    - '.webhooks[]?.clientConfig.caBundle'
```

Resource customization can also be configured to ignore all differences made by a managedField.manager at the system level. The example bellow shows how to configure ArgoCD to ignore changes made by `kube-controller-manager` in `Deployment` resources.

```yaml
data:
  resource.customizations.ignoreDifferences.apps_Deployment: |
    managedFieldsManagers:
    - kube-controller-manager
```

It is possible to configure ignoreDifferences to be applied to all resources in every Application managed by an ArgoCD instance. In order to do so, resource customizations can be configured like in the example bellow:

```yaml
data:
  resource.customizations.ignoreDifferences.all: |
    managedFieldsManagers:
    - kube-controller-manager
    jsonPointers:
    - /spec/replicas
```

The `status` field of `CustomResourceDefinitions` is often stored in Git/Helm manifest and should be ignored during diffing. The `ignoreResourceStatusField` setting simplifies
handling that edge case:

```yaml
data:
  resource.compareoptions: |
    # disables status field diffing in specified resource types
    # 'crd' - CustomResourceDefinition-s (default)
    # 'all' - all resources
    # 'none' - disabled
    ignoreResourceStatusField: crd
```

By default `status` field is ignored during diffing for `CustomResourceDefinition` resource. The behavior can be extended to all resources using `all` value or disabled using `none`.

## Known Kubernetes types in CRDs (Resource limits, Volume mounts etc)

Some CRDs are re-using data structures defined in the Kubernetes source base and therefore inheriting custom
JSON/YAML marshaling. Custom marshalers might serialize CRDs in a slightly different format that causes false
positives during drift detection. 

A typical example is the `argoproj.io/Rollout` CRD that re-using `core/v1/PodSpec` data structure. Pod resource requests
might be reformatted by the custom marshaller of `IntOrString` data type:

from:
```yaml
resources:
  requests:
    cpu: 100m
```

to:
```yaml
resources:
  requests:
    cpu: 0.1
```

The solution is to specify which CRDs fields are using built-in Kubernetes types in the `resource.customizations`
section of `argocd-cm` ConfigMap:  

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
  labels:
    app.kubernetes.io/name: argocd-cm
    app.kubernetes.io/part-of: argocd
data:
  resource.customizations.knownTypeFields.argoproj.io_Rollout: |
    - field: spec.template.spec
      type: core/v1/PodSpec
```

The list of supported Kubernetes types is available in [diffing_known_types.txt](https://raw.githubusercontent.com/argoproj/argo-cd/master/util/argo/normalizers/diffing_known_types.txt)
