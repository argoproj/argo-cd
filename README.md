[![slack](https://img.shields.io/badge/slack-argoproj-brightgreen.svg?logo=slack)](https://argoproj.github.io/community/join-slack)
[![codecov](https://codecov.io/gh/argoproj/argo-cd/branch/master/graph/badge.svg)](https://codecov.io/gh/argoproj/argo-cd)


# Argo CD - Declarative Continuous Delivery for Kubernetes

## What is Argo CD?

Argo CD is a declarative, GitOps continuous delivery tool for Kubernetes.

![Argo CD UI](docs/argocd-ui.gif)

## Why Argo CD?

Application definitions, configurations, and environments should be declarative and version controlled.
Application deployment and lifecycle management should be automated, auditable, and easy to understand.

## Getting Started

### Quickstart

```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/v0.11.0-rc2/manifests/install.yaml
```

Follow our [getting started guide](docs/getting_started.md). Further [documentation](docs/)
is provided for additional features.

## How it works

Argo CD follows the **GitOps** pattern of using git repositories as the source of truth for defining
the desired application state. Kubernetes manifests can be specified in several ways:
* [ksonnet](https://ksonnet.io) applications
* [kustomize](https://kustomize.io) applications
* [helm](https://helm.sh) charts
* Plain directory of YAML/json/jsonnet manifests

Argo CD automates the deployment of the desired application states in the specified target environments.
Application deployments can track updates to branches, tags, or pinned to a specific version of
manifests at a git commit. See [tracking strategies](docs/tracking_strategies.md) for additional
details about the different tracking strategies available.

For a quick 10 minute overview of Argo CD, check out the demo presented to the Sig Apps community
meeting:
[![Alt text](https://img.youtube.com/vi/aWDIQMbp1cc/0.jpg)](https://youtu.be/aWDIQMbp1cc?t=1m4s)


## Architecture

![Argo CD Architecture](docs/argocd_architecture.png)

Argo CD is implemented as a kubernetes controller which continuously monitors running applications
and compares the current, live state against the desired target state (as specified in the git repo).
A deployed application whose live state deviates from the target state is considered `OutOfSync`.
Argo CD reports & visualizes the differences, while providing facilities to automatically or
manually sync the live state back to the desired target state. Any modifications made to the desired
target state in the git repo can be automatically applied and reflected in the specified target
environments.

For additional details, see [architecture overview](docs/architecture.md).

## Features

* Automated deployment of applications to specified target environments
* Flexibility in support for multiple config management tools (Ksonnet, Kustomize, Helm, plain-YAML)
* Continuous monitoring of deployed applications
* Automated or manual syncing of applications to its desired state
* Web and CLI based visualization of applications and differences between live vs. desired state
* Rollback/Roll-anywhere to any application state committed in the git repository
* Health assessment statuses on all components of the application
* SSO Integration (OIDC, OAuth2, LDAP, SAML 2.0, GitLab, Microsoft, LinkedIn)
* Webhook Integration (GitHub, BitBucket, GitLab)
* PreSync, Sync, PostSync hooks to support complex application rollouts (e.g.blue/green & canary upgrades)
* Audit trails for application events and API calls
* Parameter overrides for overriding ksonnet/helm parameters in git
* Service account/access key management for CI pipelines

## Project Resources
* Argo GitHub:  https://github.com/argoproj
* Argo Slack:   [click here to join](https://argoproj.github.io/community/join-slack)
* Argo website: https://argoproj.github.io/
* Argo forum:   https://groups.google.com/forum/#!forum/argoproj


## Development Status
* Argo CD is actively developed and is being used in production to deploy SaaS services at Intuit

## Roadmap
### v0.11

* New application controller architecture
* Multi-namespaced applications
* Large application support
* Resource lifecycle hook improvements
* K8s recommended application labels
* External OIDC provider support
* OIDC group claims bindings to Project Roles
* Declarative Argo CD configuration
* Helm repository support

### v0.12
* Support for custom K8S manifest templating engines
* Support for custom health assessments (e.g. CRD health)
* Improved prometheus metrics
* Higher availability
* UI improvements
