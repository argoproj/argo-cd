# FAQ

## Why is my application still `OutOfSync` immediately after a successful Sync?

It is possible for an application to be `OutOfSync` even immediately after a successful Sync
operation. The reason might be a bug in the manifest, resource controller behavior
or a [mutating webhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#mutatingadmissionwebhook). 

To debug `OutOfSync` issues, run the `app diff` command to see the differences between git and live:
```
argocd app diff APPNAME
```

In case a difference is expected it is possible to exclude some resource fields using an Argo CD [diffing](diffing.md) customization.

## Why is my application stuck in `Progressing` state?

Argo CD provides health for several standard Kubernetes types. The `Ingress` and `StatefulSet` types have known issues which might cause health check
to return `Progressing` state instead of `Healthy`.

* `Ingress` is considered healthy if `status.loadBalancer.ingress` list is non-empty, with at least one value for `hostname` or `IP`. Some ingress controllers
 ([contour](https://github.com/heptio/contour/issues/403), [traefik](https://github.com/argoproj/argo-cd/issues/968#issuecomment-451082913)) don't update
 `status.loadBalancer.ingress` field which causes `Ingress` to stuck in `Progressing` state forever.

* `StatufulSet` is considered healthy if value of `status.updatedReplicas` field matches to `spec.replicas` field. Due to Kubernetes bug
[kubernetes/kubernetes#68573](https://github.com/kubernetes/kubernetes/issues/68573) the `status.updatedReplicas` is not populated. So unless you run Kubernetes version which
include the fix [kubernetes/kubernetes#67570](https://github.com/kubernetes/kubernetes/pull/67570) `StatefulSet` might stay in `Progressing` state.

As workaround Argo CD allows providing [health check](health.md) customization which overrides default behavior.
