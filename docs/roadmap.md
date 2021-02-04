# Roadmap

- [Roadmap](#roadmap)
    - [Core Functionality Bug Fixes](#core-functionality-bug-fixes)
    - [Performance](#performance)
    - [ApplicationsSet](#applicationsset)
    - [Large Applications support](#large-applications-support)
    - [Serviceability](#serviceability)
    - [GitOps Engine Enhancements](#gitops-engine-enhancements)
    - [GitOps Agent](#gitops-agent)
    - [Config Management Tools Integrations](#config-management-tools-integrations)
    - [Resource Actions Revamp](#resource-actions-revamp)
    - [Argo CD Notifications](#argo-cd-notifications)
    - [Automated Registry Monitoring](#automated-registry-monitoring)
    - [Application Details Page Usability](#application-details-page-usability)
    - [Cluster Management User Interface](#cluster-management-user-interface)
    - [Projects Enhancements](#projects-enhancements)

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

### GitOps Engine Enhancements

The [GitOps Engine](https://github.com/argoproj/gitops-engine) is a library that implements core GitOps functions such as K8S resource reconciliation and diffing.
A lot of Argo CD features are still not available in GitOps engine. The following features have to be contributed to the GitOps Engine:

* an ability to customize resources health assessment and existing CRD health [assessment functions](https://github.com/argoproj/argo-cd/tree/master/resource_customizations).
* resource diffing [customization](https://argoproj.github.io/argo-cd/user-guide/diffing/).
* config management [tools](https://argoproj.github.io/argo-cd/user-guide/application_sources/) integration.
* unified syncing annotations [argoproj/gitops-engine#43](https://github.com/argoproj/gitops-engine/issues/43).

### GitOps Agent

[GitOps Agent](https://github.com/argoproj/gitops-engine/tree/master/agent) is a continuation of GitOps engine work. The GitOps Agent leverages the GitOps Engine and provides
access to many engine features via a simple CLI interface.

### Config Management Tools Integrations

The community likes the first class support of Helm, Kustomize and keeps requesting support for more tools.
Argo CD provides a mechanism to integrate with any config management tool. We need to investigate why
it is not enough and implement missing features.

### Resource Actions Revamp

Resource actions is very powerful but literally hidden feature. Documentation is missing and therefore
adoption is poor. We need to document and promote it, and then iterate and work on enhancements:

* hard to configure unless you are Argo CD ninja
* half done parameters support: we have backend but no UI/CLI for it
* configuration issue: it is impossible to share actions as a YAML file since ALL resource customizations are stored in one config map key

### Argo CD Notifications

[Argo CD Notifications](https://github.com/argoproj-labs/argocd-notifications) provides the ability to notify users about Argo CD Application
changes as well as implement integrations such as update Github commit status, trigger Jenkins job, set Grafana label, etc.

### Automated Registry Monitoring

[Argo CD Image Updater](https://github.com/argoproj-labs/argocd-image-updater) provides an ability to monitor Docker registries and automatically
update image versions in the deployment repository. See [https://github.com/argoproj/argo-cd/issues/1648](https://github.com/argoproj/argo-cd/issues/1648).

### Application Details Page Usability

Application details page has accumulated multiple usability and feature requests such as 
[Node view](https://github.com/argoproj/argo-cd/issues/1483),
Logs ([1](https://github.com/argoproj/argo-cd/issues/781), [2](https://github.com/argoproj/argo-cd/issues/3382)),
Network view ([1](https://github.com/argoproj/argo-cd/issues/2892), [2](https://github.com/argoproj/argo-cd/issues/2338))
 [etc](https://github.com/argoproj/argo-cd/issues/2199).

### Cluster Management User Interface

Argo CD has information about whole clusters, not just applications in it.
We need to provide a user interface for cluster administrators that visualize cluster level resources.

### Projects Enhancements

Argo CD projects accumulated a lot of debt:

* Users don't know how to use project roles and SSO. It is one of the key features but not documented well. We need to document and promote it
* Project management UI has evolved organically and needs a complete redesign. We packaged everything into one sliding panel which is painful to use
* Enhancements: [#3598](https://github.com/argoproj/argo-cd/issues/3598)
