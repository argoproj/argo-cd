# Comparison with Other CD Tools

It is really difficult to provide a good comparison with other CD tools and avoid bias, but it feels very important. We keep getting questions about the pros and cons of different
tools so it seems valuable to collect this knowledge in a document.

The GitOps discipline keeps evolving and GitOps tools are changing rapidly. We would like to keep this document up-to-date and will continue adding more details. As a core project
team we obviously like and know Argo CD, but we also try very hard to avoid bias. If you want to provide more details or fix an inaccuracy please feel free to create a pull-request
or open an issue.

## [Flux](https://github.com/fluxcd/flux)

Flux and Argo CD are very similar in scope. Both tools are primarily focused on CD only, support popular config management tools
and use operator pattern to constantly reconcile cluster state. Similar to Flux, Argo CD allows using a Git repository as a source of Kubernetes manifests and automatically push 
changes into the cluster.

Although tools are similar in scope the design decisions and as a result user experience is quite different. 

### Multi-Tenant Clusters

One Flux instance is able to manage the whole cluster or a single namespace within the cluster. Unlike Argo CD, Flux does not have own RBAC nor SSO integration. Instead Flux support
multi-tenant cluster management using pattern described at https://www.weave.works/blog/developing-apps-multitenant-clusters-with-gitops:

- one Flux with admin access manages the creation of namespaces for the teams and install separate Flux instance into each team namespace
- namespace level Flux instances use team specific repositories and configured to manage resources only in team's namespace.

To solve that problem Argo CD introduces an abstraction in the form of Application. The Application allows the user to specify the source of manifests and destination cluster namespace.
Argo CD provides own [RBAC](https://argoproj.github.io/argo-cd/operator-manual/rbac/) which prevents one team to touch other's team namespaces. This design decision was made to 
provide a single control plane for the whole cluster, instead of having multiple deployment operators in multiple namespaces.

### Multi-Cluster Management

Flux applies manifests defined in Git into the same cluster where Flux itself is running. The external cluster management is not supported.

Argo CD allows to connect external clusters and specify the destination cluster in the Application properties. This allows to solve two use cases:
- Provide a single control plane for all applications. State of all applications in multiple clusters available via single service.
- GitOps operator as a service. Argo CD can be used by multiple teams and maintained by the central infrastructure team.

### CI Integration

Both Argo CD and Flux are monitoring expected state stored in Git repository and push changes to Kubernetes on every target state update. Tools are using a different way to integrate
with CI.

Flux pattern is described at https://www.weave.works/blog/continuous-delivery-weave-flux/ :

1. CI pipeline builds and pushes new images to Docker registry.
1. Flux detects new image version and propagates the change into Kubernetes cluster as well as matching Git repository.

Argo CD does not automate making changes to Git. CI pipeline is responsible for updating configuration in Git as described at https://argoproj.github.io/argo-cd/user-guide/ci_automation/.

Two following patterns are supported:

1. *Auto-Syncing*: after CI pipeline builds new image and updates configuration in Git, into Kubernetes cluster.
1. *CI driven syncing*. Argo CD detects changes but doesn't update resources in Kubernetes automatically. CI pipeline uses Argo CD CLI to trigger syncing and access application
health state. The second way provides tighter control over syncing process and enables the following use cases: run post-deployment tests after application updated; drive multi-step
deployment like blue-green/canary.

### Features Missing in Flux

1. *Health Assessment*. Argo CD continuously assess the health of application resources and surface aggregated application health.
1. *Web User Interface*. Argo CD UI is used by an infrastructure team to get a high level view of all applications. Developers team use UI as a Kubernetes dashboard.
1. *SSO Integration*.

## [Jenkins X](https://github.com/jenkins-x/jx)

TBD

## [Tekton](https://github.com/tektoncd)

TBD

## [Kube-Applier](https://github.com/box/kube-applier)

TBD
