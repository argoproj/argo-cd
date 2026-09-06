# ConfigMap Generator

The ConfigMap generator generates a single set of parameters from the `data` of a referenced `ConfigMap`. Each key/value pair in the ConfigMap's `data` is exposed as a parameter to the template.

This is useful for driving an Application from operator-managed configuration. Tools such as Crossplane, Cluster API, Azure Service Operator, or Pulumi often export information about the resources they provision (bucket names, endpoints, region, etc.) into a ConfigMap; the ConfigMap generator lets you consume those values directly, without an external config-management plugin.

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
  - configMap:
      # The name of a ConfigMap in the ApplicationSet controller's namespace.
      configMapRef: guestbook-config
  template:
    metadata:
      name: '{{.cluster}}-guestbook'
    spec:
      project: "my-project"
      source:
        repoURL: https://github.com/argoproj/argo-cd.git
        targetRevision: HEAD
        path: applicationset/examples/configmap-generator/guestbook/{{.cluster}}
      destination:
        server: '{{.url}}'
        namespace: guestbook
```

Given the following ConfigMap, the generator passes `cluster` and `url` as parameters into the template:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: guestbook-config
  namespace: argocd
data:
  cluster: engineering-dev
  url: https://kubernetes.default.svc
```

The referenced ConfigMap **must exist in the same namespace as the ApplicationSet controller** (the same requirement as the [Plugin generator](Generators-Plugin.md)'s `configMapRef`). Only the ConfigMap's `data` is read; `binaryData` is not exposed.

!!! note
    The ConfigMap generator only reads ConfigMaps, by design. Argo CD is not built to handle Secrets as a first-class generator input, so Secret data is intentionally out of scope. To template values from a ConfigMap into an Application while keeping secrets in a Secret, combine this generator with a [Matrix](Generators-Matrix.md) or [Merge](Generators-Merge.md) generator and reference the Secret from within the rendered manifests (for example via a `Secret`-backed Helm value or a `cert-manager`/`external-secrets` resource).

## Values

As with other generators, the `configMap` generator supports a `values` field for arbitrary key/value pairs that are added to the generated parameters (and may themselves reference the ConfigMap data when `goTemplate: true`):

```yaml
spec:
  goTemplate: true
  generators:
  - configMap:
      configMapRef: guestbook-config
      values:
        prefix: "infra"
        clusterName: "{{.cluster}}"
  template:
    metadata:
      name: '{{.values.prefix}}-{{.values.clusterName}}'
```

In `goTemplate: true` mode the values are available under `{{.values.<key>}}`. When `goTemplate` is not set, they are available as `{{values.<key>}}`, matching the behaviour of the other generators.
