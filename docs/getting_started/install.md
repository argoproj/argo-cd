# Installing Argo CD server components

!!! tip "Want to upgrade?"
    If you're looking to upgrade an existing Argo CD installation to a newer
    version, please have a look at the
    [upgrade documentation](../../operations/upgrading/).

## Requirements

Argo CD is a Kubernetes-native application, and must be installed into a Kubernetes 
cluster in order to function properly. Regardless of which
[installation type](#installation-types) or
[installation methods](#installation-methods)
you chose, you will need a target cluster running a supported version of
Kubernetes, and you will need permissions to create resources in this cluster.

Depending on which
[installation type](#installation-types) you chose, the required permissions
will vary.

You need at least one cluster to install Argo CD, but you can later add other deployment targets that themselves do not need an Argo CD instance.



## Quick installation for trying out Argo CD

!!! warning 
    This installation method is only useful for demos and prototypes.

If you are impatient and just want to give Argo CD a quick try, make sure that
your current `kubectl` context is pointing to the cluster you want to install
Argo CD to, then run

```bash
kubectl create namespace argocd
kubectl -n argocd apply -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

This will install the current stable version of Argo CD into the `argocd`
namespace of your Kubernetes cluster. Then continue reading [the first steps](getting_started/first_steps/).

## Production-Grade Installation

When you are ready to use Argo CD in a production environment you have
several installation options.

For a production setup you should pay attention to extra aspects of your installation such as:

* defining the correct resource limits
* applying proper security constraints
* setting up high availability
* managing Argo CD upgrades
* monitoring Argo CD itself 

## Installation types

Argo CD provides various installation types, from which you need to pick the
one that meets your operational requirements.

### Cluster-scoped installation

The cluster-scoped installation type provides the most functionality, and is
the required installation type if you plan to manage cluster-scoped resources
with Argo CD. In order to install Argo CD in the cluster scope, you will need
cluster admin privileges on the cluster you are going to install Argo CD to.

The cluster-scoped installation will install additional `ClusterRoles` and
`ClusterRoleBindings` in your cluster, with elevated privileges.

This is the generally recommended installation type for most use-cases. It is
possible to lock-down a cluster-scoped installation later on.

### Namespace-scoped installation

The namespace-scoped installation type will limit the Argo CD installation to
manage only resources within the namespace on the cluster it is installed to.
This installation type will not be able to manage cluster-scoped resources on
the cluster it is installed to, but can be setup to manage cluster-scoped
resources on other remote clusters.

The namespace-scoped installation will install additional `Roles` and
`RoleBindings` within the namespace it is installed to.

This installation type cannot be easily upgraded to a cluster-scope later on,
and should be used if you do not have administrative privileges on the cluster
you are installing to.

## Installation variants

Both, the cluster-scoped and namespace-scoped installation manifests come in
two distinct variants:

* Standard
* High availability

As with the installation type, you should pick the one that meets your
operational requirements.

### Standard installation

The standard installation is suitable for most use cases in development or
pre-production environments, and doesn't need much resources on the cluster
it is installed to.

It will install all Argo CD workloads with a single replica, and also will
setup a single instance, non-clustered Redis cache.

This flavour can later be easily upgraded to a HA flavour.

### High-availability installation

The HA installation differs from the standard installation in that it will
start two replicas of `argocd-server` and `argocd-repo-server` each and
will install a clustered version of the Redis cache, using three replicas
of `argocd-redis-ha-server` in a `StatefulSet` manager.

The Argo CD workloads will also be setup with anti-affinity rules, so that the
replicas will be scheduled on different nodes.

Obviously, this flavour requires some more resources on the target cluster.
You can change the number of replicas later on, except for the Redis cache
which requires 3 replicas in order to function properly.

This is the recommended flavour for production environments.

## Installation process

### Pre-requisites

No matter what method you chose, you should make sure that you have all the
required privileges in the target cluster, and that the installation namespace
is created. So make sure your `kubectl` context points to your target cluster
and run:

```bash
kubectl create namespace argocd
```

### Using plain Kubernetes manifests

Argo CD provides ready-to-use, pre-rendered installation manifests for all
combinations of installation type and installation variant, as described above.

You can install these manifests using `kubectl apply`, directly from GitHub
or previously downloaded to your local machine. To install the current *stable*
version of Argo CD version, you can use one of the below commands:

*Cluster-scope, standard availability:*

```bash
kubectl -n argocd apply -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

*Cluster-scope, high availability:*

```bash
kubectl -n argocd apply -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/ha/install.yaml
```

*Namespace-scope, standard availability:*

```bash
kubectl -n argocd apply -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/namespace-install.yaml
```

*Namespace-scope, high availability:*

```bash
kubectl -n argocd apply -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/ha/namespace-install.yaml
```

### Using Kustomize

Argo CD provides Kustomize resources which you can use to create an installation
that is custom-tailored to your environment. You can use Kustomize to render the
manifests with your own configuration and settings.

If you plan to deviate from the default settings (and you most likely will for
production environments), using Kustomize is the recommended way of installing
Argo CD.

The following are minimal examples of `kustomization.yaml` files for each of the
installation flavours:

*Cluster-scope, standard availability:*

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- https://github.com/argoproj/argo-cd/manifests/cluster-install?ref=stable
```

*Cluster-scope, high availability:*

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- https://github.com/argoproj/argo-cd/manifests/ha/cluster-install?ref=stable
```

*Namespace-scope, standard availability:*

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- https://github.com/argoproj/argo-cd/manifests/namespace-install?ref=stable
```

*Namespace-scope, high availability:*

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- https://github.com/argoproj/argo-cd/manifests/ha/namespace-install?ref=stable
```

### Using Helm

While Argo CD provides no official Helm charts, there is an awesome, community
maintained chart for Argo CD available.

You can find this chart, along with installation instructions, at the
[Argo project's community managed Helm charts](https://github.com/argoproj/argo-helm/tree/master/charts/argo-cd)
GitHub repository.

Be aware that the Helm chart's maintainers do some things differently, and you
might not find all of the terminology used here in the Chart.

Also, please note that this is not an officially supported installation method.
Please direct all questions or problems you face using the Helm chart directly
to the chart's maintainers.

## Alternative installation methods

You can also an infrastructure tool such as [Terraform](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs), [Pulumi](https://www.pulumi.com/registry/packages/kubernetes/) and [Crossplane](https://github.com/crossplane-contrib/provider-kubernetes) to manage your Argo CD installation.

!!! tip "Use Argo CD to manage itself"
    It is also possible to install an Argo CD instance and manage it with itself like any other Kubernetes application. You might find interesting the [Argo Autopilot project](https://argocd-autopilot.readthedocs.io/en/stable/) that does exactly that (among other features).
    It installs Argo CD and sets it up to manage itself.

## Post installation

After installation, you should take some time to
[lockdown your installation](operations/security).