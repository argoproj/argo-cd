# Roadmap

- [Roadmap](#roadmap)
    - [Config Management Tools Integrations (proposal)](#config-management-tools-integrations-proposal)
    - [Argo CD Extensions (proposal)](#argo-cd-extensions-proposal)
    - [Project scoped repository and clusters (proposal)](#project-scoped-repository-and-clusters-proposal)
    - [Headless Argo CD (aka GitOps Agent) (proposal)](#headless-argo-cd-aka-gitops-agent-proposal)
    - [Application Details Page Usability](#application-details-page-usability)
    - [Cluster Management User Interface](#cluster-management-user-interface)
    - [GitOps Engine Enhancements](#gitops-engine-enhancements)


### Config Management Tools Integrations ([proposal](https://github.com/argoproj/argo-cd/pull/5927))

The community likes the first class support of Helm, Kustomize and keeps requesting support for more tools.
Argo CD provides a mechanism to integrate with any config management tool. We need to investigate why
it is not enough and implement missing features.


### Argo CD Extensions ([proposal](https://github.com/argoproj/argo-cd/pull/6240))

Argo CD supports customizing handling of Kubernetes resources via diffing customizations,
health checks, and custom actions. The Argo CD Extensions proposal takes it to next
level and allows to deliver the resource customizations along with custom visualization in Argo CD
via Git repository.

### Project scoped repository and clusters ([proposal](https://github.com/argoproj/argo-cd/blob/master/docs/proposals/project-repos-and-clusters.md))

The feature streamlines the process of adding repositories and clusters to the project and makes it self-service.
Instead of asking an administrator to change Argo CD settings end users can perform the change independently.

### Headless Argo CD (aka GitOps Agent) ([proposal](https://github.com/argoproj/argo-cd/pull/6385))

Headless Argo CD allows to installation and use of lightweight Argo CD that includes only the backend without exposing the API or UI.
The Headless Argo CD provides a better experience to users who need only core Argo CD features and don't want to deal with multi-tenancy features.

### Application Details Page Usability

Application details page has accumulated multiple usability and feature requests such as 
[Node view](https://github.com/argoproj/argo-cd/issues/1483),
Network view ([1](https://github.com/argoproj/argo-cd/issues/2892), [2](https://github.com/argoproj/argo-cd/issues/2338))
 [etc](https://github.com/argoproj/argo-cd/issues/2199).

### Cluster Management User Interface

Argo CD has information about whole clusters, not just applications in it.
We need to provide a user interface for cluster administrators that visualize cluster level resources.

### GitOps Engine Enhancements

The [GitOps Engine](https://github.com/argoproj/gitops-engine) is a library that implements core GitOps functions such as K8S resource reconciliation and diffing.
A lot of Argo CD features are still not available in GitOps engine. The following features have to be contributed to the GitOps Engine:

* an ability to customize resources health assessment and existing CRD health [assessment functions](https://github.com/argoproj/argo-cd/tree/master/resource_customizations).
* resource diffing [customization](../user-guide/diffing/).
* config management [tools](../user-guide/application_sources/) integration.
* unified syncing annotations [argoproj/gitops-engine#43](https://github.com/argoproj/gitops-engine/issues/43).
