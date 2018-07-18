
# Argo CD - Declarative Continuous Delivery for Kubernetes

## What is Argo CD?

Argo CD is a declarative, continuous delivery service based on **ksonnet** for Kubernetes.

![Argo CD UI](docs/argocd-ui.gif)

## Why Argo CD?

Application definitions, configurations, and environments should be declarative and version controlled.
Application deployment and lifecycle management should be automated, auditable, and easy to understand.

## Getting Started

Follow our [getting started guide](docs/getting_started.md). Further [documentation](docs/)
is provided for additional features.

## How it works

Argo CD follows the **GitOps** pattern of using git repositories as the source of truth for defining the
desired application state. Kubernetes manifests are specified as [ksonnet](https://ksonnet.io)
applications. Argo CD automates the deployment of the desired
application states in the specified target environments.

![Argo CD Architecture](docs/argocd_architecture.png)

Application deployments can track updates to branches, tags, or pinned to a specific version of 
manifests at a git commit. See [tracking strategies](docs/tracking_strategies.md) for additional
details about the different tracking strategies available.

Argo CD is implemented as a kubernetes controller which continuously monitors running applications
and compares the current, live state against the desired target state (as specified in the git repo).
A deployed application whose live state deviates from the target state is considered out-of-sync.
Argo CD reports & visualizes the differences as well as providing facilities to automatically or
manually sync the live state back to the desired target state. Any modifications made to the desired
target state in the git repo can be automatically applied and reflected in the specified target
environments.

For additional details, see [architecture overview](docs/architecture.md).

## Features

* Automated deployment of applications to specified target environments
* Continuous monitoring of deployed applications
* Automated or manual syncing of applications to its desired state
* Web and CLI based visualization of applications and differences between live vs. desired state
* Rollback/Roll-anywhere to any application state committed in the git repository
* Health assessment statuses on all components of the application
* SSO Integration (OIDC, OAuth2, LDAP, SAML 2.0, GitLab, Microsoft, LinkedIn)
* Webhook Integration (GitHub, BitBucket, GitLab)
* PreSync, Sync, PostSync hooks to support complex application rollouts (e.g.blue/green & canary upgrades)

## What is ksonnet?

* [Jsonnet](http://jsonnet.org), the basis for ksonnet, is a domain specific configuration language,
which provides extreme flexibility for composing and manipulating JSON/YAML specifications.
* [Ksonnet](http://ksonnet.io) goes one step further by applying Jsonnet principles to Kubernetes
manifests. It provides an opinionated file & directory structure to organize applications into
reusable components, parameters, and environments. Environments can be hierarchical, which promotes
both re-use and granular customization of application and environment specifications.

## Why ksonnet?

Application configuration management is a hard problem and grows rapidly in complexity as you deploy
more applications, against more and more environments. Current templating systems, such as Jinja,
and Golang templating, are unnatural ways to maintain kubernetes manifests, and are not well suited to
capture subtle configuration differences between environments. Its ability to compose and re-use
application and environment configurations is also very limited.

Imagine we have a single guestbook application deployed in following environments:

| Environment   | K8s Version | Application Image      | DB Connection String  | Environment Vars | Sidecars      |
|---------------|-------------|------------------------|-----------------------|------------------|---------------|
| minikube      | 1.10.0      | jesse/guestbook:latest | sql://locahost/db     | DEBUG=true       |               |
| dev           | 1.11.0      | app/guestbook:latest   | sql://dev-test/db     | DEBUG=true       |               |
| staging       | 1.10.0      | app/guestbook:e3c0263  | sql://staging/db      |                  | istio,dnsmasq |
| us-west-1     | 1.9.0       | app/guestbook:abc1234  | sql://prod/db         | FOO_FEATURE=true | istio,dnsmasq |
| us-west-2     | 1.10.0      | app/guestbook:abc1234  | sql://prod/db         |                  | istio,dnsmasq |
| us-east-1     | 1.9.0       | app/guestbook:abc1234  | sql://prod/db         | BAR_FEATURE=true | istio,dnsmasq |

Ksonnet:
* Enables composition and re-use of common YAML specifications
* Allows overrides, additions, and subtractions of YAML sub-components specific to each environment
* Guarantees proper generation of K8s manifests suitable for the corresponding Kubernetes API version
* Provides [kubernetes-specific jsonnet libraries](https://github.com/ksonnet/ksonnet-lib) to enable
concise definition of kubernetes manifests

## Development Status
* Argo CD is being used in production to deploy SaaS services at Intuit

## Roadmap
* Audit trails for application events and API calls
* Service account/access key management for CI pipelines
* Revamped UI
* Customizable application actions
