# Post Selector all generators

The Selector allows to post-filter based on generated values using the kubernetes common labelSelector format. In the example, the list generator generates a set of two application which then filter by the key value to only select the `env` with value `staging`:

## Example: List generator + Post Selector
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
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
  template:
    metadata:
      name: '{{cluster}}-guestbook'
    spec:
      project: default
      source:
        repoURL: https://github.com/argoproj-labs/applicationset.git
        targetRevision: HEAD
        path: examples/list-generator/guestbook/{{cluster}}
      destination:
        server: '{{url}}'
        namespace: guestbook
```

The List generator + Post Selector generates a single set of parameters:
```yaml
- cluster: engineering-dev
  url: https://kubernetes.default.svc
  env: staging
```