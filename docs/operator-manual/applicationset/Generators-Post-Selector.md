# Post Selector all generators

The `selector` field on a generator allows an `ApplicationSet` to post-filter results using [the Kubernetes common labelSelector format](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors) and the generated values.

`matchLabels` is a map of `{key,value}` pairs. This `list` generator generates a set of two `Applications`, which is then filtered using `matchLabels` to only the list element containing the key `env` with value `staging`:
```
spec:
  generators:
  - list:
      elements:
        - cluster: engineering-dev
          url: https://kubernetes.default.svc
          env: staging
        - cluster: engineering-prod
          url: https://kubernetes.default.svc
          env: prod
    selector:
      matchLabels:
        env: staging
```

The `list` generator + `matchLabels` selector generates a single set of parameters:
```yaml
- cluster: engineering-dev
  url: https://kubernetes.default.svc
  env: staging
```

It is also possible to use `matchExpressions` for more powerful selectors.

A single `{key,value}` in the `matchLabels` map is equivalent to an element of `matchExpressions`, whose `key` field is the "key", the `operator` is "In", and the `values` array contains only the "value". So the same example using `matchExpressions` looks like:
```yaml
spec:
  generators:
  - list:
      elements:
        - cluster: engineering-dev
          url: https://kubernetes.default.svc
          env: staging
        - cluster: engineering-prod
          url: https://kubernetes.default.svc
          env: prod
      selector:
        matchExpressions:
          - key: env
            operator: In
            values:
              - staging
```

Valid `operators` include `In`, `NotIn`, `Exists`, and `DoesNotExist`. The `values` set must be non-empty in the case of `In` and `NotIn`. 

## Full Example
In the example, the list generator generates a set of two applications, which then filter by the key value to only select the `env` with value `staging`:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
spec:
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  generators:
  - list:
      elements:
        - cluster: engineering-dev
          url: https://kubernetes.default.svc
          env: staging
        - cluster: engineering-prod
          url: https://kubernetes.default.svc
          env: prod
    selector:
      matchLabels:
        env: staging
  template:
    metadata:
      name: '{{.cluster}}-guestbook'
    spec:
      project: default
      source:
        repoURL: https://github.com/argoproj-labs/applicationset.git
        targetRevision: HEAD
        path: examples/list-generator/guestbook/{{.cluster}}
      destination:
        server: '{{.url}}'
        namespace: guestbook
```
