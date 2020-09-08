# Argo CD Installation Manifests

Four sets of installation manifests are provided:

## Normal Installation:

* [install.yaml](install.yaml) - Standard Argo CD installation with cluster-admin access. Use this
  manifest set if you plan to use Argo CD to deploy applications in the same cluster that Argo CD runs
  in (i.e. kubernetes.svc.default). Will still be able to deploy to external clusters with inputted
  credentials.

* [namespace-install.yaml](namespace-install.yaml) - Installation of Argo CD which requires only
  namespace level privileges (does not need cluster roles). Use this manifest set if you do not
  need Argo CD to deploy applications in the same cluster that Argo CD runs in, and will rely solely
  on inputted cluster credentials. An example of using this set of manifests is if you run several
  Argo CD instances for different teams, where each instance will be deploying applications to
  external clusters. Will still be possible to deploy to the same cluster (kubernetes.svc.default)
  with inputted credentials (i.e. `argocd cluster add <CONTEXT> --in-cluster --namespace <YOUR NAMESPACE>`).

## High Availability:

* [ha/install.yaml](ha/install.yaml) - the same as install.yaml but with multiple replicas for
  supported components.

* [ha/namespace-install.yaml](ha/namespace-install.yaml) - the same as namespace-install.yaml but
  with multiple replicas for supported components.
