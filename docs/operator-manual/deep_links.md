# Deep Links

Deep links allow users to quickly redirect to third-party systems, such as Splunk, Datadog, etc. from the Argo CD
user interface.

Argo CD administrator will be able to configure links to third-party systems by providing 
deep link templates configured in `argocd-cm`. The templates can be conditionally rendered and are able 
to reference different types of resources relating to where the links show up, this includes projects, applications,
or individual resources (pods, services, etc.).

## Configuring Deep Links

The configuration for Deep Links is present in `argocd-cm` as `<location>.links` fields where 
`<location>` determines where it will be displayed. The possible values for `<location>` are:

- `project`: all links under this field will show up in the project tab in the Argo CD UI
- `application`: all links under this field will show up in the application summary tab
- `resource`: all links under this field will show up in the resource (deployments, pods, services, etc.) summary tab

Each link in the list has five subfields:

1. `title`: title/tag that will be displayed in the UI corresponding to that link
2. `url`: the actual URL where the deep link will redirect to, this field can be templated to use data from the
   corresponding application, project or resource objects (depending on where it is located). This uses [text/template](https://pkg.go.dev/text/template) pkg for templating
3. `description` (optional): a description for what the deep link is about
4. `icon.class` (optional): a font-awesome icon class to be used when displaying the links in dropdown menus
5. `if` (optional): a conditional statement that results in either `true` or `false`, it also has access to the same
   data as the `url` field. If the condition resolves to `true` the deep link will be displayed - else it will be hidden. If
   the field is omitted, by default the deep links will be displayed. This uses [expr-lang/expr](https://github.com/expr-lang/expr/tree/master/docs) for evaluating conditions

!!!note
    For resources of kind Secret the data fields are redacted but other fields are accessible for templating the deep links.

!!!warning
    Make sure to validate the url templates and inputs to prevent data leaks or possible generation of any malicious links.

As mentioned earlier the links and conditions can be templated to use data from the resource, each category of links can access different types of data linked to that resource.
Overall we have these 4 resources available for templating in the system:

- `app` or `application`: this key is used to access the application resource data.
- `resource`: this key is used to access values for the actual k8s resource.
- `cluster`: this key is used to access the related destination cluster data like name, server, namespaces etc.
- `project`: this key is used to access the project resource data.

The above resources are accessible in particular link categories, here's a list of resources available in each category:

- `resource.links`: `resource`, `application`, `cluster` and `project`
- `application.links`: `app`/`application` and `cluster`
- `project.links`: `project`

An example `argocd-cm.yaml` file with deep links and their variations :

```yaml
  # sample project level links
  project.links: |
    - url: https://myaudit-system.com?project={{.project.metadata.name}}
      title: Audit
      description: system audit logs
      icon.class: "fa-book"
  # sample application level links
  application.links: |
    # pkg.go.dev/text/template is used for evaluating url templates
    - url: https://mycompany.splunk.com?search={{.app.spec.destination.namespace}}&env={{.project.metadata.labels.env}}
      title: Splunk
    # conditionally show link e.g. for specific project
    # github.com/expr-lang/expr is used for evaluation of conditions
    - url: https://mycompany.splunk.com?search={{.app.spec.destination.namespace}}
      title: Splunk
      if: application.spec.project == "default"
    - url: https://{{.app.metadata.annotations.splunkhost}}?search={{.app.spec.destination.namespace}}
      title: Splunk
      if: app.metadata.annotations.splunkhost != ""
  # sample resource level links
  resource.links: |
    - url: https://mycompany.splunk.com?search={{.resource.metadata.name}}&env={{.project.metadata.labels.env}}
      title: Splunk
      if: resource.kind == "Pod" || resource.kind == "Deployment"
    
    # sample checking a tag exists that contains - or / and how to alternatively access it
    - url: https://mycompany.splunk.com?tag={{ index .resource.metadata.labels "some.specific.kubernetes.like/tag" }}
      title: Tag Service
      if: resource.metadata.labels["some.specific.kubernetes.like/tag"] != nil && resource.metadata.labels["some.specific.kubernetes.like/tag"] != ""
```
