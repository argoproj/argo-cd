# Helm

## Values Files

Helm has the ability to use a different, or even multiple "values.yaml" files to derive its
parameters from. Alternate or multiple values file(s), can be specified using the `--values`
flag. The flag can be repeated to support multiple values files:

```bash
argocd app set helm-guestbook --values values-production.yaml
```

## Helm Parameters

Helm has the ability to set parameter values, which override any values in
a `values.yaml`. For example, `service.type` is a common parameter which is exposed in a Helm chart:

```bash
helm template . --set service.type=LoadBalancer
```

Similarly Argo CD can override values in the `values.yaml` parameters using `argo app set` command,
in the form of `-p PARAM=VALUE`. For example:

```bash
argocd app set helm-guestbook -p service.type=LoadBalancer
```

## Helm Release Name

By default the Helm release name is equal to the Application name to which it belongs. Sometimes, especially on a centralised ArgoCD, 
you may want to override that  name, and it is possible with the `release-name` flag on the cli:

```bash
argocd app set helm-guestbook --release-name myRelease
```

 or using the releaseName for yaml:

```yaml
source:
    helm:
      releaseName: myRelease
```

!!! warning "Important notice on overriding the release name"
    Please note that overriding the Helm release name might cause problems when the chart you are deploying is using the `app.kubernetes.io/instance` label. ArgoCD injects this label with the value of the Application name for tracking purposes. So when overriding the release name, the Application name will stop being equal to the release name. Because ArgoCD will overwrite the label with the Application name it might cause some selectors on the resources to stop working. In order to avoid this we can configure ArgoCD to use another label for tracking in the [ArgoCD configmap argocd-cm.yaml](./../operator-manual/argocd-cm.yaml) - check the lines describing `application.instanceLabelKey`.

## Helm Hooks

> v1.3 or later

Helm hooks are similar to [Argo CD hooks](resource_hooks.md). In Helm, a hook
is any normal Kubernetes resource annotated with the `helm.sh/hook` annotation. 

Argo CD supports many (most?) Helm hooks by mapping the Helm annotations onto Argo CD's own hook annotations: 

| Helm Annotation | Notes |
|---|---|
| `helm.sh/hook: crd-install` | Supported as equivalent to `argocd.argoproj.io/hook: PreSync`. |
| `helm.sh/hook: pre-delete` | Not supported. In Helm stable there are 3 cases used to clean up CRDs and 3 to clean-up jobs. |
| `helm.sh/hook: pre-rollback` | Not supported. Never used in Helm stable. |
| `helm.sh/hook: pre-install` | Supported as equivalent to `argocd.argoproj.io/hook: PreSync`. |
| `helm.sh/hook: pre-upgrade` | Supported as equivalent to `argocd.argoproj.io/hook: PreSync`. |
| `helm.sh/hook: post-upgrade` | Supported as equivalent to `argocd.argoproj.io/hook: PostSync`. |
| `helm.sh/hook: post-install` | Supported as equivalent to `argocd.argoproj.io/hook: PostSync`. |
| `helm.sh/hook: post-delete` | Not supported. Never used in Helm stable. |
| `helm.sh/hook: post-rollback` | Not supported. Never used in Helm stable. |
| `helm.sh/hook: test-success` | Not supported. No equivalent in Argo CD. |
| `helm.sh/hook: test-failure` | Not supported. No equivalent in Argo CD. |
| `helm.sh/hook-delete-policy` | Supported. See also `argocd.argoproj.io/hook-delete-policy`). |
| `helm.sh/hook-delete-timeout` | No supported. Never used in Helm stable |
| `helm.sh/hook-weight` | Supported as equivalent to `argocd.argoproj.io/sync-wave`. |

Unsupported hooks are ignored. In Argo CD, hooks are created by using `kubectl apply`, rather than `kubectl create`. This means that if the hook is named and already exists, it will not change unless you have annotated it with `before-hook-creation`.

!!! warning "'install' vs 'upgrade' vs 'sync'"
    Argo CD cannot know if it is running a first-time "install" or an "upgrade" - every operation is a "sync'. This means that, by default, apps that have `pre-install` and `pre-upgrade` will have those hooks run at the same time.

### Hook Tips

* Make your hook idempotent. 
* Annotate `crd-install` with `hook-weight: "-2"` to make sure it runs to success before any install or upgrade hooks.
* Annotate  `pre-install` and `post-install` with `hook-weight: "-1"`. This will make sure it runs to success before any upgrade hooks.
* Annotate `pre-upgrade` and `post-upgrade` with `hook-delete-policy: before-hook-creation` to make sure it runs on every sync.

Read more about [Argo hooks](resource_hooks.md) and [Helm hooks](https://github.com/kubernetes/helm/blob/master/docs/charts_hooks.md). 

## Random Data

Helm templating has the ability to generate random data during chart rendering via the
`randAlphaNum` function. Many helm charts from the [charts repository](https://github.com/helm/charts)
make use of this feature. For example, the following is the secret for the
[redis helm chart](https://github.com/helm/charts/blob/master/stable/redis/templates/secret.yaml):

```yaml
data:
  {{- if .Values.password }}
  redis-password: {{ .Values.password | b64enc | quote }}
  {{- else }}
  redis-password: {{ randAlphaNum 10 | b64enc | quote }}
  {{- end }}
```

The Argo CD application controller periodically compares Git state against the live state, running
the `helm template <CHART>` command to generate the helm manifests. Because the random value is
regenerated every time the comparison is made, any application which makes use of the `randAlphaNum`
function will always be in an `OutOfSync` state. This can be mitigated by explicitly setting a
value, in the values.yaml such that the value is stable between each comparison. For example:

```bash
argocd app set redis -p password=abc123
```

