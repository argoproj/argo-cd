# Application configuration

Most of the runtime configuration for Argo CD Image Updater is performed by
setting various annotations on resources of type `Application`, which are
managed by Argo CD.

For its annotations, Argo CD Image Updater uses the following prefix:

```yaml
argocd-image-updater.argoproj.io
```

This section of the documentation tells you about which things you can
configure, and what annotations are available.

## <a name="application-mark"></a>Marking an application for being updateable

In order for Argo CD Image Updater to know which applications it should inspect
for updating the workloads' container images, the corresponding Kubernetes
resource needs to be correctly annotated. Argo CD Image Updater will inspect
only resources of kind `application.argoproj.io`, that is, your Argo CD
`Application` resources. Annotations on other kinds of resources will have no
effect and will not be considered.

As explained earlier, your Argo CD applications must be of either `Kustomize`
or `Helm` type. Other types of applications will be ignored.

So, in order for Argo CD Image Updater to consider your application for the
update of its images, at least the following criteria must be met:

* Your `Application` resource is annotated with the mandatory annotation of
  `argocd-image-updater.argoproj.io/image-list`, which contains at least one
  valid image specification (see [Images Configuration](images.md)).

* Your `Application` resource is of type `Helm` or `Kustomize`

An example of a correctly annotated `Application` resources might look like:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd-image-updater.argoproj.io/image-list: gcr.io/heptio-images/ks-guestbook-demo:^0.1
  name: guestbook
  namespace: argocd
spec:
  destination:
    namespace: guestbook
    server: https://kubernetes.default.svc
  project: default
  source:
    path: helm-guestbook
    repoURL: https://github.com/argocd-example-apps/argocd-example-apps
    targetRevision: HEAD
```

There is a whole chapter dedicated on how to
[configure images for update](../images).

## <a name="configure-write-back"></a>Configuring the write-back method

The Argo CD Image Updater supports two distinct methods on how to update images
of an application:

* *imperative*, via Argo CD API
* *declarative*, by pushing changes to a Git repository

Depending on your setup and requirements, you can chose the write-back method
per Application, but not per image. As a rule of thumb, if you are managing
`Application` in Git (i.e. in an *app-of-apps* setup), you most likely want
to chose the Git write-back method.

The write-back method is configured via an annotation on the `Application`
resource:

```yaml
argocd-image-updater.argoproj.io/write-back-method: <method>
```

Where `<method>` must be one of `argocd` (imperative) or `git` (declarative).

The default used by Argo CD Image Updater is `argocd`.

You can read more about the update methods and how to configure them in the
[chapter about update methods](../basics/update-methods.md)

## <a name="application-defaults"></a>Application defaults

Additional to per-image configuration, you can also set some defaults for the
application.

For example, if you have multiple images tracked in your application, and all
of them should use the `latest` update strategy, you can define this strategy
as the application's default and may omit any specific configuration for the
images' update strategy.
