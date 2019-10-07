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
  To work around this, you can order `spec.replicas` in Git in the same order that the controller
  prefers.

In case it is impossible to fix the upstream issue, Argo CD allows you to optionally ignore differences of problematic resources.
The diffing customization can be configured for single or multiple application resources or at a system level.

## Application Level Configuration

Argo CD allows ignoring differences at a specific JSON path. The following sample application is configured to ignore differences in `spec.replicas` for all deployments:

```yaml
spec:
  ignoreDifferences:
  - group: apps
    kind: Deployment
    jsonPointers:
    - /spec/replicas
```

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

> v1.4 and later

You can further restrict the resources on which the customization should be applied by defining conditions that the resource has to match (or not match). A condition compares the state of a given part in the resource's definition specified as JSON pointer against a given expression.

This enables you to apply customization on auto-generated resources or values, such as the case with aggregated `ClusterRole` rules.

```yaml
spec:
  ignoreDifferences:
  - group: rbac.authorization.k8s.io
    kind: ClusterRole
    jsonPointers:
    - /rules
    conditions:
    - /aggregationRule/clusterRoleSelectors is defined
```

The above example would ignore all differences within `/rules` specification of the objects that are of Kind `rbac.authorization.k8s.io/ClusterRole` and have `/aggregationRule/clusterRoleSelectors` defined in their manifest. 

You can define any number of expressions in the `conditions` list. If you define multiple conditions, the final matching result will be evaluated according to the match strategy. The match strategy can be set by the `matchStrategy` property and can take one of the values:

* `all` (logical and, all conditions have to evaluate to true)
* `any` (logical or, one of the conditions has to evaluate to true)
* `none` (none of the conditions have to evaluate to true).

The default value for `matchStrategy` is `all`, which will be used if `matchStrategy` is not set in the configuration.

```yaml
spec:
  ignoreDifferences:
  - group: rbac.authorization.k8s.io
    kind: ClusterRole
    jsonPointers:
    - /rules
    conditions:
    - /aggregationRule/clusterRoleSelectors is defined
    - /metadata/labels/rules.my-app.org~1ignorerbac is defined
    matchStrategy: any
```

Considering above example, the differences in the `/rules` part of the object's specification would be ignored if either `/aggregationRule/clusterRoleSelectors` or the label `rules.my-app.org/ignorerbac` is 
defined in the resource's definition. For the special value `~1` seen in the expression, refer to the note below.

Currently supported expressions are:

* `is defined` evaluates to true if the selector is defined in the resource's manifest
* `not defined` evaluates to true if the selector is not defined in the resource's manifest

!!! note "JSON pointer escaping"
    If you have JSON pointer path names that contain the special characters `/` or `~`, they need to be properly escaped in their notation. `/` becomes `~1` and `~` becomes `~0`. That means if you want to specify a label named `foo/bar/baz`, you'd have to specify the pointer to it as `foo~1bar~1baz`. For more information, refer to [the JSON pointer RFC](https://tools.ietf.org/html/rfc6901)

## System-Level Configuration

The comparison of resources with well-known issues can be customized at a system level. Ignored differences can be configured for a specified group and kind
in `resource.customizations` key of `argocd-cm` ConfigMap. Following is an example of a customization which ignores the `caBundle` field
of a `MutatingWebhookConfiguration` webhooks:

```yaml
data:
  resource.customizations: |
    admissionregistration.k8s.io/MutatingWebhookConfiguration:
      ignoreDifferences: |
        jsonPointers:
        - /webhooks/0/clientConfig/caBundle
```
