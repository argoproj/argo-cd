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
