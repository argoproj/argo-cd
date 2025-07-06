# Overview

!!! warning "As an Argo CD user, you probably don't want to be reading this section of the docs."
    This part of the manual is aimed at helping people contribute to Argo CD, the documentation, or to develop third-party applications that interact with Argo CD, e.g.
    
    * A chat bot
    * A Slack integration
    

## Contributing to Argo CD backend, frontend and documentation

This guide shall help you in setting up your build & test environment, so that you can start developing and testing bug fixes and feature enhancements without having to make too much effort in setting up a local toolchain.

As is the case with the development process, this document is under constant change. If you notice any error, or if you think this document is out-of-date, or if you think it is missing something: Feel free to submit a PR or submit a bug to our GitHub issue tracker.

### Understand the [Code Contribution Guide](code-contributions.md)
### Understand the [Code Contribution Preface](submit-your-pr.md#preface)
### Set up your development environment
  - [Install required tools (Git, Go, Docker, etc)](development-environment.md#install-required-tools)
  - [Install and start a local K8s cluster (Kind, Minikube or K3d)](development-environment.md#install-a-local-k8s-cluster)
  - [Fork and clone Argo CD repository](development-environment.md#fork-and-clone-the-repository)
  - [Install additional required development tools](development-environment.md#install-additional-required-development-tools)
  - [Install latest Argo CD on your local cluster](development-environment.md#install-latest-argo-cd-on-your-local-cluster)
  - Set up a development toolchain (local or virtualized)
    - [Understand the differences between the toolchains](toolchain.md#local-vs-virtualized-toolchain)
    - Set up local development toolchain
      - Understand and install general prereqs for local development
      - Perform specific config for your Operating System (MacOS, Linux, Windows WSL)
    - Set up virtualized development toolchain
      - Understand and install general prereqs for virtualized development
      - Perform specific config for your Operating System (MacOS, Linux, Windows WSL)

### Perform the development cycle 
  - Pull in all build dependencies
  - Generate API glue code and other assets
  - Build your code and run unit tests
  - Make sure your code has sufficient coverage
  - Lint your code base
  - Run e2e tests
  - docs contribution: [build and run documentation site](docs-site/) on your machine for manual testing
  - cli contribution: build Argo CD cli for manual testing

### Run Argo CD on your machine for debugging and manual testing
  
### Submit your PR
  - [Before submitting a PR](submit-your-pr.md#before-submitting-a-pr)
  - [Understand the Continuous Integration process](submit-your-pr.md#understand-the-continuous-integration-process)
  - [Choose a correct title for your PR](submit-your-pr.md#choose-a-correct-title-for-your-pr)
  - [Perform the PR template checklist](submit-your-pr.md#perform-the-PR-template-checklist)
  - [Understand the CI automated builds & tests](submit-your-pr.md#automated-builds-&-tests)
  - [Understand & make sure your PR meets the CI code test coverage requirements](submit-your-pr.md#code-test-coverage)

Need help? Start with the [Contributors FAQ](faq/)

## Contributing to Argo CD dependencies
 - [contributing to argo-ui](dependencies.md#argo-ui-components)
 - [contributing to gitops-engine](dependencies.md#gitops-engine-githubcomargoprojgitops-engine)


## Contributing to Argo Website
The Argo website is maintained in the [argo-site](https://github.com/argoproj/argo-site) repository.