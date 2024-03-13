### Automating the generation of Argo CD Applications with the ApplicationSet Controller

The [ApplicationSet controller](../operator-manual/applicationset/index.md) adds Application automation and seeks to improve multi-cluster support and cluster multitenant support within Argo CD. Argo CD Applications may be templated from multiple different sources, including from Git or Argo CD's own defined cluster list.

The set of tools provided by the ApplicationSet controller may also be used to allow developers (without access to the Argo CD namespace) to independently create Applications without cluster-administrator intervention.

!!! warning
    Be aware of the [security implications](../operator-manual/applicationset/Security.md) before allowing developers to
    create Applications via ApplicationSets.

The ApplicationSet controller automatically generates Argo CD Applications based on the contents of an `ApplicationSet` Custom Resource (CR).

Here is an example of an `ApplicationSet` resource that can be used to target an Argo CD Application to multiple clusters:
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
        url: https://1.2.3.4
      - cluster: engineering-prod
        url: https://2.4.6.8
      - cluster: finance-preprod
        url: https://9.8.7.6
  template:
    metadata:
      name: '{{.cluster}}-guestbook'
    spec:
      project: my-project
      source:
        repoURL: https://github.com/infra-team/cluster-deployments.git
        targetRevision: HEAD
        path: guestbook/{{.cluster}}
      destination:
        server: '{{.url}}'
        namespace: guestbook
```

The List generator passes the `url` and `cluster` fields into the template as `{{param}}`-style parameters, which are then rendered into three corresponding Argo CD Applications (one for each defined cluster). Targeting new clusters (or removing existing clusters) is simply a matter of altering the `ApplicationSet` resource, and the corresponding Argo CD Applications will be automatically created.

Likewise, changes made to the ApplicationSet `template` fields will automatically be applied to every generated Application. Managing a set of multiple Argo CD Applications is thus as easy as managing a single `ApplicationSet` resource.

Within ApplicationSet there exist other more powerful generators in addition to the List generator, including the Cluster generator (which automatically uses Argo CD-defined clusters to template Applications), and the Git generator (which uses the files/directories of a Git repository to template applications).

To learn more about the ApplicationSet controller, check out the [ApplicationSet documentation](../operator-manual/applicationset/index.md).
