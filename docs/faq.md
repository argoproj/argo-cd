# FAQ

## I've deleted/corrupted my repo and can't delete my app.

Argo CD can't delete an app if it cannot generate manifests. You need to either: 

1. Reinstate/fix your repo.
1. Delete the app using `--cascade=false` and then manually deleting the resources.

## Why is my application still `OutOfSync` immediately after a successful Sync?

See [Diffing](user-guide/diffing.md) documentation for reasons resources can be OutOfSync, and ways to configure
Argo CD to ignore fields when differences are expected.


## Why is my application stuck in `Progressing` state?

Argo CD provides health for several standard Kubernetes types. The `Ingress` and `StatefulSet` types have known issues which might cause health check
to return `Progressing` state instead of `Healthy`.

* `Ingress` is considered healthy if `status.loadBalancer.ingress` list is non-empty, with at least one value for `hostname` or `IP`. Some ingress controllers
 ([contour](https://github.com/heptio/contour/issues/403), [traefik](https://github.com/argoproj/argo-cd/issues/968#issuecomment-451082913)) don't update
 `status.loadBalancer.ingress` field which causes `Ingress` to stuck in `Progressing` state forever.

* `StatefulSet` is considered healthy if value of `status.updatedReplicas` field matches to `spec.replicas` field. Due to Kubernetes bug
[kubernetes/kubernetes#68573](https://github.com/kubernetes/kubernetes/issues/68573) the `status.updatedReplicas` is not populated. So unless you run Kubernetes version which
include the fix [kubernetes/kubernetes#67570](https://github.com/kubernetes/kubernetes/pull/67570) `StatefulSet` might stay in `Progressing` state.

As workaround Argo CD allows providing [health check](operator-manual/health.md) customization which overrides default behavior.

## I forgot the admin password, how do I reset it?

By default the password is set to the name of the server pod, as per [the getting started guide](getting_started.md).

To change the password, edit the `argocd-secret` secret and update the `admin.password` field with a new bcrypt hash. You
can use a site like https://www.browserling.com/tools/bcrypt to generate a new hash. For example:

```bash
# bcrypt(Password1!)=$2a$10$hDj12Tw9xVmvybSahN1Y0.f9DZixxN8oybyA32Uy/eqWklFU4Mo8O
kubectl -n argocd patch secret argocd-secret \
  -p "{\"data\": \
    {\
      \"admin.password\": \"$(echo -n '$2a$10$hDj12Tw9xVmvybSahN1Y0.f9DZixxN8oybyA32Uy/eqWklFU4Mo8O' | base64)\", \
      \"admin.passwordMtime\": \"$(date +%FT%T%Z | base64)\" \
  }}"
```

Another option is to delete both the `admin.password` and `admin.passwordMtime` keys and restart argocd-server. This will set the password back to the pod name as per [the getting started guide](getting_started.md).

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

## Why Is My App Out Of Sync Even After Syncing?

Is some cases, the tool you use may conflict with Argo CD by adding the `app.kubernetes.io/instance` label. E.g. using Kustomize common labels feature.

Argo CD automatically sets the `app.kubernetes.io/instance` label and uses it to determine which resources form the app. If the tool does this too, this causes confusion. You can change this label by setting the `application.instanceLabelKey` value in the `argocd-cm`.  We recommend that you use `argocd.argoproj.io/instance`. 

!!! note    When you make this change your applications will become out of sync and will need re-syncing.

See [#1482](https://github.com/argoproj/argo-cd/issues/1482).


# How Do I Fix "invalid cookie, longer than max length 4093"?

Argo CD uses a JWT as the auth token. You likely are part of many groups and have gone over the 4KB limit which is set for cookies.
You can get the list of groups by opening "developer tools -> network"

* Click log in
* Find the call to `<argocd_instance>/auth/callback?code=<random_string>`

Decode the token at https://jwt.io/. That will provide the list of teams that you can remove yourself from.

See [#2165](https://github.com/argoproj/argo-cd/issues/2165).

## Why Am I Getting `rpc error: code = Unavailable desc = transport is closing` When Using The CLI?

Maybe you're behind a proxy that does not support HTTP 2? Try the `--grcp-web` flag.:

```bash
argocd ... --grcp-web
```

## Why Am I Getting `x509: certificate signed by unknown authority` When Using The CLI?

Your not running your server with correct certs.

If you're not running in a production system (e.g. you're testing Argo CD out), try the `--insecure` flag:

```bash
argocd ... --insecure
```

!!! warning "Do not use `--insecure` in production"
