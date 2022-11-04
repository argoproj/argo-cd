# Testing outside the cluster

The `argocd-image-updater` binary provides means to test its behavior using the
`test` subcommand.

You can use this command from your workstation without any modifications to your
Argo CD installation or applications, and without having to install the image
updater in your Kubernetes cluster. The `test` command does not need to talk to
Argo CD, and only needs access to your Kubernetes cluster if you need to use
image pull secret stored in Kubernetes.

The `test` command's main purpose is to verify the behaviour of Argo CD
Image Updater on arbitrary images and to validate your configuration. For
example, the most simple form of running a test is the following:

```shell
argocd-image-updater test <image_name>
```

## Testing registry access

For example, to see what Argo CD Image Updater would consider the latest nginx
version on Docker Hub according to semantic versioning, you can run:

```shell
$ argocd-image-updater test nginx 
INFO[0000] getting image                                 image_name=nginx registry=
INFO[0002] Fetching available tags and metadata from registry  image_name=nginx
INFO[0004] Found 321 tags in registry                    image_name=nginx
INFO[0004] latest image according to constraint is nginx:1.19.5
```

## Multi-arch images

As stated in the section about
[multi-arch support](../basics/update.md#multi-arch),
Argo CD Image Updater by default only considers images in the registry for the
same platform as `argocd-image-updater` is executed on.

For the `test` command, this means it takes the platform of the system you are
running it on. If you are executing it for example on your Mac workstation, most
likely no results will yield at all - because there are simple no native images
for any of the `darwin` platforms.

You can specify the target platforms manually, using the `--platforms` command
line option, e.g. the following demonstrates no available images for the given
platform:

```shell
$ argocd-image-updater test gcr.io/heptio-images/ks-guestbook-demo --platforms darwin/amd64 --update-strategy latest
DEBU[0000] Creating in-cluster Kubernetes client        
INFO[0000] retrieving information about image            image_alias= image_name=gcr.io/heptio-images/ks-guestbook-demo registry_url=gcr.io
DEBU[0000] setting rate limit to 20 requests per second  prefix=gcr.io registry="https://gcr.io"
DEBU[0000] Inferred registry from prefix gcr.io to use API https://gcr.io 
INFO[0000] Fetching available tags and metadata from registry  application=test image_alias= image_name=gcr.io/heptio-images/ks-guestbook-demo registry_url=gcr.io
DEBU[0001] ignoring v2 manifest sha256:fe18e00a6aeece16b5b2f77a32ee60929e8a60e27c71df8df66bf804f5677f47. Manifest platform: linux/amd64, requested: darwin/amd64  application=test image_alias= image_name=gcr.io/heptio-images/ks-guestbook-demo registry_url=gcr.io
DEBU[0001] No metadata found for heptio-images/ks-guestbook-demo:0.1  application=test image_alias= image_name=gcr.io/heptio-images/ks-guestbook-demo registry_url=gcr.io
DEBU[0001] ignoring v2 manifest sha256:e6fbd640820c32c847a53b93eb7b404f25ffb56df1cc16d0711eec3a405d6509. Manifest platform: linux/amd64, requested: darwin/amd64  application=test image_alias= image_name=gcr.io/heptio-images/ks-guestbook-demo registry_url=gcr.io
DEBU[0001] No metadata found for heptio-images/ks-guestbook-demo:0.2  application=test image_alias= image_name=gcr.io/heptio-images/ks-guestbook-demo registry_url=gcr.io
INFO[0001] Found 0 tags in registry                      application=test image_alias= image_name=gcr.io/heptio-images/ks-guestbook-demo registry_url=gcr.io
INFO[0001] no newer version of image found               application=test image_alias= image_name=gcr.io/heptio-images/ks-guestbook-demo registry_url=gcr.io
```

While setting the `platforms` to include `linux/amd64`, yields the following:

```shell
$ argocd-image-updater test gcr.io/heptio-images/ks-guestbook-demo --platforms linux/amd64 --update-strategy latest
DEBU[0000] Creating in-cluster Kubernetes client        
INFO[0000] retrieving information about image            image_alias= image_name=gcr.io/heptio-images/ks-guestbook-demo registry_url=gcr.io
DEBU[0000] setting rate limit to 20 requests per second  prefix=gcr.io registry="https://gcr.io"
DEBU[0000] Inferred registry from prefix gcr.io to use API https://gcr.io 
INFO[0000] Fetching available tags and metadata from registry  application=test image_alias= image_name=gcr.io/heptio-images/ks-guestbook-demo registry_url=gcr.io
INFO[0001] Found 2 tags in registry                      application=test image_alias= image_name=gcr.io/heptio-images/ks-guestbook-demo registry_url=gcr.io
DEBU[0001] found 2 from 2 tags eligible for consideration  image=gcr.io/heptio-images/ks-guestbook-demo
INFO[0001] latest image according to constraint is gcr.io/heptio-images/ks-guestbook-demo:0.2  application=test image_alias= image_name=gcr.io/heptio-images/ks-guestbook-demo registry_url=gcr.io

```

## Testing for semver constraints

To see what it would consider the latest patch version within the 1.17 release,
run:

```shell
$ argocd-image-updater test nginx --semver-constraint 1.17.X
INFO[0000] getting image                                 image_name=nginx registry=
INFO[0002] Fetching available tags and metadata from registry  image_name=nginx
INFO[0004] Found 321 tags in registry                    image_name=nginx
INFO[0004] latest image according to constraint is nginx:1.17.10
```

## Testing different update strategies

You can test the result of different
[update strategies](../basics/update-strategies.md)
using the `--update-strategy` command line option, e.g.:

```shell
$ argocd-image-updater test ghcr.io/argoproj/argocd --update-strategy latest
DEBU[0000] Creating in-cluster Kubernetes client        
INFO[0000] retrieving information about image            image_alias= image_name=ghcr.io/argoproj/argocd registry_url=ghcr.io
DEBU[0000] setting rate limit to 20 requests per second  prefix=ghcr.io registry="https://ghcr.io"
DEBU[0000] Inferred registry from prefix ghcr.io to use API https://ghcr.io 
INFO[0000] Fetching available tags and metadata from registry  application=test image_alias= image_name=ghcr.io/argoproj/argocd registry_url=ghcr.io
INFO[0139] Found 864 tags in registry                    application=test image_alias= image_name=ghcr.io/argoproj/argocd registry_url=ghcr.io
DEBU[0139] found 864 from 864 tags eligible for consideration  image=ghcr.io/argoproj/argocd
INFO[0139] latest image according to constraint is ghcr.io/argoproj/argocd:2.4.0-f8390c94  application=test image_alias= image_name=ghcr.io/argoproj/argocd registry_url=ghcr.io
```

## Using credentials

If you need to specify 
[credentials](../basics/authentication.md#auth-registries),
you can do so using the `--credentials` parameter. It accepts the same values
as the corresponding
[annotation](../configuration/images.md#pull-secrets), i.e.:

```shell
$ export GITHUB_PULLSECRET="<username>:<token>"
$ argocd-image-updater test docker.pkg.github.com/argoproj/argo-cd/argocd --update-strategy latest --credentials env:GITHUB_PULLSECRET
INFO[0000] getting image                                 image_name=argoproj/argo-cd/argocd registry=docker.pkg.github.com
INFO[0000] Fetching available tags and metadata from registry  image_name=argoproj/argo-cd/argocd
INFO[0040] Found 100 tags in registry                    image_name=argoproj/argo-cd/argocd
INFO[0040] latest image according to constraint is docker.pkg.github.com/argoproj/argo-cd/argocd:1.8.0-9fb51f7a
```

For a complete list of available command line parameters, run
`argocd-image-updater test --help`.

It is recommended that you read about core updating and image concepts in the
[documentation](../../configuration/images/)
before using this command.
