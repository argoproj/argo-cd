
# Argo CD - GitOps Continuous Delivery for Kubernetes

## What is Argo CD?

Argo CD is a declarative, continuous delivery service for Kubernetes based on ksonnet.

## Why Argo CD?

We believe that application definitions, configuration, and environments should be completely
declarative and version controlled. Deploying applications should go through an automated,
auditable, and well understood process. Argo CD accomplishes this by using git repositories to
define the desired application state and provides automated procedures to deploy from this state.

## How it works

Argo CD runs as a service, and is pointed to git repository(s) containing kuberentes manifests 
defined as [ksonnet](https://ksonnet.io) applications, and the cluster(s) in which to deploy these
applications to. The git repository is considered the single source of truth, and reflects the
target state of an application.

Argo CD is implemented as a kubernetes controller which continuously monitors running applications
and compares the live state against the target state (the git repo). A deployed application whose
live state deviates from the target state is considered out-of-sync. Argo CD reports & visualizes
the differences, as well as providing the facilities to automatically or manually sync the live
state back to the target state.

## Features
* Continous monitoring of deployed applications
* Automated or manual syncing of applications to its target state
* Web and CLI based visualization of applications and differences between live vs. target state
* Rollback/Roll-anywhere to any application state committed in the git repository

## Why ksonnet?

Configuration management is a hard problem to solve. This problem grows exponentially as you deploy
more applications, against more and more environments. Current templating systems, such as Jinja,
and Golang templating, are unnatural ways to maintain kubernetes manifests, and are ill-suited to
capture subtle configuration differences between environments. The ability to compose and re-use
parts of is limited.

* [Jsonnet](http://jsonnet.org), the language in which Ksonnet is built on, is a domain specific
configuration language which provides extreme flexibility of composing and manipulating JSON/YAML. 
* [Ksonnet](http://ksonnet.io), goes one step further by applying Jsonnet principles to Kubernetes
manifests. It provides an opinionated file & directory structure to organize an application into
resable components, parameters, and environments. Environments can be hierarchical, which promotes
re-use of parts of your application. 

Imagine we have a single guestbook application deployed in following environments:

| Environment        | K8s Version | Application Images      | DB Connection String | Environment Vars | Sidecars      |
|--------------------|-------------|------------------------|-----------------------|------------------|---------------|
| minikube           | 1.10.0      | jesse/guestbook:latest | sql://locahost/db     | DEBUG=true       |               |
| dev                | 1.9.0       | app/guestbook:latest   | sql://dev-test/db     | DEBUG=true       |               |
| staging            | 1.8.0       | app/guestbook:e3c0263  | sql://staging/db      |                  | istio,dnsmasq |
| prod-us-west-1     | 1.8.0       | app/guestbook:abc1234  | sql://prod/db         | FOO_FEATURE=true | istio,dnsmasq |
| prod-us-west-2     | 1.8.0       | app/guestbook:abc1234  | sql://prod/db         |                  | istio,dnsmasq |
| prod-us-central-1  | 1.9.0       | app/guestbook:abc1234  | sql://prod/db         | BAR_FEATURE=true | istio,dnsmasq |
| prod-us-east-1     | 1.8.0       | app/guestbook:abc1234  | sql://prod/db         |                  | istio,dnsmasq |

Ksonnet solves the problem of:
* Composition and re-use of the common parts of the YAML specification
* Overrides, additions, subtractions of YAML parts which are customized to each environment
* Proper generation of K8s manifests suitable for the corresponding Kubernetes API version 
 (e.g. deployments 1.9 apps/v1 vs. deployments 1.8 apps/v1beta2)
* Providing kubernetes-specific jsonnet libraries to enable concise definition of kubernetes manifest

## Development Status
* Argo CD is in early development stages

## Roadmap
* PreSync, PostSync, OutOfSync hooks
* Blue/Green & canary upgrades
* SSO Integration
* GitHub & Docker webhooks
