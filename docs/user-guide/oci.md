# OCI

## Declarative

Argo CD supports using OCI (Open Container Initiative) images as an application source. 
You can install applications using OCI images through the UI, or in the declarative GitOps way.  
Here is an example:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-custom-image
  namespace: argocd
spec:
  project: default
  source:
    path: .
    repoURL: oci://registry-1.docker.io/some-user/my-custom-image
    targetRevision: 1.16.1
  destination:
    server: "https://kubernetes.default.svc"
    namespace: my-namespace
```

Another example using a public OCI helm chart:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: nginx
spec:
  project: default
  source:
    path: .
    repoURL: oci://registry-1.docker.io/bitnamicharts/nginx 
    targetRevision: 15.9.0
    helm:
      valuesObject:
        some-value: foo
  destination:
    name: "in-cluster"
    namespace: nginx
```

The key to start using OCI images are the following components in the application spec:  

* `repoURL`: Specify the OCI image repository URL using the `oci://` scheme, followed by the registry and image name.
* `targetRevision`: Use this field to specify the desired image tag or digest.
* `path`: Use this field to select a relative path from the expanded image. If you don't want to select a subpath, use `.`.
In the case of OCI Helm charts (an OCI artifact where the `mediaType` is set to `application/vnd.cncf.helm.chart.content.v1.tar+gzip`), 
the path should always be set to `.`. 

### Private OCI Registries
The above configuration works only for public repositories. For private repositories, please refer to this [page](private-repositories.md#helm)
to use the proper flags `--enable-oci --type helm` when adding the repository Argo CD and use an application configuration like this:

> [!NOTE]
> The repository URL should not contain the OCI scheme prefix `oci://`.
> Also the path should be removed from the repository URL and should be defined instead in the `path` attribute.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-custom-image
  namespace: argocd
spec:
  project: default
  source:
    path: project/charts/mychart
    repoURL: europe-west4-docker.pkg.dev
    targetRevision: 1.16.1
  destination:
    server: "https://kubernetes.default.svc"
    namespace: my-namespace
```

## Usage Guidelines

First off, you'll need to have a repository that is OCI-compliant. As an example, DockerHub, ECR, GHCR and GCR all fit 
the bill.

Secondly, Argo CD expects an OCI image to contain a single layer. It also expects an OCI image to have a media type which 
is accepted by the Argo CD repository server. By default, Argo CD accepts one of the following media types for the image 
layer:

* `application/vnd.oci.image.layer.v1.tar+gzip`
* `application/vnd.cncf.helm.chart.content.v1.tar+gzip`

Custom media types can be configured by setting the `ARGOCD_REPO_SERVER_OCI_LAYER_MEDIA_TYPES` environment variable 
in the repo-server deployment.

To create an OCI artifact compatible with Argo CD, there are a multitude of tools to choose from. For this example we'll
use [ORAS](https://oras.land/). Navigate to the directory where your manifests are located and run `oras push`.

```shell
oras push <registry-url>/guestbook:latest .
```

ORAS will take care of packaging the directory to a single layer and setting the `mediaType` to 
`application/vnd.oci.image.layer.v1.tar+gzip`.

You can also package your OCI image using a compressed archive.

```shell
# Create a tarball of the directory containing your manifests. If you are not in the current directory, please ensure 
# that you are setting the correct parent of the directory (that is what the `-C` flag does).
tar -czvf archive.tar.gz -C manifests .
```

Then, you can push the archive to your OCI registry using ORAS:

```shell
# In the case of tarballs, you currently need to set the media type manually. 
oras push <registry-url>/guestbook:latest archive.tar.gz:application/vnd.oci.image.layer.v1.tar+gzip
```

## OCI Metadata Annotations

Argo CD can display standard OCI metadata annotations, providing additional context and information about your OCI 
images directly in the Argo CD UI.

### Supported Annotations

Argo CD recognizes and displays the following standard OCI annotations:

* `org.opencontainers.image.title`
* `org.opencontainers.image.description`
* `org.opencontainers.image.version`
* `org.opencontainers.image.revision`
* `org.opencontainers.image.url`
* `org.opencontainers.image.source`
* `org.opencontainers.image.authors`
* `org.opencontainers.image.created`

Using the previous example with ORAS, we can set annotations which Argo CD can make use of:

```shell
oras push -a "org.opencontainers.image.authors=some author" \
          -a "org.opencontainers.image.url=http://some-url" \
          -a "org.opencontainers.image.version=some-version" \
          -a "org.opencontainers.image.source=http://some-source" \
          -a "org.opencontainers.image.description=some description" \
          <registry-url>/guestbook:latest .
```
