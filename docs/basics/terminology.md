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

GitOps is a set of best practices for application (and infrastructure) deployments. Argo CD implements these practices, so if you understand the principles of GitOps you can understand the decisions behind Argo CD. The GitOps principles are explained at [opengitops.dev](https://opengitops.dev/). In summary Argo CD works as GitOps controller that pulls automatically updates (principle 3) from manifests stored in Git (principle 2) that describe Kubernetes objects in a declarative manner (principle 1). The syncing process between Git and cluster is happening at regular intervals and works both way (principle 4).

## Application

The Application is one of the central entities in Argo CD. Kubernetes by itself does not describe what exactly constitutes an application. Argo CD fills this gap by introducing the Application entity that not only groups associated Kubernetes manifests but also defines the source of truth for these manifests in the form of a Git repository. At its simplest form an Argo CD application is an association between a Git repository and a target cluster.

## Project

Project is another entity introduced by Argo CD and is used as a way to group applications. You can use projects in any way you see fit (e.g. per team, per department) but in most cases each project is used to define different security constraints and rules. Projects are the way an operator
can segment and secure an Argo CD instance for different teams of developers.
Note that using projects is completely optional. Argo CD comes with a "default" project

## Cluster

A cluster is any compliant Kubernetes platform that you want to deploy an application to. A single Argo CD instance can manage multiple clusters. By default Argo CD can manage the cluster it was installed on, but you can add extra clusters as deployment targets which themselves do not need to run an Argo CD installation on their own. It is also possible to do the opposite and install multiple Argo CD instances in a single cluster and manage only specific namespaces with each instance.

## Git repository

A Git source is one of the central concepts under GitOps as it holds the source of truth for all your application. The basic Argo CD control loop is to compare each associated Git repository with each cluster deployment and see if there any differences. Argo CD can work with different types of Git providers and protocols and also supports both private and public Git repositories.


## Application set
apps-of-apps pattern



CLI
API
UI

Live state
Target state
Diff process
Sync process
Sync status
Refresh
Health status

Configuration tool
Application parameters

Sync waves
Hooks


CRDs

RBAC
SSO
HA





