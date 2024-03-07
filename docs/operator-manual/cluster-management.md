# Cluster Management

This guide is for operators looking to manage clusters on the CLI. If you want to use Kubernetes resources for this, check out [Declarative Setup](./declarative-setup.md#clusters).

Not all commands are described here, see the [argocd cluster Command Reference](../user-guide/commands/argocd_cluster.md) for all available commands.

## Adding a cluster

Run `argocd cluster add context-name`.

If you're unsure about the context names, run `kubectl config get-contexts` to get them all listed.

This will connect to the cluster and install the necessary resources for ArgoCD to connect to it.
Note that you will need privileged access to the cluster.

## Removing a cluster

Run `argocd cluster rm context-name`.

This removes the cluster with the specified name.

!!!note "in-cluster cannot be removed"
    The `in-cluster` cluster cannot be removed with this. If you want to disable the `in-cluster` configuration, you need to update your `argocd-cm` ConfigMap. Set [`cluster.inClusterEnabled`](./argocd-cm-yaml.md) to `"false"`
