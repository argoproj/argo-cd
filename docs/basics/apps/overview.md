# Argo CD Applications

## Overview

*Applications* are at the heart of Argo CD. An *Application* is the entity that
tells Argo CD where to find resources to deploy, where to deploy them and when
to do it.

You can think of an *Application* as a collection of one or more Kubernetes
resources that are managed together on a Kubernetes cluster. These resources can
be comprised of anything that is managable by the target Kubernetes cluster,
and can also possibly span over multiple namespaces. There is no artifical limit
of how many *Applications* you can configure in Argo CD, however, there might
be other limits (such as, compute resource constraints).

Each *Application* must be configured to have at least

* a unique
  [Name](#application-name),
* a relationship to a
  [Project](../projects.md),
* a [Source](source.md)
  to define the source of the *Application's* resources and
* a [Destination](destination.md)
  to define the target of the *Application's* resources.

Optionally, each *Application* can also have a
[Sync Policy](../syncing.md)
that controls how it will be synced to its destination.

The relationship between a *Source* and an *Application* is always 1:n. That
is, each *Application* must have exactly one *Source*, while you can create
multiple *Applications* from a single *Source*.

The same is true for the relationship between a *Destination* and an
*Application*, which is also alway 1:n. Each *Application* is managed on
exactly one *Destination*, but your *Destination* can contain multiple
*Applications*. This also means, you cannot install the same application to
multiple clusters, or multiple times on the same cluster.

Along with its configuration, each *Application* also has a
[state](state.md)
that represents its current reconciliation status, and a
[history](history.md)
which contains recordings of previous states and reconciliation results.

## Application name

An *Application name* defines the name of the application. Application names
are also the names of the Custom Resource in your cluster (defined using the
`.metadata.name` field of the CR) and therefore must be unique within your Argo
CD installation. It is not possible to have two applications with the same
name, regardless of their *Source* and *Destination* configuration.

It is recommended to use an easy to memorize naming scheme for applications,
especially if you are going to install a similar application to multiple
destinations. For example, if you have an *Application* you want to name
`monitoring`, and this application would be deployed to multiple clusters,

## Parent project

Each *Application* must belong to a parent
[project](../projects.md)
that specifies certain rules and additional configuration for *Applications*
that belong to it. The project is specified using the `.spec.project` field,
which must contain the *name* of the project to associate the application to.

Argo CD ships a default project named `default`, which can be used if you
haven't created other projects yet.

## Sync Policy

Each *Application* has a *Sync Policy* that defines how the *Application* should
be synced to the target *Cluster*. This policy is set in the `.spec.syncPolicy`
part of the *Application*.

Specifying a *Sync Policy* for an *Application* is *optional*. If no policy is
configured, the default policy will be used.

You can read more about *Sync Policies* in the
[Sync Policy documentation](../syncing.md).

## Implementation details

*Applications* are implemented as Kubernetes Custom Resources of kind
`Application` in the `argoproj.io/v1alpha1` API and can be managed either using
the Argo CD CLI, the web UI or the Kubernetes API.

!!! note "About the location of Application resources"
    *Application* resources live in the installation namespace in the cluster of
    your Argo CD installation, which is `argocd` by default. *Application* resources
    created in other namespaces or clusters will not be used up by Argo CD.
