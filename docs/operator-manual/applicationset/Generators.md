# Generators

Generators are responsible for generating *parameters*, which are then rendered into the `template:` fields of the ApplicationSet resource. See the [Introduction](index.md) for an example of how generators work with templates, to create Argo CD Applications.

Generators are primarily based on the data source that they use to generate the template parameters. For example: the List generator provides a set of parameters from a *literal list*, the Cluster generator uses the *Argo CD cluster list* as a source, the Git generator uses files/directories from a *Git repository*, and so.

As of this writing there are nine generators:

- [List generator](Generators-List.md): The List generator allows you to target Argo CD Applications to clusters based on a fixed list of any chosen key/value element pairs.
- [Cluster generator](Generators-Cluster.md): The Cluster generator allows you to target Argo CD Applications to clusters, based on the list of clusters defined within (and managed by) Argo CD (which includes automatically responding to cluster addition/removal events from Argo CD).
- [Git generator](Generators-Git.md): The Git generator allows you to create Applications based on files within a Git repository, or based on the directory structure of a Git repository.
- [Matrix generator](Generators-Matrix.md): The Matrix generator may be used to combine the generated parameters of two separate generators.
- [Merge generator](Generators-Merge.md): The Merge generator may be used to merge the generated parameters of two or more generators. Additional generators can override the values of the base generator.
- [SCM Provider generator](Generators-SCM-Provider.md): The SCM Provider generator uses the API of an SCM provider (eg GitHub) to automatically discover repositories within an organization.
- [Pull Request generator](Generators-Pull-Request.md): The Pull Request generator uses the API of an SCMaaS provider (eg GitHub) to automatically discover open pull requests within an repository.
- [Cluster Decision Resource generator](Generators-Cluster-Decision-Resource.md): The Cluster Decision Resource generator is used to interface with Kubernetes custom resources that use custom resource-specific logic to decide which set of Argo CD clusters to deploy to.
- [Plugin generator](Generators-Plugin.md): The Plugin generator make RPC HTTP request to provide parameters.

All generators can be filtered by using the [Post Selector](Generators-Post-Selector.md)

If you are new to generators, begin with the **List** and **Cluster** generators. For more advanced use cases, see the documentation for the remaining generators above.
