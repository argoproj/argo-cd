# Overview

> [!WARNING]
> **As an Argo CD user, you probably don't want to be reading this section of the docs.**
>
> This part of the manual is aimed at helping people contribute to Argo CD, documentation, or to develop third-party applications that interact with Argo CD, e.g.
> 
> * A chat bot
> * A Slack integration

## Preface
#### Understand the [Code Contribution Guide](code-contributions.md)
#### Understand the [Code Contribution Preface](submit-your-pr.md#preface)
    
## Contributing to Argo CD documentation

This guide will help you get started quickly with contributing documentation changes, performing the minimum setup you'll need.   
For backend and frontend contributions, that require a full building-testing-running-locally cycle, please refer to [Contributing to Argo CD backend and frontend ](index.md#contributing-to-argo-cd-backend-and-frontend) 

### Fork and clone Argo CD repository
- [Fork and clone Argo CD repository](development-environment.md#fork-and-clone-the-repository)

### Submit your PR
- [Before submitting a PR](submit-your-pr.md#before-submitting-a-pr)
- [Choose a correct title for your PR](submit-your-pr.md#title-of-the-pr)
- [Perform the PR template checklist](submit-your-pr.md#pr-template-checklist)

## Contributing to Argo CD Notifications documentation

This guide will help you get started quickly with contributing documentation changes, performing the minimum setup you'll need.
The notifications docs are located in [notifications-engine](https://github.com/argoproj/notifications-engine) Git repository and require 2 pull requests: one for the `notifications-engine` repo and one for the `argo-cd` repo.
For backend and frontend contributions, that require a full building-testing-running-locally cycle, please refer to [Contributing to Argo CD backend and frontend ](index.md#contributing-to-argo-cd-backend-and-frontend) 

### Fork and clone Argo CD repository
- [Fork and clone Argo CD repository](development-environment.md#fork-and-clone-the-repository)

### Submit your PR to notifications-engine
- [Before submitting a PR](submit-your-pr.md#before-submitting-a-pr)
- [Choose a correct title for your PR](submit-your-pr.md#title-of-the-pr)
- [Perform the PR template checklist](submit-your-pr.md#pr-template-checklist)

### Install Go on your machine
- [Install Go](development-environment.md#install-go)

### Submit your PR to argo-cd
- [Contributing to notifications-engine](dependencies.md#notifications-engine-githubcomargoprojnotifications-engine)
- [Before submitting a PR](submit-your-pr.md#before-submitting-a-pr)
- [Choose a correct title for your PR](submit-your-pr.md#title-of-the-pr)
- [Perform the PR template checklist](submit-your-pr.md#pr-template-checklist)

## Contributing to Argo CD backend and frontend 

This guide will help you set up your build & test environment, so that you can start developing and testing bug fixes and feature enhancements without having to make too much effort in setting up a local toolchain.

As is the case with the development process, this document is under constant change. If you notice any error, or if you think this document is out-of-date, or if you think it is missing something: Feel free to submit a PR or submit a bug to our GitHub issue tracker.

### Set up your development environment
- [Install required tools (Git, Go, Docker, etc)](development-environment.md#install-required-tools)
- [Install and start a local K8s cluster (Kind, Minikube or K3d)](development-environment.md#install-a-local-k8s-cluster)
- [Fork and clone Argo CD repository](development-environment.md#fork-and-clone-the-repository)
- [Install additional required development tools](development-environment.md#install-additional-required-development-tools)
- [Install latest Argo CD on your local cluster](development-environment.md#install-latest-argo-cd-on-your-local-cluster)

### Set up a development toolchain (local or virtualized)
- [Understand the differences between the toolchains](toolchain-guide.md#local-vs-virtualized-toolchain)
- Choose a development toolchain

    - Either [set up a local toolchain](toolchain-guide.md#setting-up-a-local-toolchain)
    - Or [set up a virtualized toolchain](toolchain-guide.md#setting-up-a-virtualized-toolchain)

### Perform the development cycle 
- [Set kubectl context to argocd namespace](development-cycle.md#set-kubectl-context-to-argocd-namespace)
- [Pull in all build dependencies](development-cycle.md#pull-in-all-build-dependencies)
- [Generate API glue code and other assets](development-cycle.md#generate-api-glue-code-and-other-assets)
- [Build your code and run unit tests](development-cycle.md#build-your-code-and-run-unit-tests)
- [Lint your code base](development-cycle.md#lint-your-code-base)
- [Run e2e tests](development-cycle.md#run-end-to-end-tests)
- How to contribute to documentation: [build and run documentation site](docs-site.md) on your machine for manual testing

### Run and debug Argo CD locally
- [Run Argo CD on your machine for manual testing](running-locally.md)
- [Debug Argo CD in an IDE on your machine](debugging-locally.md)
  
### Submit your PR
- [Before submitting a PR](submit-your-pr.md#before-submitting-a-pr)
- [Understand the Continuous Integration process](submit-your-pr.md#continuous-integration-process)
- [Choose a correct title for your PR](submit-your-pr.md#title-of-the-pr)
- [Perform the PR template checklist](submit-your-pr.md#pr-template-checklist)
- [Understand the CI automated builds & tests](submit-your-pr.md#automated-builds-tests)
- [Understand & make sure your PR meets the CI code test coverage requirements](submit-your-pr.md#code-test-coverage)

Need help? Start with the [Contributors FAQ](faq.md)

## Contributing to Argo CD custom healthchecks 

### Preface
While Argo CD depends on the status health checks for the different CRDs in the K8s ecosystem, the resource status structure of those CRDs is decided upon and managed externally to Argo CD - by the authors and maintainers of each such CRD. Currently, the only way for Argo CD to reliably report the health status of each CRD is by having custom CRD healthchecks contributed to Argo CD.   

Due to the lack of standard for the CRD status field structure in the K8s ecosystem, the best persona to contribute such a health check to Argo CD is the author/maintainer of the corresponding CRD, since this persona knows the implementation details of how the CRD status is popluated, it's structure and when it is considered healthy or not. 
Also, the CRD maintainer has the full knowledge about whether the CRD follows the [kstatus](https://github.com/kubernetes-sigs/cli-utils/blob/master/pkg/kstatus/README.md) spec or not, whether the [observedGeneration](https://alenkacz.medium.com/kubernetes-operator-best-practices-implementing-observedgeneration-250728868792) field is used and populated correctly, etc.

The best way to achieve such a comlpete healthcheck in Argo CD is to open an issue in the respective project GitHub repo, asking for it's maintainers to contribute an Argo CD healthcheck.

If this is not possible, Argo CD users can contribute such healthchecks themselves, based on their observations of how the different statuses of the CRD behave. The below guidelines are relevant for both CRD maintainers and Argo CD users contributing to healthchecks.

### Guidelines for writing health checks

#### Using kstatus
If the CRD status is in [kstatus](https://github.com/kubernetes-sigs/cli-utils/blob/master/pkg/kstatus/README.md) format, please state it as a comment in the health check (as Argo CD maintainers evaluate the usage of kstatus-based health calculation, having the knowledge about which CRDs follow this standard can help us with adoption).

#### Using K8s observedGeneration field
If the CRD uses the [observedGeneration](https://alenkacz.medium.com/kubernetes-operator-best-practices-implementing-observedgeneration-250728868792) field correctly, please base the health check on it. Using this field in calculating the CRD health is important for preventing situations in which the health status in Argo CD may flap if Argo CD is evaluating the health status before the CRD controller finished reconciling the changed CR.   
This is an [example](https://github.com/argoproj/argo-cd/blob/stable/resource_customizations/argoproj.io/Rollout/health.lua) of using this field in a health check.  

### Contributing the health checks

Custom health check scripts are located in the `resource_customizations` directory of [https://github.com/argoproj/argo-cd](https://github.com/argoproj/argo-cd). This must have the following directory structure:

```
argo-cd
|-- resource_customizations
|    |-- your.crd.group.io               # CRD group
|    |    |-- MyKind                     # Resource kind
|    |    |    |-- health.lua            # Health check
|    |    |    |-- health_test.yaml      # Test inputs and expected results
|    |    |    +-- testdata              # Directory with test resource YAML definitions
```

Each health check must have tests defined in `health_test.yaml` file. The `health_test.yaml` is a YAML file with the following structure:

```yaml
tests:
- healthStatus:
    status: ExpectedStatus
    message: Expected message
  inputPath: testdata/test-resource-definition.yaml
```

To test the implemented custom health checks, run `go test -v ./util/lua/`.

The [PR#1139](https://github.com/argoproj/argo-cd/pull/1139) is an example of Cert Manager CRDs custom health check.

#### Wildcard Support for Built-in Health Checks

You can use a single health check for multiple resources by using a wildcard in the group or kind directory names.

The `_` character behaves like a `*` wildcard. For example, consider the following directory structure:

```
argo-cd
|-- resource_customizations
|    |-- _.group.io               # CRD group
|    |    |-- _                   # Resource kind
|    |    |    |-- health.lua     # Health check
```

Any resource with a group that ends with `.group.io` will use the health check in `health.lua`.

Wildcard checks are only evaluated if there is no specific check for the resource.

If multiple wildcard checks match, the first one in the directory structure is used.

We use the [doublestar](https://github.com/bmatcuk/doublestar) glob library to match the wildcard checks. We currently
only treat a path as a wildcard if it contains a `_` character, but this may change in the future.

> [!IMPORTANT]
> **Avoid Massive Scripts**
>
> Avoid writing massive scripts to handle multiple resources. They'll get hard to read and maintain. Instead, just
> duplicate the relevant parts in resource-s

## Contributing to Argo CD dependencies
- [Contributing to argo-ui](dependencies.md#argo-ui-components-githubcomargoprojargo-ui)
- [Contributing to notifications-engine](dependencies.md#notifications-engine-githubcomargoprojnotifications-engine)

## Extensions and Third-Party Applications
* [UI Extensions](extensions/ui-extensions.md)
* [Proxy Extensions](extensions/proxy-extensions.md)
* [Config Management Plugins](../operator-manual/config-management-plugins.md)

## Contributing to Argo Website
The Argo website is maintained in the [argo-site](https://github.com/argoproj/argo-site) repository.
