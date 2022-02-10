Before using ArgoCD we assume that you already know about containers, tags and registries. Ideally you should also know how to build containers and publish them to a registry.

In addition, you should also be familiar with basic Kubernetes concepts such as:

 * [manifest](https://kubernetes.io/docs/concepts/cluster-administration/manage-deployment/)
 * [namespace](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/) 
 * [deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/)
 * other resources such as [secrets](https://kubernetes.io/docs/concepts/configuration/secret/), [services](https://kubernetes.io/docs/concepts/services-networking/service/), [configuration maps](https://kubernetes.io/docs/concepts/configuration/configmap/) etc.

Argo CD is a Continuous Delivery solution specifically created
for deploying containers to Kubernetes clusters, so a basic understanding
of both is required in order to use Argo CD effectively.


## GitOps

GitOps is a set of best practices for application (and infrastructure) deployments. Argo CD implements these practices, so if you understand the principles of GitOps you can understand the decisions behind Argo CD. The GitOps principles are explained at [opengitops.dev](https://opengitops.dev/). In summary Argo CD works as GitOps controller that pulls automatically updates (principle 3) from manifests stored in Git (principle 2) that describe Kubernetes objects in a declarative manner (principle 1). The syncing process between Git and cluster is happening at regular intervals and works both ways (principle 4).

## Application

The Application is one of the central entities in Argo CD. Kubernetes by itself does not describe what exactly constitutes an application. Argo CD fills this gap by introducing the Application entity that not only groups associated Kubernetes manifests but also defines the source of truth for these manifests in the form of a Git repository. At its simplest form an Argo CD application is an association between a Git repository and a target cluster.

## Project

Project is another entity introduced by Argo CD and is used as a way to group applications. You can use projects in any way you see fit (e.g. per team, per department) but in most cases each project is used to define different security constraints and rules. Projects are the way an operator
can segment and secure an Argo CD instance for different teams of developers.
Note that using projects is completely optional. Argo CD comes with a "default" project.

## Cluster

A cluster is any compliant Kubernetes platform that you want to deploy an application to. A single Argo CD instance can manage multiple clusters. By default Argo CD can manage the cluster it was installed on, but you can add extra clusters as deployment targets which themselves do not need to run an Argo CD installation on their own. It is also possible to do the opposite and install multiple Argo CD instances in a single cluster and manage only specific namespaces with each instance.

## Git repository

A Git source is one of the central concepts under GitOps as it holds the source of truth for all your application. The basic Argo CD control loop is to compare each associated Git repository with each cluster deployment and see if there any differences. Argo CD can work with different types of Git providers and protocols and also supports both private and public Git repositories.

## Apps-of-apps pattern

At its most simple form an Application is grouping individual Kubernetes resources (deployments, services etc.). But since the Application itself is a custom Kubernetes object, you can create Argo CD applications that recursively include other Applications. This pattern is called App-of-Apps and is a great way to group micro-services or otherwise related applications. Argo CD supports this pattern in many ways, (for example you can easily drill down in the application hierarchy in the Web UI), but does not enforce it in any way.

## Application set

The [Application Set controller](https://argocd-applicationset.readthedocs.io/en/stable/) is a subproject of Argo CD geared towards multi-cluster and multi-application Argo CD installations. It is a way to automatically create applications/clusters using different generators. For example you can create multiple applications [according to a set of folders in Git](https://argocd-applicationset.readthedocs.io/en/stable/Generators-Git/), or a set of environments according to the [Pull Requests](https://argocd-applicationset.readthedocs.io/en/stable/Generators-Pull-Request/) that you have open. Using Application Sets is optional.

## Command Line Interface (CLI)

Argo CD has a CLI called `argocd` that can manage applications, clusters and other entities. Using the CLI is great for automation and scripting. It can also be used from a Continuous Integration (CI) pipeline to automate deployments as part of a workflow. The CLI is optional and some people might prefer to use the web UI for the same actions.

## Web User Interface (UI)

Argo CD has a very powerful web interface that can be used to inspect your applications as well as drill down on their individual components. You can also perform common actions from within the UI (including deleting applications). Installing the UI is optional, and you can deploy Argo CD in headless mode without it. You can also lock down the UI using Access Control and User management. 

## Application Programming Interface (API)

Argo CD exposes an API that allows you to automate all possible actions from your own program or script. Everything that is available from the CLI and the UI is also available in API form. 

## Target state

The desired state of an application as described in Git. Typically it consists of Kubernetes manifests either in raw form or templated with Helm/Kustomize or other configuration tool. 

##  Live state

The state of the application as found in the cluster. It typically includes pods, services, secrets, configmaps and other Kubernetes objects.

## Diff process

The operation when the live state and target state are compared. If they are the same we know that what is in the cluster is also in Git and thus no action needs to be taken. If they are not the same, ArgoCD can take an action according to its configuration. A possible action is to apply changes from the Git state to the live state starting the sync process. It is also possible to customize the diff process to ignore specific fields.

## Sync process

The sync process takes places after the diff process and makes all the necessary changes to the live state in order to bring the cluster back to the same state as what is described in Git. There are many settings for the sync process that control if it is automated or not, what objects to ignore, if removing objects is allowed etc.

## Sync status

The sync status for all applications is monitored by Argo CD at all times. "Synced" denotes the case where live state and target state are the same. "Out Of Sync" means that there is a drift between the two of them.

## Refresh status

By default Argo CD will compare live state and target state every 3 minutes. You can change this period with a configuration setting. You can also "refresh" on demand any application from the UI or CLI forcing Argo CD to start the diff process. You can also start the diff process with webhooks from your Git provider.

## Sync waves

Sync waves allow you to customize the order of resource creation when an application is synced. For example you might want a service account object to be created before another Kubernetes resource that needs it.

## Resource Hooks

For each sync operation ArgoCD can optionally run other actions before/during/after the sync process. For example you can use the `PostSync` hook
to run some smoke tests in your new deployments. In the case of Helm, Argo CD also tries to translate [Helm hooks](https://helm.sh/docs/topics/charts_hooks/) to Argo CD hooks.

## Health status

This is an Argo CD specific status that is monitored for all applications. The "healthy" state is different per application type. For example, a Kubernetes ReplicaSet is deemed healthy if the live generation and live replicas match the target generation and desired replicas. Argo CD also has built-in health checks for other common resources such as [Argo Rollout Objects](https://argoproj.github.io/argo-rollouts/) or [Bitnami Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets).
You can create your own Health checks with the [Lua programming language](https://www.lua.org/).

## Configuration tool

Argo CD can use any supported templating tool such as Helm, Kustomize, JSonnet. You can add your own third party tool for preparing/templating Kubernetes manifests. Note that in the case of Helm, Argo CD uses it as a pure templating tool. Helm applications installed with Argo CD are not visible to normal Helm commands.

## Custom Resource Definitions (CRDs)

Argo CD defines CRDs for all its entities (applications, projects etc). Since CRDs can be stored declaratively in Git on their own, you can fully manage Argo CD configuration with Argo CD itself.

## Role Based Access Control

Argo CD includes a powerful RBAC mechanism on top of applications, projects, clusters and git repositories. With this mechanism Argo CD operators can lock down resources and restrict them only to a specific subset of users. RBAC configuration will also integrate with any of the supported Single Sign On (SSO) providers that might use in your organization.






