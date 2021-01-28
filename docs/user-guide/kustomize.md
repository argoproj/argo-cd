# Kustomize

The following configuration options are available for Kustomize:

* `namePrefix` is a prefix appended to resources for Kustomize apps
* `nameSuffix` is a suffix appended to resources for Kustomize apps
* `images` is a list of Kustomize image overrides
* `commonLabels` is a string map of an additional labels
* `commonAnnotations` is a string map of an additional annotations
    
To use Kustomize with an overlay, point your path to the overlay.

!!! tip
    If you're generating resources, you should read up how to ignore those generated resources using the [`IgnoreExtraneous` compare option](compare-options.md).

## Private Remote Bases

If you have remote bases that are either (a) HTTPS and need username/password (b) SSH and need SSH private key, then they'll inherit that from the app's repo. 

This will work if the remote bases uses the same credentials/private key. It will not work if they use different ones. For security reasons your app only ever knows about its own repo (not other team's or users repos), and so you won't be able to access other private repos, even if Argo CD knows about them.

Read more about [private repos](private-repositories.md).

## `kustomize build` Options/Parameters

To provide build options to `kustomize build` add a property to the ArgoCD CM under data:

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
    kustomize.buildOptions: --load_restrictor none
```
## Custom Kustomize versions

Argo CD supports using multiple kustomize versions simultaneously and specifies required version per application.
To add additional versions make sure required versions are [bundled](../operator-manual/custom_tools.md) and then
use `kustomize.version.<version>` fields of `argocd-cm` ConfigMap to register bundled additional versions.   

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
    kustomize.version.v3.5.1: /custom-tools/kustomize_3_5_1
    kustomize.version.v3.5.4: /custom-tools/kustomize_3_5_4
```

Once a new version is configured you can reference it in Application spec as following:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
spec:
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    path: guestbook-kustomize

    kustomize:
      version: v3.5.4
```

Additionally application kustomize version can be configured using Parameters tab of Application Details page or using following CLI command:

```
argocd app set <appyName> --kustomize-version v3.5.4
```


## Build Environment

Kustomize does not support parameters and therefore cannot support the standard [build environment](build-environment.md).
