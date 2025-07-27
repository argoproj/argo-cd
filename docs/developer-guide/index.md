# Overview

!!! warning "As an Argo CD user, you probably don't want to be reading this section of the docs."
    This part of the manual is aimed at helping people contribute to Argo CD, documentation, or to develop third-party applications that interact with Argo CD, e.g.
    
    * A chat bot
    * A Slack integration

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
- [Choose a correct title for your PR](submit-your-pr.md#choose-a-correct-title-for-your-pr)
- [Perform the PR template checklist](submit-your-pr.md#perform-the-PR-template-checklist)

## Contributing to Argo CD Notifications documentation

This guide will help you get started quickly with contributing documentation changes, performing the minimum setup you'll need.
The notificaions docs are located in [notifications-engine](https://github.com/argoproj/notifications-engine) Git repository and require 2 pull requests: one for the `notifications-engine` repo and one for the `argo-cd` repo.
For backend and frontend contributions, that require a full building-testing-running-locally cycle, please refer to [Contributing to Argo CD backend and frontend ](index.md#contributing-to-argo-cd-backend-and-frontend) 

### Fork and clone Argo CD repository
- [Fork and clone Argo CD repository](development-environment.md#fork-and-clone-the-repository)

### Submit your PR to notifications-engine
- [Before submitting a PR](submit-your-pr.md#before-submitting-a-pr)
- [Choose a correct title for your PR](submit-your-pr.md#choose-a-correct-title-for-your-pr)
- [Perform the PR template checklist](submit-your-pr.md#perform-the-PR-template-checklist)

### Install Go on your machine
- [Install Go](development-environment.md#install-go)

### Submit your PR to argo-cd
- [Contributing to notifications-engine](dependencies.md#notifications-engine-githubcomargoprojnotifications-engine)
- [Before submitting a PR](submit-your-pr.md#before-submitting-a-pr)
- [Choose a correct title for your PR](submit-your-pr.md#choose-a-correct-title-for-your-pr)
- [Perform the PR template checklist](submit-your-pr.md#perform-the-PR-template-checklist)

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
- [Generate API glue code and other assets](development-cycle.md#generate-API-glue-code-and-other-assets)
- [Build your code and run unit tests](development-cycle.md#build-your-code-and-run-unit-tests)
- [Lint your code base](development-cycle.md#lint-your-code-base)
- [Run e2e tests](development-cycle.md#run-end-to-end-tests)
- How to contribute to documentation: [build and run documentation site](docs-site/) on your machine for manual testing

### Run and debug Argo CD locally
- [Run Argo CD on your machine for manual testing](running-locally.md)
- [Debug Argo CD in an IDE on your machine](debugging-locally.md)
  
### Submit your PR
- [Before submitting a PR](submit-your-pr.md#before-submitting-a-pr)
- [Understand the Continuous Integration process](submit-your-pr.md#understand-the-continuous-integration-process)
- [Choose a correct title for your PR](submit-your-pr.md#choose-a-correct-title-for-your-pr)
- [Perform the PR template checklist](submit-your-pr.md#perform-the-PR-template-checklist)
- [Understand the CI automated builds & tests](submit-your-pr.md#automated-builds-&-tests)
- [Understand & make sure your PR meets the CI code test coverage requirements](submit-your-pr.md#code-test-coverage)

Need help? Start with the [Contributors FAQ](faq/)

## Contributing to Argo CD dependencies
- [Contributing to argo-ui](dependencies.md#argo-ui-components-githubcomargoprojargo-ui)
- [Contributing to gitops-engine](dependencies.md#gitops-engine-githubcomargoprojgitops-engine)
- [Contributing to notifications-engine](dependencies.md#notifications-engine-githubcomargoprojnotifications-engine)

## Extensions and Third-Party Applications
* [UI Extensions](extensions/ui-extensions.md)
* [Proxy Extensions](extensions/proxy-extensions.md)
* [Config Management Plugins](../operator-manual/config-management-plugins/)

## Contributing to Argo Website
The Argo website is maintained in the [argo-site](https://github.com/argoproj/argo-site) repository.