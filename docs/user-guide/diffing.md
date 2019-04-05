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
