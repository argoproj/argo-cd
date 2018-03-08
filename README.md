
# Argo CD - GitOps Continuous Delivery for Kubernetes

## What is Argo CD?

Argo CD is a declarative, continuous delivery service based on ksonnet for Kubernetes.

## Why Argo CD?

Application definitions, configurations, and environments should be declarative and version controlled.
Application deployment and lifecycle management should be automated, auditable, and easy to understand.

## How it works

Argo CD uses git repositories as the source of truth for defining the desired application state as well as the target
deployment environments. Kubernetes manifests are specified as [ksonnet](https://ksonnet.io) applications and
implements automated services to deploy and maintain the desired application states in the specified target environments.

Argo CD is implemented as a kubernetes controller which continuously monitors running applications
and compares the current live state against the desired target state specified in the git repo.
A deployed application whose live state deviates from the target state is considered out-of-sync.
Argo CD reports & visualizes the differences as well as providing facilities to automatically or manually sync the live
state back to the desired target state. Any modifications made to the desired target state in the git repo is
automatically applied and reflected in the specified target environments.

## Features

* Automated deployment of applications to specified target environments
* Continuous monitoring of deployed applications
* Automated or manual syncing of applications to its target state
* Web and CLI based visualization of applications and differences between live vs. target state
* Rollback/Roll-anywhere to any application state committed in the git repository

## What is ksonnet?

* [Jsonnet](http://jsonnet.org), the basis for ksonnet, is a domain specific
configuration language, which provides extreme flexibility for composing and manipulating JSON/YAML specifications. 
* [Ksonnet](http://ksonnet.io), goes one step further by applying Jsonnet principles to Kubernetes
manifests. It also provides an opinionated file & directory structure to organize applications into
reusable components, parameters, and environments. Environments can be hierarchical, which promotes
re-use of application and environment specifications. 

## Why ksonnet?

Application configuration management is a hard problem and grows rapidly in complexity as you deploy
more applications, against more and more environments. Current templating systems, such as Jinja,
and Golang templating, are unnatural ways to maintain kubernetes manifests, and are not well suited to
capture subtle configuration differences between environments. The ability to compose and re-use
application and environment configurations is also limited.

Imagine we have a single guestbook application deployed in following environments:

| Environment        | K8s Version | Application Image      | DB Connection String  | Environment Vars | Sidecars      |
|--------------------|-------------|------------------------|-----------------------|------------------|---------------|
| minikube           | 1.10.0      | jesse/guestbook:latest | sql://locahost/db     | DEBUG=true       |               |
| dev                | 1.9.0       | app/guestbook:latest   | sql://dev-test/db     | DEBUG=true       |               |
| staging            | 1.8.0       | app/guestbook:e3c0263  | sql://staging/db      |                  | istio,dnsmasq |
| prod-us-west-1     | 1.8.0       | app/guestbook:abc1234  | sql://prod/db         | FOO_FEATURE=true | istio,dnsmasq |
| prod-us-west-2     | 1.8.0       | app/guestbook:abc1234  | sql://prod/db         |                  | istio,dnsmasq |
| prod-us-east-1     | 1.9.0       | app/guestbook:abc1234  | sql://prod/db         | BAR_FEATURE=true | istio,dnsmasq |

Ksonnet:
* Enables composition and re-use of YAML specifications
* Allows overrides, additions, and subtractions of YAML sub-components which are specific to each environment
* Guarantees proper generation of K8s manifests suitable for the corresponding Kubernetes API version
* Provides kubernetes-specific jsonnet libraries to enable concise definition of kubernetes manifests

## Development Status
* Argo CD is in early development

## Roadmap
* PreSync, PostSync, OutOfSync hooks
* Blue/Green & canary upgrades
* SSO Integration
* GitHub & Docker webhooks
