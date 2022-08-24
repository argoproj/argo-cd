# Configuring images for update

## Annotation format

You can specify one or more image(s) for each application that should be
considered for updates. To specify those images, the following annotation
is used:

```yaml
argocd-image-updater.argoproj.io/image-list: <image_spec_list>
```

The `<image_spec_list>` is a comma separated list of image specifications. Each
image specification is composed of mandatory and optional information, and is
used to specify the image, its version constraint and a few meta data.

An image specification could be formally described as:

```text
[<alias_name>=]<image_path>[:<version_constraint>]
```

Specifying the fields denoted in square brackets is optional and can be left
out.

## Allowing an image for update

The most simple form of specifying an image allowed to update would be the
following:

```yaml
argocd-image-updater.argoproj.io/image-list: nginx
```

The above example would specify to update the image `nginx` to it's most recent
version found in the container registry, without taking any version constraints
into consideration.

This is most likely not what you want, because you could pull in some breaking
changes when `nginx` releases a new major version and the image gets updated.
So you can give a version constraint along with the image specification:

```yaml
argocd-image-updater.argoproj.io/image-list: nginx:~1.26
```

The above example would allow the `nginx` image to be updated to any patch
version within the `1.26` minor release.

More information on how to specify semantic version constraints can be found
in the
[documentation](https://github.com/Masterminds/semver#checking-version-constraints)
of the [Semver library](https://github.com/Masterminds/semver) we're using.

!!!note
    If you use an
    [update strategy](#update-strategies)
    other than `semver` or `digest`, the `version_constraint` will not be have any effect
    and all tags returned from the registry will be considered for update. If
    you need to further restrict the list of tags to consider, see
    [filtering tags](#filtering-tags)
    below.

### Forcing Image Updates

By default, Image Updater will only update an image that is actually used in your Application
(i.e., is it exported in the Status of your ArgoCD Application.)

To support custom resources and things like PodTemplates that don't actually create a container,
you may force an update:

```yaml
argocd-image-updater.argoproj.io/image-list: myalias=some/image
argocd-image-updater.argoproj.io/myalias.force-update: "true"
```

## Assigning aliases to images

It's possible (and sometimes necessary) to assign an alias name to any given
image. Alias names should consist of alphanumerical characters only, and must
be unique within the same application. Re-using an alias name across different
applications is allowed.

An alias name is assigned during image specification in the `image-list`
annotation, for example the following would assign the alias `myalias` to the
image `some/image`:

```yaml
argocd-image-updater.argoproj.io/image-list: myalias=some/image
```

Assigning an alias name to an image is necessary in these scenarios:

* If you need to specify the Helm parameters used for rendering the image name
  and version using Helm and the parameter names do not equal `image.name` and
  `image.tag`. In this case, the name is just symbolic.

* If you want to set custom options for a given image's update strategy, or
  require referencing unique pull secrets for each image

The alias you assign to any image will be reused as a key in the annotations
used to define further options, so a little care should be taken when defining
such a name. It's generally advised to use only alpha-numerical characters. The
character `/` (forward-slash) can be used in the name, but must be referenced
as `_` (underscore) in the annotation. This is a limit of Kubernetes. So for
example, if you assign the alias `argoproj/argocd` to your image, the
appropriate key in the annotation would be referenced as `argoproj_argocd`.

!!!note
    It is generally recommended to set aliases for each of your images. Many of
    the features depend on an alias being set, and aliases might become a strict
    requirement in the future.

## Update strategies

Argo CD Image Updater can update images according to the following strategies:

|Strategy|Description|
|--------|-----------|
|`semver`| Update to the tag with the highest allowed semantic version|
|`latest`| Update to the tag with the most recent creation date|
|`name`  | Update to the tag with the latest entry from an alphabetically sorted list|
|`digest`| Update to the most recent pushed version of a mutable tag|

You can define the update strategy for each image independently by setting the
following annotation to an appropriate value:

```yaml
argocd-image-updater.argoproj.io/<image_name>.update-strategy: <strategy>
```

If no update strategy is given, or an invalid value was used, the default
strategy `semver` will be used.

!!!warning
    As of November 2020, Docker Hub has introduced pull limits for accounts on
    the free plan and unauthenticated requests. The `latest` update strategy
    will perform manifest pulls for determining the most recently pushed tags,
    and these will count into your pull limits. So unless you are not affected
    by these pull limits, it is **not recommended** to use the `latest` update
    strategy with images hosted on Docker Hub.

## Filtering tags

You can specify an expression that is matched against each tag returned from
the registry. On a positive match, the tag will be included in the list of tags
that will be considered to update the image to. If the expression does not
match the tag, the tag will not be included in the list. This allows you to
only consider tags that you are generally interested in.

You can define a tag filter by using the following annotation:

```yaml
argocd-image-updater.argoproj.io/<image_name>.allow-tags: <match_func>
```

The following match functions are currently available:

|Function|Description|
|--------|-----------|
|`regexp:<expression>`|Matches the tag name against the regular expression `<expression>`|
|`any`|Will match any tag|

If you specify an invalid match function, or the match function is misconfigured
(i.e. an invalid regular expression is supplied), no tag will be matched at all
to prevent considering (and possibly update to) the wrong tags by accident.

If the annotation is not specified, a match function `any` will be used to match
the tag names, effectively performing no filtering at all.

## Ignoring certain tags

If you want to ignore certain tags from the registry for any given image, you
can define a comma separated list of glob-like patterns using the following
annotation:

```yaml
argocd-image-updater.argoproj.io/<image_name>.ignore-tags: <pattern1>[, <pattern2>, ...]
```

You can use glob patterns as described in this
[documentation](https://golang.org/pkg/path/filepath/#Match)

If you want to disable updating an image temporarily, without removing all of
the configuration, you can do so by just ignoring all tags, effectively
preventing the image updater to consider any of the tags returned from the
registry:

```yaml
argocd-image-updater.argoproj.io/<image_name>.ignore-tags: "*"
```

Please note that regular expressions are not supported to be used for patterns.

## <a name="platforms"></a>Image platforms

By default, Argo CD Image Updater will only consider images from the registry
that are built for the same platform as the one Argo CD Image Updater is
running on. In multi-arch clusters, your workloads may be targeted to a
different platform, and you can configure the allowed platforms for a given
image.

For example, when Argo CD Image Updater is running on a `linux/amd64` node but
your application will be executed on a node with `linux/arm64` platform, you
need to let Argo CD Image Updater know:

```yaml
argocd-image-updater.argoproj.io/<image_alias>.platforms: linux/arm64
```

You can specify multiple allowed platforms as a comma separated list of allowed
platforms:

```yaml
argocd-image-updater.argoproj.io/<image_alias>.platforms: linux/arm64,linux/amd64
```

The correct image to execute will be chosen by Kubernetes.

!!!note
    The `platforms` annotation only has effect for images that use an update
    strategy which fetches meta-data. Currently, these are the `latest` and
    `digest` strategies. For `semver` and `name` strategies, the `platforms`
    setting has no effect.

## <a name="pull-secrets"></a>Specifying pull secrets

There are generally two ways on how to specify pull secrets for Argo CD Image
Updater to use. Either you configure a secret reference globally for the
container registry (as described [here](../registries/)), or you can specify
the pull secret to use for a given image using the annotation

```yaml
argocd-image-updater.argoproj.io/<image_name>.pull-secret: <secret_ref>
```

A configuration for an image will override what is configured for the registry,
for that certain image.

The `secret_ref` can either be a reference to a secret or a reference to an
environment variable. If a secret is referenced, the secret must exist in the
cluster where Argo CD Image Updater is running in (or has access to).

Valid values for `secret_ref` are:

* `secret:<namespace>/<secret_name>#<field>` - Use credentials stored in the
  field `field` from secret `secret_name` in namespace `namespace`.

* `pullsecret:<namespace>/<secret_name>` - Use credentials stored in the secret
  `secret_name` in namespace `namespace`. The secret is treated as Docker pull
  secret, that is, it must have a valid Docker config in JSON format in the
  field `.dockerconfigjson`.

* `env:<variable_name>` - Use credentials supplied by the environment variable
  named `variable_name`. This can be a variable that is i.e. bound from a
  secret within your pod spec.

* `ext:<path_to_script>` - Use credentials generated by a script. The script
  to execute must be specified using an absolute path, and must have the
  executable bit set. The script is supposed to output the credentials to be
  used as a single line to stdout, in the format `<username>:<password>`.
  Please note that the script will be executed every time Argo CD Image
  Updater goes to find a new version, and credentials will not be cached. If
  you want it to execute only once and cache credentials, you should configure
  this secret on the registry level instead.

In case of `secret` or `env`references, the data stored in the reference must
be in format `<username>:<password>`

## Custom images with Kustomize

In Kustomize, if you want to use an image from another registry or a completely
different image than what is specified in the manifests, you can give the image
specification as follows.

First of all, you will have to set up an `image_alias` for your image so you
are able to provide additional configuration for it:

```yaml
argocd-image-updater.argoproj.io/image-list: <image_alias>=<image_name>:<image_tag>
```

In this case, `image_name` should be the name of the image that you want to 
update to, rather than the currently running image.

To provide the original image name, you need to set the `kustomize.image-name`
annotation to the original image's name, like follows:

```yaml
argocd-image-updater.argoproj.io/<image_alias>.kustomize.image-name: <original_image_name>
```

Let's take Argo CD's Kustomize base as an example: The original image used by
Argo CD is `quay.io/argoproj/argocd`, pulled from Quay container registry. If
you want to follow the latest builds, as published on the GitHub registry, you
could override the image specification in Kustomize as follows:

```yaml
argocd-image-updater.argoproj.io/image-list: argocd=ghcr.io/argoproj/argocd
argocd-image-updater.argoproj.io/argocd.kustomize.image-name: quay.io/argoproj/argocd
```

Under the hood, this would be similar to the following kustomize command:

```shell
kustomize edit set image quay.io/argoproj/argocd=ghcr.io/argoproj/argocd
```

Finally, if you have not yet overridden the image name in your manifests (i.e.
there's no image `ghcr.io/argoproj/argocd` running in your application, you
may need to tell Image Updater to force the update despite no image is running:

```yaml
argocd-image-updater.argoproj.io/argocd.force-update: true
```

## Specifying Helm parameter names

In case of Helm applications which contain more than one image in the manifests
or use another set of parameters than `image.name` and `image.tag` to define
which image to render in the manifests, you need to set an `<image_alias>`
in the image specification to define an alias for that image, and then
use another set of annotations to specify the appropriate parameter names
that should get set if an image gets updated.

For example, if you have an image `quay.io/dexidp/dex` that is configured in
your helm chart using the `dex.image.name` and `dex.image.tag` Helm parameters,
you can set the following annotations on your `Application` resource so that
Argo CD Image Updater will know which Helm parameters to set:

```yaml
argocd-image-updater.argoproj.io/image-list: dex=quay.io/dexidp/dex
argocd-image-updater.argoproj.io/dex.helm.image-name: dex.image.name
argocd-image-updater.argoproj.io/dex.helm.image-tag: dex.image.tag

```

The general syntax for the two Helm specific annotations is:

```yaml
argocd-image-updater.argoproj.io/<image_alias>.helm.image-name: <name of helm parameter to set for the image name>
argocd-image-updater.argoproj.io/<image_alias>.helm.image-tag: <name of helm parameter to set for the image tag>
```

If the chart uses a parameter for the canonical name of the image (i.e. image
name and tag combined), a third option can be used:

```yaml
argocd-image-updater.argoproj.io/<image_alias>.helm.image-spec: <name of helm parameter to set for canonical name of image>
```

If the `<image_alias>.helm.image-spec` annotation is set, the two other
annotations `<image_alias>.helm.image-name` and `<image_alias>.helm.image-tag`
will be ignored.

## Examples

### Following an image's patch branch

*Scenario:* You have deployed image `nginx:1.19.1` and want to make sure it's
always up-to-date to the latest patch level within the `1.19` branch.

*Solution:* Use standard `semver` update strategy with a constraint on the
patch level (`~`), i.e.

```yaml
argocd-image-updater.argoproj.io/image-list: nginx:~1.19
```

### Always deploy the latest build

*Scenario:* Your CI regularly pushes images built from the latest source, using
some identifier (i.e. the hash of the Git commit) in the tag.

*Solution:*

1. Give your image a proper alias, i.e. `yourtool` and do not define a version
   constraint.

2. Use `latest` as update strategy

3. If you just want to consider a given set of tags, i.e. `v1.0.0-<hash>`, use a
  `allow-tags` annotation.

Annotations might look like follows:

```yaml
argocd-image-updater.argoproj.io/image-list: yourtool=yourorg/yourimage
argocd-image-updater.argoproj.io/yourtool.update-strategy: latest
argocd-image-updater.argoproj.io/yourtool.allow-tags: regexp:^v1.0.0-[0-9a-zA-Z]+$
```

### Multiple images in the same Helm chart

*Scenario:* You want to update multiple images within the same Helm chart to
their latest available version according to semver. 

The Helm parameters to set the image version
are `foo.image` and `foo.tag` for the first image, and `bar.image` and
`bar.tag` for the second image. The image names are `foo/bar` and `bar/foo`
for simplicity.

*Solution:*

1. Define an alias for both images, i.e. `fooalias` and `baralias`

2. Set `helm.image-name` and `helm.image-tag` for both aliases to their
   appropriate values

Annotations might look like follows:

```yaml
argocd-image-updater.argoproj.io/image-list: fooalias=foo/bar, baralias=bar/foo
argocd-image-updater.argoproj.io/fooalias.helm.image-name: foo.image
argocd-image-updater.argoproj.io/fooalias.helm.image-tag: foo.tag
argocd-image-updater.argoproj.io/baralias.helm.image-name: bar.image
argocd-image-updater.argoproj.io/baralias.helm.image-tag: bar.tag
```

### Tracking an image's `latest` tag

*Scenario:* You want to track the latest build of a given tag, e.g. the `latest`
tag that many images use without having to restart your pods manually.

*Solution:*

1. Define an alias for your image, i.e. `fooalias`

2. Set the constraint of your image to the tag you want to track, e.g. `latest`

3. Set the update strategy for this image to `digest`

```yaml
argocd-image-updater.argoproj.io/image-list: fooalias=yourorg/yourimage:latest
argocd-image-updater.argoproj.io/fooalias.update-strategy: digest
```

When there's a new build for `yourorg/yourimage:latest` found in the registry,
Argo CD Image Updater will update your configuration to use the SHA256 sum of
the image, and Kubernetes will restart your pods automatically to have them
use the new image.

## Appendix

### <a name="appendix-annotations"></a>Available annotations

The following is a complete list of available annotations to control the
update strategy and set options for images. Please note, all annotations
must be prefixed with `argocd-image-updater.argoproj.io/`.

|Annotation name|Default value|Description|
|---------------|-------|-----------|
|`image-list`|*none*|Comma separated list of images to consider for update|
|`<image_alias>.update-strategy`|`semver`|The update strategy to be used for the image|
|`<image_alias>.force-update`|`"false"`|If set to "true" (with quotes), even images that are not currently deployed will be updated|
|`<image_alias>.allow-tags`|*any*|A function to match tag names from registry against to be considered for update|
|`<image_alias>.ignore-tags`|*none*|A comma-separated list of glob patterns that when match ignore a certain tag from the registry|
|`<image_alias>.pull-secret`|*none*|A reference to a secret to be used as registry credentials for this image|
|`<image_alias>.platform`|*none*|Only update to images for given platform(s). Comma separated list, e.g. `linux/amd64,linux/arm64`|
|`<image_alias>.helm.image-spec`|*none*|Name of the Helm parameter to specify the canonical name of the image, i.e. holds `image/name:1.0`. If this is set, other Helm parameter related options will be ignored.|
|`<image_alias>.helm.image-name`|`image.name`|Name of the Helm parameter used for specifying the image name, i.e. holds `image/name`|
|`<image_alias>.helm.image-tag`|`image.tag`|Name of the Helm parameter used for specifying the image tag, i.e. holds `1.0`|
|`<image_alias>.kustomize.image-name`|*original name of image*|Name of Kustomize image parameter to set during updates|

### <a name="appendix-defaults"></a>Application-wide defaults

If you want to update multiple images in an Application, that all share common
settings (such as, update strategy, allowed tags, etc), you can define common
options. These options are valid for all images, unless an image overrides it
with specific configuration.

The following annotations are available. Please note, all annotations must be
prefixed with `argocd-image-updater.argoproj.io/`.

|Annotation name|Description|
|---------------|-----------|
|`update-strategy`|The update strategy to be used for all images|
|`force-update`|If set to "true" (with quotes), even images that are not currently deployed will be updated|
|`allow-tags`|A function to match tag names from registry against to be considered for update|
|`ignore-tags`|A comma-separated list of glob patterns that when match ignore a certain tag from the registry|
|`pull-secret`|A reference to a secret to be used as registry credentials for this image|
