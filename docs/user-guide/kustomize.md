# Kustomize

The following configuration options are available for Kustomize:

* `namePrefix` is a prefix appended to resources for Kustomize apps
* `nameSuffix` is a suffix appended to resources for Kustomize apps
* `images` is a list of Kustomize image overrides
* `replicas` is a list of Kustomize replica overrides
* `commonLabels` is a string map of additional labels
* `forceCommonLabels` is a boolean value which defines if it's allowed to override existing labels
* `commonAnnotations` is a string map of additional annotations
* `namespace` is a kubernetes resources namespace
* `forceCommonAnnotations` is a boolean value which defines if it's allowed to override existing annotations
* `commonAnnotationsEnvsubst` is a boolean value which enables env variables substition in annotation  values
* `patches` is a list of Kustomize patches that supports inline updates

To use Kustomize with an overlay, point your path to the overlay.

!!! tip
    If you're generating resources, you should read up how to ignore those generated resources using the [`IgnoreExtraneous` compare option](compare-options.md).

## Patches
Patches are a way to kustomize resources using inline configurations in Argo CD applications.  This allows for kustomizing without  kustomization file.  `patches`  follow the same logic as the corresponding Kustomization.  Any patches that target existing Kustomization file will be merged.

The following Kustomization can be done similarly in an Argo CD application.
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
metadata:
  name: kustomize-inline-example
namespace: test1
resources:
  - https://raw.githubusercontent.com/argoproj/argocd-example-apps/master/guestbook/guestbook-ui-deployment.yaml
  - https://raw.githubusercontent.com/argoproj/argocd-example-apps/master/guestbook/guestbook-ui-svc.yaml
patches:
  - target:
      kind: Deployment
      name: guestbook-ui
    patch: |-
      - op: replace
        path: /spec/template/spec/containers/0/ports/0/containerPort
        value: 443
```
Application will clone the repository, use the specified path, then kustomize using inline patches configuration.
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: kustomize-inline-guestbook
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  destination:
    namespace: test1
    server: https://kubernetes.default.svc
  project: default
  source:
    path: guestbook
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: master
    kustomize:
      patches:
        - target:
            kind: Deployment
            name: guestbook-ui
          patch: |-
            - op: replace
              path: /spec/template/spec/containers/0/ports/0/containerPort
              value: 443
```

## Private Remote Bases

If you have remote bases that are either (a) HTTPS and need username/password (b) SSH and need SSH private key, then they'll inherit that from the app's repo.

This will work if the remote bases uses the same credentials/private key. It will not work if they use different ones. For security reasons your app only ever knows about its own repo (not other team's or users repos), and so you won't be able to access other private repos, even if Argo CD knows about them.

Read more about [private repos](private-repositories.md).

## `kustomize build` Options/Parameters

To provide build options to `kustomize build` of default Kustomize version, use `kustomize.buildOptions` field of `argocd-cm` ConfigMap. Use `kustomize.buildOptions.<version>` to register version specific build options.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
  labels:
    app.kubernetes.io/name: argocd-cm
    app.kubernetes.io/part-of: argocd
data:
    kustomize.buildOptions: --load-restrictor LoadRestrictionsNone
    kustomize.buildOptions.v4.4.0: --output /tmp
```
## Custom Kustomize versions

Argo CD supports using multiple Kustomize versions simultaneously and specifies required version per application.
To add additional versions make sure required versions are [bundled](../operator-manual/custom_tools.md) and then
use `kustomize.path.<version>` fields of `argocd-cm` ConfigMap to register bundled additional versions.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
  labels:
    app.kubernetes.io/name: argocd-cm
    app.kubernetes.io/part-of: argocd
data:
    kustomize.path.v3.5.1: /custom-tools/kustomize_3_5_1
    kustomize.path.v3.5.4: /custom-tools/kustomize_3_5_4
```

Once a new version is configured you can reference it in an Application spec as follows:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
spec:
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    path: kustomize-guestbook

    kustomize:
      version: v3.5.4
```

Additionally, the application kustomize version can be configured using the Parameters tab of the Application Details page, or using the following CLI command:

```bash
argocd app set <appName> --kustomize-version v3.5.4
```


## Build Environment

Kustomize apps have access to the [standard build environment](build-environment.md) which can be used in combination with a [config managment plugin](../operator-manual/config-management-plugins.md) to alter the rendered manifests.

## Kustomizing Helm charts

It's possible to [render Helm charts with Kustomize](https://github.com/kubernetes-sigs/kustomize/blob/master/examples/chart.md).
Doing so requires that you pass the `--enable-helm` flag to the `kustomize build` command.
This flag is not part of the Kustomize options within Argo CD.
If you would like to render Helm charts through Kustomize in an Argo CD application, you have two options:
You can either create a [custom plugin](https://argo-cd.readthedocs.io/en/stable/user-guide/config-management-plugins/), or modify the `argocd-cm` ConfigMap to include the `--enable-helm` flag globally for all Kustomize applications:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  kustomize.buildOptions: --enable-helm
```

## Setting the manifests' namespace

The `spec.destination.namespace` field only adds a namespace when it's missing from the manifests generated by Kustomize. It also uses `kubectl` to set the namespace, which sometimes misses namespace fields in certain resources (for example, custom resources). In these cases, you might get an error like this: `ClusterRoleBinding.rbac.authorization.k8s.io "example" is invalid: subjects[0].namespace: Required value.`

Using Kustomize directly to set the missing namespaces can resolve this problem. Setting `spec.source.kustomize.namespace` instructs Kustomize to set namespace fields to the given value.

If `spec.destination.namespace` and `spec.source.kustomize.namespace` are both set, Argo CD will defer to the latter, the namespace value set by Kustomize.
