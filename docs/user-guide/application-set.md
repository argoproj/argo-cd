### Automating the generation of Argo CD Applications with the ApplicationSet Controller

Within the Argo CD project is the [ApplicationSet controller](https://github.com/argoproj-labs/applicationset) sub-project, which adds Application automation, improved multi-cluster support, and improved cluster multitenant support, to Argo CD.

The ApplicationSet controller is installed alongside Argo CD (within the same namespace), and the controller automatically generates Argo CD Applications based on the contents of a new `ApplicationSet` Custom Resource (CR).

Here is an example of an `ApplicationSet` resource that can be used to target an Argo CD Application to multiple clusters:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
spec:
  generators:
  - list:
      items:
      - cluster: engineering-dev
        url: https://1.2.3.4
      - cluster: engineering-prod
        url: https://2.4.6.8
      - cluster: finance-preprod
        url: https://9.8.7.6
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

The List generator passes the `url` and `cluster` fields into the template as `{{param}}`-style parameters, which are then rendered into three corresponding Argo CD Applications (one for each defined cluster). Targeting new clusters (or removing existing clusters) is simply a matter of altering the `ApplicationSet` resource, and the corresponding Argo CD Applications will be automatically created.

Likewise, changes made to the ApplicationSet `template` fields will automatically be applied to every generated Application. Managing a set of multiple Argo CD Applications is thus as easy as managing a single `ApplicationSet` resource.

Within ApplicationSet there exist other more powerful generators in addition to the List generator, including the Cluster generator (which automatically uses Argo CD-defined clusters to template Applications), and the Git generator (which uses the files/directories of a Git repository to template applications).

To learn more the ApplicationSet controller, check out [ApplicationSet documentation](https://argocd-applicationset.readthedocs.io/en/stable/) and [Getting Started](https://argocd-applicationset.readthedocs.io/en/stable/Geting-Started/) to install the ApplicationSet controller alongside Argo CD.
