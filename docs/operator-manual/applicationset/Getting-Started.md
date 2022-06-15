# Getting Started

This guide assumes you are familiar with Argo CD and its basic concepts. See the [Argo CD documentation](../../core_concepts.md) for more information.
    
## Requirements

* Installed [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) command-line tool
* Have a [kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/) file (default location is `~/.kube/config`).

## Installation

There are a few options for installing the ApplicationSet controller.


### A) Install ApplicationSet as part of Argo CD

Starting with Argo CD v2.3, the ApplicationSet controller is bundled with Argo CD. It is no longer necessary to install the ApplicationSet controller separately from Argo CD.

Follow the [Argo CD Getting Started](../../getting_started.md) instructions for more information.



### B) Install ApplicationSet into an existing Argo CD install (pre-Argo CD v2.3)

**Note**: These instructions only apply to versions of Argo CD before v2.3.0.

The ApplicationSet controller *must* be installed into the same namespace as the Argo CD it is targeting.

Presuming that Argo CD is installed into the `argocd` namespace, run the following command:

```bash
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/applicationset/v0.4.0/manifests/install.yaml
```

Once installed, the ApplicationSet controller requires no additional setup.

The `manifests/install.yaml` file contains the Kubernetes manifests required to install the ApplicationSet controller:

- CustomResourceDefinition for `ApplicationSet` resource
- Deployment for `argocd-applicationset-controller`
- ServiceAccount for use by ApplicationSet controller, to access Argo CD resources
- Role granting RBAC access to needed resources, for ServiceAccount
- RoleBinding to bind the ServiceAccount and Role


<!-- ### C) Install development builds of ApplicationSet controller for access to the latest features

Development builds of the ApplicationSet controller can be installed by running the following command:
```bash
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/applicationset/master/manifests/install.yaml
```

With this option you will need to ensure that Argo CD is already installed into the `argocd` namespace.

How it works:

- After each successful commit to *argoproj/applicationset* `master` branch, a GitHub action will run that performs a container build/push to [`argoproj/argocd-applicationset:latest`](https://quay.io/repository/argoproj/argocd-applicationset?tab=tags )
- [Documentation for the `master`-branch-based developer builds](https://argocd-applicationset.readthedocs.io/en/master/)  is available from Read the Docs.

!!! warning
    Development builds contain newer features and bug fixes, but are more likely to be unstable, as compared to release builds.

See the `master` branch [Read the Docs](https://argocd-applicationset.readthedocs.io/en/master/) page for documentation on post-release features. -->


<!-- ## Upgrading to a Newer Release

To upgrade from an older release (eg 0.1.0, 0.2.0) to a newer release (eg 0.3.0), you only need to `kubectl apply` the `install.yaml` for the new release, as described under *Installation* above.

There are no manual upgrade steps required between any release of ApplicationSet controller, (including 0.1.0, 0.2.0, and 0.3.0) as of this writing, however, see the behaviour changes in ApplicationSet controller v0.3.0, below.

### Behaviour changes in ApplicationSet controller v0.3.0

There are no breaking changes, however, a couple of behaviours have changed from v0.2.0 to v0.3.0. See the [v0.3.0 upgrade page](upgrading/v0.2.0-to-v0.3.0.md) for details. -->
## Enabling high availability mode

To enable high availability, you have to set the command ``` --enable-leader-election=true  ``` in argocd-applicationset-controller container and increase the replicas. 

do following changes in manifests/install.yaml

```bash
    spec:
      containers:
      - command:
        - entrypoint.sh
        - argocd-applicationset-controller
        - --enable-leader-election=true
```

### Optional: Additional Post-Upgrade Safeguards

See the [Controlling Resource Modification](Controlling-Resource-Modification.md) page for information on additional parameters you may wish to add to the ApplicationSet Resource in `install.yaml`, to provide extra security against any initial, unexpected post-upgrade behaviour. 

For instance, to temporarily prevent the upgraded ApplicationSet controller from making any changes, you could:

- Enable dry-run
- Use a create-only policy
- Enable `preserveResourcesOnDeletion` on your ApplicationSets
- Temporarily disable automated sync in your ApplicationSets' template

These parameters would allow you to observe/control the behaviour of the new version of the ApplicationSet controller in your environment, to ensure you are happy with the result (see the ApplicationSet log file for details). Just don't forget to remove any temporary changes when you are done testing!

However, as mentioned above, these steps are not strictly necessary: upgrading the ApplicationSet controller should be a minimally invasive process, and these are only suggested as an optional precaution for extra safety.

## Next Steps

Once your ApplicationSet controller is up and running, proceed to [Use Cases](Use-Cases.md) to learn more about the supported scenarios, or proceed directly to [Generators](Generators.md) to see example `ApplicationSet` resources. 
