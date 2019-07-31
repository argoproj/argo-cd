# Comparison with Other CD Tools

We are often asked what is the difference between various CD tools including Argo CD. This is a really difficult question to answer, particularly without bias. But it is an important question, so we try our best to answer it here. If you would like to provide more details or fix an inaccuracy, please feel free to create a pull-requestor open an issue.

The GitOps discipline is evolving and GitOps tools continue to change rapidly. We would like to keep this document up-to-date and will continue adding more details. 

## [Flux](https://github.com/fluxcd/flux)

Flux and Argo CD are very similar in scope. Both tools are primarily focused on CD, support popular config management tools
and use the operator pattern to continuously reconcile cluster state with desired state. Both tools allow using a Git repository as a source of Kubernetes manifests and automatically push changes into the cluster.

Although the tools are similar in scope, the design decisions and user experience are quite different. 

### Multi-Tenant Clusters

One Flux instance is able to manage the whole cluster or a single namespace within the cluster. Unlike Argo CD, Flux does not have its own RBAC and SSO integration. Instead Flux supports
multi-tenant cluster management using the pattern described at https://www.weave.works/blog/developing-apps-multitenant-clusters-with-gitops:

- One Flux with admin access manages the creation of namespaces for teams and installs a separate Flux instance into each team namespace
- Namespace-level Flux instances use team specific repositories and are configured to manage resources only in the team's namespace.

To address multi-tenancy, Argo CD introduces the Application abstraction, which allows the user to specify the source of manifests and destination cluster namespace for each Application.
Argo CD provides its own [RBAC](https://argoproj.github.io/argo-cd/operator-manual/rbac/) which isolates teams from each other. This design decision was made to 
provide a single control plane for the whole cluster, instead of having multiple deployment operators in multiple namespaces.

### Multi-Cluster Management

Flux applies manifests defined in Git into the same cluster where Flux itself is running. As a result, each Flux instance can manage only a single cluster.

Argo CD can run anywhere and can be used to manage multiple clusters and specify the target cluster using Application properties. This solves for two use cases:
- Provide a single control plane for all applications. State of all applications across multiple clusters can be maintained by a single Argo CD instance.
- GitOps operator as a service. Argo CD can be used by multiple teams but maintained by a central infrastructure team.

### CI Integration

Both Argo CD and Flux monitor the desired state Git repositories and push changes to Kubernetes on every target state update. However, each tool uses a different pattern to integrate with CI.

The Flux pattern is described at https://www.weave.works/blog/continuous-delivery-weave-flux/ :

1. CI pipeline builds and pushes new images to Docker registry.
1. Flux detects new image version and propagates the change into the Kubernetes cluster as well as matching Git repository.

Argo CD does not automate making changes to Git. A CI pipeline is responsible for updating configurations in Git as described at https://argoproj.github.io/argo-cd/user-guide/ci_automation/.

The following patterns are supported by Argo CD:

1. *Auto-Syncing*: after CI pipeline builds new image and updates configuration in Git, into Kubernetes cluster.
1. *CI driven syncing*. Argo CD detects changes but doesn't update resources in Kubernetes automatically. A CI pipeline uses Argo CD CLI to trigger syncing and access application
health state. The second way provides better control over the syncing process and enables the following use cases: run post-deployment tests after application updated and drive multi-step
deployments like blue-green or canary.

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
