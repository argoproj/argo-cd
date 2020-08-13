# Roadmap

* [Core Functionality Bug Fixes](#core-functionality-bug-fixes)
* [2000+ Applications support](#2000-applications-support)
* [100+ Clusters support](#100-clusters-support)
* [Mono Repository support](#mono-repository-support)
* [ApplicationsSet](#applicationsset)
* [Large Applications support](#large-applications-support)
* [Supportability](#supportability)
* [GitOps Engine Enhancements](#gitops-engine-enhancements)
* [GitOps Agent](#gitops-agent)
* [Config Management Tools integrations](#config-management-tools-integrations)
* [Cluster Management User interface](#cluster-management-user-interface)
* [Resource Actions revamp](#resource-actions-revamp)
* [Argo CD Notifications](#argo-cd-notifications)
* [Automated Registry Monitoring](#automated-registry-monitoring)
* [Application details page usability](#application-details-page-usability)
* [Projects enhancements](#projects-enhancements)

### Core Functionality Bug Fixes

The core GitOps features still have several known bugs and limitations. The full list is available in [v1.7 milestone](
https://github.com/argoproj/argo-cd/issues?q=is%3Aopen+is%3Aissue+label%3Abug+milestone%3A%22v1.7+%22+label%3Acomponent%3Acore)

The most notable issues:
* [Application is incorrectly reporting a diff](https://github.com/argoproj/argo-cd/issues/2865)
* [Helm hooks are deleted right after creation](https://github.com/argoproj/argo-cd/issues/2737)
* [Argo CD synchronization lasts incredibly long](https://github.com/argoproj/argo-cd/issues/3663)

### Performance

* 2000+ Applications support. The user interface became notable slower if one Argo CD instance manages more than 1 thousand applications.
A set of optimizations is required to fix that issue.

* 100+ Clusters support. The cluster addon management use-case requires connecting a large number of clusters to one Argo CD controller.
Currently Argo CD controller unable to handle that many clusters. The solution to support horizontal controller scaling and automated sharding.

* Mono Repository support. Argo CD is not optimized for mono repositories with a large number of applications. With 50+ applications in the same repository, manifest generation performance
drops significantly. The repository server optimization is required to improve it.

### ApplicationsSet

The project automates Argo CD applications management: https://github.com/argoproj-labs/applicationset

### Large Applications support

Application details page is not suitable to visualize applications that includes large number of resources (hundreds of resources). The page have to be reworked
to improve user experience.

### Supportability

To make Argo CD successful we need to build tools that enable Argo CD operators to handle scalability and performance issues without asking Argo CD team help.
That includes more metrics, out of the box alerts and cluster management user interface.

### GitOps Engine Enhancements

The [GitOps Engine](https://github.com/argoproj/gitops-engine) is a library that implements core GitOps functions such as K8S resource reconciliation and diffing.
A lot of Argo CD features are still no available in GitOps engine. The following features have to be contributed to the GitOps Engine:

* an ability to customize resources health assessment and existing CRD health [assessment functions](https://github.com/argoproj/argo-cd/tree/master/resource_customizations).
* resource diffing [customization](https://argoproj.github.io/argo-cd/user-guide/diffing/).
* config management [tools](https://argoproj.github.io/argo-cd/user-guide/application_sources/) integration
* unified syncing annotations [argoproj/gitops-engine#43](https://github.com/argoproj/gitops-engine/issues/43)

### GitOps Agent

[GitOps Agent](https://github.com/argoproj/gitops-engine/tree/master/agent) is a continuation of GitOps engine work. The GitOps Agent leverages the GitOps Engine and provides
access to many engine features via a simple CLI interface.

### Config Management Tools integrations

Community likes the first class support of Helm, Kustomize and keep requesting to support more tools.
Argo CD provides mechanism to integrate with any config management tool. We need to enhance investigate why
it is not enough and implement missing features.

### Resource Actions revamp

Resource actions is very powerful but literally hidden feature. Documentation is totally missing and therefore
adoption is poor. We need to document and promote it. Then iterate and work on enhancements:
- hard to configure unless you are Argo CD ninja;
- half done parameters support: we have backend but no UI/CLI for it;
- configuration issue: it is impossible to share actions as a YAML file since ALL resource customizations are stored in one config map key;

### Argo CD Notifications

[Argo CD Notifications](https://github.com/argoproj-labs/argocd-notifications) provides the ability to notify users about Argo CD Application
changes as well as implement integrations such as update Github commit status, trigger Jenkins job, set Grafana label etc.

### Automated Registry Monitoring

An ability to monitor Docker registry and automatically update image versions in the deployment repository.
https://github.com/argoproj/argo-cd/issues/1648

### Application details page usability

Application details page has accumulated multiple usability and feature requests such as 
[Node view](https://github.com/argoproj/argo-cd/issues/1483),
Logs ([1](https://github.com/argoproj/argo-cd/issues/781), [2](https://github.com/argoproj/argo-cd/issues/3382)),
Network view ([1](https://github.com/argoproj/argo-cd/issues/2892), [2](https://github.com/argoproj/argo-cd/issues/2338))
 [etc](https://github.com/argoproj/argo-cd/issues/2199).

### Cluster Management User interface

Argo CD has information about whole clusters, not just applications in it.
We need to provide a user interface for cluster administrators that visualize cluster level resources.

### Projects enhancements

Argo CD projects accumulated a lot of debt:
- Users don't know how to use project roles and SSO. It is one of the key features but just not documented. We need to document and promote it.
- Project management UI has evolved organically and need complete redesign. We packaged everything into one sliding panel which is painful to use.
- Enhancements: [#2718](https://github.com/argoproj/argo-cd/issues/2718), [#3598](https://github.com/argoproj/argo-cd/issues/3598)
