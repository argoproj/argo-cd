## Troubleshooting

The `argocd-notifications` binary includes a set of CLI commands that helps to configure the controller
settings and troubleshoot issues.

## Global flags
Following global flags are available for all sub-commands:

* `config-map` - path to the file containing `argocd-notifications-cm` ConfigMap. If not specified
then the command loads `argocd-notification-cm` ConfigMap using the local Kubernetes config file.
* `secret` - path to the file containing `argocd-notifications-secret` ConfigMap. If not
specified then the command loads `argocd-notification-secret` Secret using the local Kubernetes config file.
Additionally, you can specify `:empty` value to use empty secret with no notification service settings. 

**Examples:**

* Get list of triggers configured in the local config map:

```bash
argocd-notifications trigger get \
  --config-map ./argocd-notifications-cm.yaml --secret :empty
```

* Trigger notification using in-cluster config map and secret:

```bash
argocd-notifications template notify \
  app-sync-succeeded guestbook --recipient slack:argocd-notifications
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

You can download `argocd-notifications` from the github [release](https://github.com/argoproj-labs/argocd-notifications/releases)
attachments.

The binary is available in `argoprojlabs/argocd-notifications` image. Use the `docker run` and volume mount
to execute binary on any platform. 

**Example:**

```bash
docker run --rm -it -w /src -v $(pwd):/src \
  argoprojlabs/argocd-notifications:<version> \
  /app/argocd-notifications trigger get \
  --config-map ./argocd-notifications-cm.yaml --secret :empty
```

### In your cluster

SSH into the running `argocd-notifications-controller` pod and use `kubectl exec` command to validate in-cluster
configuration.

**Example**
```bash
kubectl exec -it argocd-notifications-controller-<pod-hash> \
  /app/argocd-notifications trigger get
```

## Commands

{!troubleshooting-commands.md!}

## Errors

{!troubleshooting-errors.md!}
