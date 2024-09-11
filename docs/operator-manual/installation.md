# Installation

Argo CD has two type of installations: multi-tenant and core.

## Multi-Tenant

The multi-tenant installation is the most common way to install Argo CD. This type of installation is typically used to service multiple application developer teams
in the organization and maintained by a platform team.

The end-users can access Argo CD via the API server using the Web UI or `argocd` CLI. The `argocd` CLI has to be configured using `argocd login <server-host>` command
(learn more [here](../user-guide/commands/argocd_login.md)).

Two types of installation manifests are provided:

### Non High Availability:

Not recommended for production use. This type of installation is typically used during evaluation period for demonstrations and testing.

* [install.yaml](https://github.com/argoproj/argo-cd/blob/master/manifests/install.yaml) - Standard Argo CD installation with cluster-admin access. Use this
  manifest set if you plan to use Argo CD to deploy applications in the same cluster that Argo CD runs
  in (i.e. kubernetes.svc.default). It will still be able to deploy to external clusters with inputted
  credentials.

  > Note: The ClusterRoleBinding in the installation manifest is bound to a ServiceAccount in the argocd namespace. 
  > Be cautious when modifying the namespace, as changing it may cause permission-related errors unless the ClusterRoleBinding is correctly adjusted to reflect the new namespace.

* [namespace-install.yaml](https://github.com/argoproj/argo-cd/blob/master/manifests/namespace-install.yaml) - Installation of Argo CD which requires only
  namespace level privileges (does not need cluster roles). Use this manifest set if you do not
  need Argo CD to deploy applications in the same cluster that Argo CD runs in, and will rely solely
  on inputted cluster credentials. An example of using this set of manifests is if you run several
  Argo CD instances for different teams, where each instance will be deploying applications to
  external clusters. It will still be possible to deploy to the same cluster (kubernetes.svc.default)
  with inputted credentials (i.e. `argocd cluster add <CONTEXT> --in-cluster --namespace <YOUR NAMESPACE>`).

  > Note: Argo CD CRDs are not included into [namespace-install.yaml](https://github.com/argoproj/argo-cd/blob/master/manifests/namespace-install.yaml).
  > and have to be installed separately. The CRD manifests are located in the [manifests/crds](https://github.com/argoproj/argo-cd/blob/master/manifests/crds) directory.
  > Use the following command to install them:
  > ```
  > kubectl apply -k https://github.com/argoproj/argo-cd/manifests/crds\?ref\=stable
  > ```

### High Availability:

High Availability installation is recommended for production use. This bundle includes the same components but tuned for high availability and resiliency.

* [ha/install.yaml](https://github.com/argoproj/argo-cd/blob/master/manifests/ha/install.yaml) - the same as install.yaml but with multiple replicas for
  supported components.

* [ha/namespace-install.yaml](https://github.com/argoproj/argo-cd/blob/master/manifests/ha/namespace-install.yaml) - the same as namespace-install.yaml but
  with multiple replicas for supported components.

## Core

The Argo CD Core installation is primarily used to deploy Argo CD in
headless mode. This type of installation is most suitable for cluster
administrators who independently use Argo CD and don't need
multi-tenancy features. This installation includes fewer components
and is easier to setup. The bundle does not include the API server or
UI, and installs the lightweight (non-HA) version of each component.

Installation manifest is available at [core-install.yaml](https://github.com/argoproj/argo-cd/blob/master/manifests/core-install.yaml).

For more details about Argo CD Core please refer to the [official
documentation](./core.md)

## Kustomize

The Argo CD manifests can also be installed using Kustomize. It is recommended to include the manifest as a remote resource and apply additional customizations
using Kustomize patches.


```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: argocd
resources:
- https://raw.githubusercontent.com/argoproj/argo-cd/v2.7.2/manifests/install.yaml
```

For an example of this, see the [kustomization.yaml](https://github.com/argoproj/argoproj-deployments/blob/master/argocd/kustomization.yaml)
used to deploy the [Argoproj CI/CD infrastructure](https://github.com/argoproj/argoproj-deployments#argoproj-deployments).

#### Installing Argo CD in a Custom Namespace
If you want to install Argo CD in a namespace other than the default argocd, you can use Kustomize to apply a patch that updates the ClusterRoleBinding to reference the correct namespace for the ServiceAccount. This ensures that the necessary permissions are correctly set in your custom namespace.

Below is an example of how to configure your kustomization.yaml to install Argo CD in a custom namespace:
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: <your-custom-namespace>
resources:
  - https://raw.githubusercontent.com/argoproj/argo-cd/v2.7.2/manifests/install.yaml

patches:
  - patch: |-
      - op: replace
        path: /subjects/0/namespace
        value: <your-custom-namespace>
    target:
      kind: ClusterRoleBinding
```

This patch ensures that the ClusterRoleBinding correctly maps to the ServiceAccount in your custom namespace, preventing any permission-related issues during the deployment.

## Helm

The Argo CD can be installed using [Helm](https://helm.sh/). The Helm chart is currently community maintained and available at
[argo-helm/charts/argo-cd](https://github.com/argoproj/argo-helm/tree/main/charts/argo-cd).

## Supported versions

For detailed information regarding Argo CD's version support policy, please refer to the [Release Process and Cadence documentation](https://argo-cd.readthedocs.io/en/stable/developer-guide/release-process-and-cadence/).

## Tested versions

The following table shows the versions of Kubernetes that are tested with each version of Argo CD.

{!docs/operator-manual/tested-kubernetes-versions.md!}
