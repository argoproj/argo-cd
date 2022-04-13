# List Generator

The List generator generates parameters based on an arbitrary list of key/value pairs (as long as the values are string values). In this example, we're targeting a local cluster named `engineering-dev`:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
  namespace: argocd
spec:
  generators:
  - list:
      elements:
      - cluster: engineering-dev
        url: https://kubernetes.default.svc
#     - cluster: engineering-prod
#       url: https://kubernetes.default.svc
#       foo: bar
  template:
    metadata:
      name: '{{cluster}}-guestbook'
    spec:
      project: default
      source:
        repoURL: https://github.com/argoproj/argo-cd.git
        targetRevision: HEAD
        path: applicationset/examples/list-generator/guestbook/{{cluster}}
      destination:
        server: '{{url}}'
        namespace: guestbook
```
(*The full example can be found [here](https://github.com/argoproj/argo-cd/tree/master/applicationset/examples/list-generator).*)

In this example, the List generator passes the `url` and `cluster` fields as parameters into the template. If we wanted to add a second environment, we could uncomment the second element and the ApplicationSet controller would automatically target it with the defined application.

With the ApplicationSet v0.1.0 release, one could *only* specify `url` and `cluster` element fields (plus arbitrary `values`). As of ApplicationSet v0.2.0, any key/value `element` pair is supported (which is also fully backwards compatible with the v0.1.0 form):
```yaml
spec:
  generators:
  - list:
    elements:
      # v0.1.0 form - requires cluster/url keys:
      - cluster: engineering-dev
        url: https://kubernetes.default.svc
        values:
          additional: value
      # v0.2.0+ form - does not require cluster/URL keys
      # (but they are still supported).
      - staging: "true"
        gitRepo: https://kubernetes.default.svc   
# (...)
```

!!! note "Clusters must be predefined in Argo CD"
    These clusters *must* already be defined within Argo CD, in order to generate applications for these values. The ApplicationSet controller does not create clusters within Argo CD (for instance, it does not have the credentials to do so).
