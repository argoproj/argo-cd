`argocd admin notifications` is a CLI command group that helps to configure the controller
settings and troubleshoot issues. Full command details are available in the [command reference](../../user-guide/commands/argocd_admin_notifications.md).

## Global flags
The following global flags are available for all sub-commands:

* `--config-map` - path to the file containing `argocd-notifications-cm` ConfigMap. If not specified
then the command loads `argocd-notification-cm` ConfigMap using the local Kubernetes config file.
* `--secret` - path to the file containing `argocd-notifications-secret` ConfigMap. If not
specified then the command loads `argocd-notification-secret` Secret using the local Kubernetes config file.
Additionally, you can specify `:empty` to use empty secret with no notification service settings. 

**Examples:**

* Get a list of triggers configured in the local config map:

    ```bash
    argocd admin notifications trigger get \
      --config-map ./argocd-notifications-cm.yaml --secret :empty
    ```

* Trigger notification using in-cluster config map and secret:

    ```bash
    argocd admin notifications template notify \
      app-sync-succeeded guestbook --recipient slack:argocd admin notifications
    ```

## Kustomize

If you are managing `argocd-notifications` config using Kustomize you can pipe whole `kustomize build` output
into stdin using `--config-map -` flag:

```bash
kustomize build ./argocd-notifications | \
  argocd-notifications \
  template notify app-sync-succeeded guestbook --recipient grafana:argocd \
  --config-map -
```

## How to get it

### On your laptop

You can download the `argocd` CLI from the GitHub [release](https://github.com/argoproj/argo-cd/releases)
attachments.

The binary is available in the `quay.io/argoproj/argocd` image. Use the `docker run` and volume mount
to execute binary on any platform. 

**Example:**

```bash
docker run --rm -it -w /src -v $(pwd):/src \
  quay.io/argoproj/argocd:<version> \
  /app/argocd admin notifications trigger get \
  --config-map ./argocd-notifications-cm.yaml --secret :empty
```

### In your cluster

SSH into the running `argocd-notifications-controller` pod and use `kubectl exec` command to validate in-cluster
configuration.

**Example**
```bash
kubectl exec -it argocd-notifications-controller-<pod-hash> \
  /app/argocd admin notifications trigger get
```

## Commands

The following commands may help debug issues with notifications:

* [`argocd admin notifications template get`](../../user-guide/commands/argocd_admin_notifications_template_get.md)
* [`argocd admin notifications template notify`](../../user-guide/commands/argocd_admin_notifications_template_notify.md)
* [`argocd admin notifications trigger get`](../../user-guide/commands/argocd_admin_notifications_trigger_get.md)
* [`argocd admin notifications trigger run`](../../user-guide/commands/argocd_admin_notifications_trigger_run.md)

## Errors

{!docs/operator-manual/notifications/troubleshooting-errors.md!}
