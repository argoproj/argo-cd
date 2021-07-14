# Roadmap

- [Roadmap](#roadmap)
  - [v2.1](#v21)
    - [Config Management Tools Integrations (proposal)](#config-management-tools-integrations-proposal)
    - [Argo CD Extensions (proposal)](#argo-cd-extensions-proposal)
    - [Project scoped repository and clusters (proposal)](#project-scoped-repository-and-clusters-proposal)
    - [Headless Argo CD (aka GitOps Agent) (proposal)](#headless-argo-cd-aka-gitops-agent-proposal)
  - [v2.2 and beyond](#v22-and-beyond)
    - [Application Details Page Usability](#application-details-page-usability)
    - [Cluster Management User Interface](#cluster-management-user-interface)
    - [GitOps Engine Enhancements](#gitops-engine-enhancements)
  - [Completed](#completed)
    - [Core Functionality Bug Fixes](#core-functionality-bug-fixes)
    - [Performance](#performance)
    - [ApplicationSet](#applicationset)
    - [Large Applications support](#large-applications-support)
    - [Serviceability](#serviceability)
    - [Argo CD Notifications](#argo-cd-notifications)
    - [Automated Registry Monitoring](#automated-registry-monitoring)
    - [Projects Enhancements](#projects-enhancements)


## v2.1

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

## v2.2 and beyond

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

## Completed


### Core Functionality Bug Fixes

The core GitOps features still have several known bugs and limitations. The full list is available in [v1.9 milestone](
https://github.com/argoproj/argo-cd/issues?q=is%3Aopen+is%3Aissue+label%3Abug+milestone%3A%22v1.9%22+label%3Acomponent%3Acore)

The most notable issues:

* [Argo CD synchronization lasts incredibly long](https://github.com/argoproj/argo-cd/issues/3663)

### Performance

* 2000+ Applications support. The user interface becomes notably slower if one Argo CD instance manages more than 1 thousand applications.
A set of optimizations is required to fix that issue.

* 100+ Clusters support. The cluster addon management use-case requires connecting a large number of clusters to one Argo CD controller.
Currently Argo CD controller is unable to handle that many clusters. The solution is to support horizontal controller scaling and automated sharding.

* Mono Repository support. Argo CD is not optimized for mono repositories with a large number of applications. With 50+ applications in the same repository, manifest generation performance drops significantly. The repository server optimization is required to improve it.

### ApplicationSet

Argo CD Applications allow splitting the cluster configuration into logic groups that are managed independently. However, the set of applications
is a configuration that should be managed declaratively as well. The app-of-apps pattern solves this problem but still has some challenges such as
maintenance overhead, security, and lack of some additional features.

[ApplicationSet](https://github.com/argoproj-labs/applicationset) project provides a better solution for managing applications across multiple environments.

### Large Applications support

The application details page is not suitable to visualize applications that include a large number of resources (hundreds of resources). The page has to be reworked
to improve user experience.

### Serviceability

To make Argo CD successful we need to build tools that enable Argo CD administrators to handle scalability and performance issues in a self-service model.

That includes more metrics, out of the box alerts and a cluster management user interface.


### Argo CD Notifications

[Argo CD Notifications](https://github.com/argoproj-labs/argocd-notifications) provides the ability to notify users about Argo CD Application
changes as well as implement integrations such as update GitHub commit status, trigger Jenkins job, set Grafana label, etc.

### Automated Registry Monitoring

[Argo CD Image Updater](https://github.com/argoproj-labs/argocd-image-updater) provides an ability to monitor Docker registries and automatically
update image versions in the deployment repository. See [https://github.com/argoproj/argo-cd/issues/1648](https://github.com/argoproj/argo-cd/issues/1648).


### Projects Enhancements

Argo CD projects accumulated a lot of debt:

* Users don't know how to use project roles and SSO. It is one of the key features but not documented well. We need to document and promote it
* Project management UI has evolved organically and needs a complete redesign. We packaged everything into one sliding panel which is painful to use
* Enhancements: [#3598](https://github.com/argoproj/argo-cd/issues/3598)