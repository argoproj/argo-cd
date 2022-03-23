# Roadmap

- [Roadmap](#roadmap)
  - [v2.3](#v23)
    - [Merge Argo CD Notifications into Argo CD](#merge-argo-cd-notifications-into-argo-cd)
    - [Merge ApplicationSet controller into Argo CD](#merge-applicationset-controller-into-argo-cd)
    - [Compact resources tree](#compact-resources-tree)
    - [Maintain difference in cluster and git values for specific fields](#maintain-difference-in-cluster-and-git-values-for-specific-fields)
    - [ARM images and CLI binary](#arm-images-and-cli-binary)
    - [Server side apply](#server-side-apply)
  - [v2.4 and beyond](#v24-and-beyond)
    - [First class support for ApplicationSet resources](#first-class-support-for-applicationset-resources)
    - [Input Forms UI Refresh](#input-forms-ui-refresh)
    - [Merge Argo CD Image Updater into Argo CD](#merge-argo-cd-image-updater-into-argo-cd)
    - [Web Shell](#web-shell)
    - [Helm values from external repo](#helm-values-from-external-repo)
    - [Add support for secrets in Application parameters](#add-support-for-secrets-in-application-parameters)
    - [Config Management Tools Integrations UI/CLI](#config-management-tools-integrations-uicli)
    - [Allow specifying parent/child relationships in config](#allow-specifying-parentchild-relationships-in-config)
    - [Dependencies between applications](#dependencies-between-applications)
    - [Multi-tenancy improvements](#multi-tenancy-improvements)
    - [GitOps Engine Enhancements](#gitops-engine-enhancements)
  - [Completed](#completed)
    - [✅ Config Management Tools Integrations (proposal)](#-config-management-tools-integrations-proposal)
    - [✅ Argo CD Extensions (proposal)](#-argo-cd-extensions-proposal)
    - [✅ Project scoped repository and clusters (proposal)](#-project-scoped-repository-and-clusters-proposal)
    - [✅ Core Argo CD (proposal)](#-core-argo-cd-proposal)
    - [✅ Core Functionality Bug Fixes](#-core-functionality-bug-fixes)
    - [✅ Performance](#-performance)
    - [✅ ApplicationSet](#-applicationset)
    - [✅ Large Applications support](#-large-applications-support)
    - [✅ Serviceability](#-serviceability)
    - [✅ Argo CD Notifications](#-argo-cd-notifications)
    - [✅ Automated Registry Monitoring](#-automated-registry-monitoring)
    - [✅ Projects Enhancements](#-projects-enhancements)

## v2.3

> ETA: Feb 2021

### Merge Argo CD Notifications into Argo CD

The [Argo CD Notifications](https://github.com/argoproj-labs/argocd-notifications) should be merged into Argo CD and available out-of-the-box: [#7350](https://github.com/argoproj/argo-cd/issues/7350)

### Merge ApplicationSet controller into Argo CD

The ApplicationSet functionality is available in Argo CD out-of-the-box ([#7351](https://github.com/argoproj/argo-cd/issues/7351)).

### Compact resources tree

An ability to collaps leaf resources tree to improve visualization of very large applications: [#7349](https://github.com/argoproj/argo-cd/issues/7349)

### Maintain difference in cluster and git values for specific fields

The feature allows to avoid updating fields excluded from diffing ([#2913](https://github.com/argoproj/argo-cd/issues/2913)).

### ARM images and CLI binary

The release workflow should build and publish ARM images and CLI binaries: ([#4211](https://github.com/argoproj/argo-cd/issues/4211))

### Server side apply

Support using [server side apply](https://kubernetes.io/docs/reference/using-api/server-side-apply/) during application syncing
[#2267](https://github.com/argoproj/argo-cd/issues/2267)

## v2.4 and beyond

### First class support for ApplicationSet resources

The Argo CD UI/CLI/API allows to manage ApplicationSet resources same as Argo CD Applications ([#7352](https://github.com/argoproj/argo-cd/issues/7352)).

### Input Forms UI Refresh

Improved design of the input forms in Argo CD Web UI: https://www.figma.com/file/IIlsFqqmM5UhqMVul9fQNq/Argo-CD?node-id=0%3A1

### Merge Argo CD Image Updater into Argo CD

The [Argo CD Image Updater](https://github.com/argoproj-labs/argocd-image-updater) should be merged into Argo CD and available out-of-the-box: [#7385](https://github.com/argoproj/argo-cd/issues/7385)

### Web Shell

Exec into the Kubernetes Pod right from Argo CD Web UI! [#4351](https://github.com/argoproj/argo-cd/issues/4351)

### Helm values from external repo

The feature allows combining of-the-shelf Helm chart and value file in Git repository ([#2789](https://github.com/argoproj/argo-cd/issues/2789))

### Support multiple sources for an Application

Support more than one source for creating an Application [#8322](https://github.com/argoproj/argo-cd/pull/8322).

### Sharding application controller 

Application controller to scale automatically to provide high availability[#8340](https://github.com/argoproj/argo-cd/issues/8340).

### Add support for secrets in Application parameters

The feature allows referencing secrets in Application parameters. [#1786](https://github.com/argoproj/argo-cd/issues/1786).

### Config Management Tools Integrations UI/CLI

The continuation of the Config Management Tools of [proposal](https://github.com/argoproj/argo-cd/pull/5927). The Argo CD UI/CLI
should provide first class experience for configured third-party config management tools: [#5734](https://github.com/argoproj/argo-cd/issues/5734).

### Allow specifying parent/child relationships in config

The feature [#5082](https://github.com/argoproj/argo-cd/issues/5082) allows configuring parent/child relationships between resources. This allows to correctly
visualize custom resources that don't have owner references.

### Dependencies between applications

The feature allows specifying dependencies between applications that allow orchestrating synchronization of multiple applications. [#3517](https://github.com/argoproj/argo-cd/issues/3517)


### Multi-tenancy improvements

The multi-tenancy improvements that allow end-users to create Argo CD applications using Kubernetes directly without accessing Argo CD API.

* [Applications outside argocd namespace](https://github.com/argoproj/argo-cd/pull/6409)
* [AppSource](https://github.com/argoproj-labs/appsource)


### GitOps Engine Enhancements

The [GitOps Engine](https://github.com/argoproj/gitops-engine) is a library that implements core GitOps functions such as K8S resource reconciliation and diffing.
A lot of Argo CD features are still not available in GitOps engine. The following features have to be contributed to the GitOps Engine:

* an ability to customize resources health assessment and existing CRD health [assessment functions](https://github.com/argoproj/argo-cd/tree/master/resource_customizations).
* resource diffing [customization](../user-guide/diffing/).
* config management [tools](../user-guide/application_sources/) integration.
* unified syncing annotations [argoproj/gitops-engine#43](https://github.com/argoproj/gitops-engine/issues/43).

## Completed

### ✅ Config Management Tools Integrations ([proposal](https://github.com/argoproj/argo-cd/pull/5927))

The community likes the first class support of Helm, Kustomize and keeps requesting support for more tools.
Argo CD provides a mechanism to integrate with any config management tool. We need to investigate why
it is not enough and implement missing features.


### ✅ Argo CD Extensions ([proposal](https://github.com/argoproj/argo-cd/pull/6240))

Argo CD supports customizing handling of Kubernetes resources via diffing customizations,
health checks, and custom actions. The Argo CD Extensions proposal takes it to next
level and allows to deliver the resource customizations along with custom visualization in Argo CD
via Git repository.

### ✅ Project scoped repository and clusters ([proposal](https://github.com/argoproj/argo-cd/blob/master/docs/proposals/project-repos-and-clusters.md))

The feature streamlines the process of adding repositories and clusters to the project and makes it self-service.
Instead of asking an administrator to change Argo CD settings end users can perform the change independently.

### ✅ Core Argo CD ([proposal](https://github.com/argoproj/argo-cd/pull/6385))

Core Argo CD allows to installation and use of lightweight Argo CD that includes only the backend without exposing the API or UI.
The Core Argo CD provides a better experience to users who need only core Argo CD features and don't want to deal with multi-tenancy features.

### ✅ Core Functionality Bug Fixes

The core GitOps features still have several known bugs and limitations. The full list is available in [v1.9 milestone](
https://github.com/argoproj/argo-cd/issues?q=is%3Aopen+is%3Aissue+label%3Abug+milestone%3A%22v1.9%22+label%3Acomponent%3Acore)

The most notable issues:

* [Argo CD synchronization lasts incredibly long](https://github.com/argoproj/argo-cd/issues/3663)

### ✅ Performance

* 2000+ Applications support. The user interface becomes notably slower if one Argo CD instance manages more than 1 thousand applications.
A set of optimizations is required to fix that issue.

* 100+ Clusters support. The cluster addon management use-case requires connecting a large number of clusters to one Argo CD controller.
Currently Argo CD controller is unable to handle that many clusters. The solution is to support horizontal controller scaling and automated sharding.

* Mono Repository support. Argo CD is not optimized for mono repositories with a large number of applications. With 50+ applications in the same repository, manifest generation performance drops significantly. The repository server optimization is required to improve it.

### ✅ ApplicationSet

Argo CD Applications allow splitting the cluster configuration into logic groups that are managed independently. However, the set of applications
is a configuration that should be managed declaratively as well. The app-of-apps pattern solves this problem but still has some challenges such as
maintenance overhead, security, and lack of some additional features.

[ApplicationSet](https://github.com/argoproj-labs/applicationset) project provides a better solution for managing applications across multiple environments.

### ✅ Large Applications support

The application details page is not suitable to visualize applications that include a large number of resources (hundreds of resources). The page has to be reworked
to improve user experience.

### ✅ Serviceability

To make Argo CD successful we need to build tools that enable Argo CD administrators to handle scalability and performance issues in a self-service model.

That includes more metrics, out-of-the-box alerts and a cluster management user interface.


### ✅ Argo CD Notifications

[Argo CD Notifications](https://github.com/argoproj-labs/argocd-notifications) provides the ability to notify users about Argo CD Application
changes as well as implement integrations such as update GitHub commit status, trigger Jenkins job, set Grafana label, etc.

### ✅ Automated Registry Monitoring

[Argo CD Image Updater](https://github.com/argoproj-labs/argocd-image-updater) provides an ability to monitor Docker registries and automatically
update image versions in the deployment repository. See [https://github.com/argoproj/argo-cd/issues/1648](https://github.com/argoproj/argo-cd/issues/1648).


### ✅ Projects Enhancements

Argo CD projects accumulated a lot of debt:

* Users don't know how to use project roles and SSO. It is one of the key features but not documented well. We need to document and promote it
* Project management UI has evolved organically and needs a complete redesign. We packaged everything into one sliding panel which is painful to use
* Enhancements: [#3598](https://github.com/argoproj/argo-cd/issues/3598)