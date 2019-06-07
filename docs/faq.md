# FAQ

## Why is my application still `OutOfSync` immediately after a successful Sync?

See [Diffing](user-guide/diffing.md) documentation for reasons resources can be OutOfSync, and ways to configure
Argo CD to ignore fields when differences are expected.


## Why is my application stuck in `Progressing` state?

Argo CD provides health for several standard Kubernetes types. The `Ingress` and `StatefulSet` types have known issues which might cause health check
to return `Progressing` state instead of `Healthy`.

* `Ingress` is considered healthy if `status.loadBalancer.ingress` list is non-empty, with at least one value for `hostname` or `IP`. Some ingress controllers
 ([contour](https://github.com/heptio/contour/issues/403), [traefik](https://github.com/argoproj/argo-cd/issues/968#issuecomment-451082913)) don't update
 `status.loadBalancer.ingress` field which causes `Ingress` to stuck in `Progressing` state forever.

* `StatufulSet` is considered healthy if value of `status.updatedReplicas` field matches to `spec.replicas` field. Due to Kubernetes bug
[kubernetes/kubernetes#68573](https://github.com/kubernetes/kubernetes/issues/68573) the `status.updatedReplicas` is not populated. So unless you run Kubernetes version which
include the fix [kubernetes/kubernetes#67570](https://github.com/kubernetes/kubernetes/pull/67570) `StatefulSet` might stay in `Progressing` state.

As workaround Argo CD allows providing [health check](operator-manual/health.md) customization which overrides default behavior.

## I forgot the admin password, how do I reset it?

Edit the `argocd-secret` secret and update the `admin.password` field with a new bcrypt hash. You
can use a site like https://www.browserling.com/tools/bcrypt to generate a new hash. Another option
is to delete both the `admin.password` and `admin.passwordMtime` keys and restart argocd-server.

## Argo CD cannot deploy Helm Chart based applications without internet access, how can I solve it?

Argo CD might fail to generate Helm chart manifests if the chart has dependencies located in external repositories. To solve the problem you need to make sure that `requirements.yaml`
uses only internally available Helm repositories. Even if the chart uses only dependencies from internal repos Helm might decide to refresh `stable` repo. As workaround override
`stable` repo URL in `argocd-cm` config map:

```yaml
data:
  helm.repositories: |
    - url: http://<internal-helm-repo-host>:8080
      name: stable
```

## I've configured [cluster secret](./operator-manual/declarative-setup.md#clusters) but it does not show up in CLI/UI, how do I fix it?

Check if cluster secret has `argocd.argoproj.io/secret-type: cluster` label. If secret has the label but the cluster is still not visible then make sure it might be a
permission issue. Try to list clusters using `admin` user (e.g. `argocd login --username admin && argocd cluster list`).

## Argo CD is unable to connect to my cluster, how do I troubleshoot it?

Use the following steps to reconstruct configured cluster config and connect to your cluster manually using kubectl:

```bash
kubectl exec -it <argocd-pod-name> bash # ssh into any argocd server pod
argocd-util kubeconfig https://<cluster-url> /tmp/config --namespace argocd # generate your cluster config
KUBECONFIG=/tmp/config kubectl get pods # test connection manually
```

Now you can manually verify that cluster is accessible from the Argo CD pod.

## How Can I Terminate A Sync?

To terminate the sync, click on the "synchronisation" then "terminate":

![Synchronization](assets/synchronization-button.png) ![Terminate](assets/terminate-button.png)