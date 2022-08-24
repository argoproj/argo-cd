# Updating container images

## General process overview

Argo CD Image Updater can update container images managed by one or more of
your Argo CD applications, according to how it is configured.

The workflow of Argo CD Image Updater can be described as follows:

* It scans for currently configured Argo CD `Application` resources either
  using the Kubernetes or the Argo CD API (depending on how you installed
  the Argo CD Image Updater). The `Applications` to consider can be further
  limited by having Argo CD Image Updater selecting only those applications
  whose name match a given pattern, or match a given label.

* It then goes through the list of `Applications` found and inspects each
  for the the annotation `argocd-image-updater.argoproj.io/image-list`. This
  annotation holds a list of image names that should be updated, and is a
  mandatory annotation for Argo CD Image Updater to indicate it should
  process this `Application`. Read more about the syntax expected in this
  annotations's value in the [marking images for update](#TODO)
  section in this doc.

* For each image found in the list, Argo CD Image Updater will first check
  if this image is actually deployed with the application. It does a strict
  check for the complete image name, including the registry the image is
  pulled from. For example, `docker.io/some/image` and `quay.io/some/image`,
  while both referring to `some/image`, are not considered equal. This strict
  behavior can be relaxed, however. See [forcing image updates](#TODO) for
  further explanation.

* If Argo CD Image Updater considers an image from the list eligible for an
  update check, it will connect the corresponding container registry to see
  if there is a newer version of the image according to the
  [update strategy](../strategies/)
  and other constraints that may have been configured for the image (e.g.
  a list of tags that are allowed to consider).

* If a newer version of an image was found, Argo CD Image Updater will try
  to update the image according to the configured
  [update method](../methods/). Please note that Argo CD Image Updater will
  never update your manifests, instead it re-configures your Application
  sources to use the new image tag, and control is handed over to Argo CD.

## <a name="multi-arch"></a>Multi-arch images and clusters

As of version 0.12, Argo CD Image Updater has full support for multi-arch
images (and multi-arch clusters) by being able to inspect images with multiple
manifests (i.e. a manifest list).

Multi-arch currently only is supported for
[update strategies](../basics/update-strategies.md)
which fetch image meta-data: `latest` and `digest`. Multi-arch will be ignored
for the update strategies that do not fetch meta-data, `semver` and `name`.

By default, Argo CD Image Updater will only consider updating to images that
have a manifest for the same platform where itself runs on. If you are on a
cluster that has nodes of multiple architectures, and are pinning certain
workloads to certain nodes, you will have to tell Argo CD Image Updater which
platforms are allowed for a certain application or an image. This can be done
by setting an appropriate
[annotation per image](../configuration/images.md#platforms)
or for all images of a given
[application as a default setting](../configuration/images.md#appendix-defaults).

Multi-arch is also implemented by the
[test command](../install/testing.md#multi-arch).

## Sync policies and image updates

As explained above, the Argo CD Image Updater will assume that Argo CD will
update the manifests in your cluster to use any new image that has been set
by the Argo CD Image Updater.

Argo CD Image Updater will work best with automatic syncing enabled for the
Argo CD applications that are being updated.

## Rollback and image updates

Currently, Argo CD Image Updater does not honor the rollback status of an
Argo CD application, and keeps updating to new images also for Applications
that are being rolled back. However, Argo CD will disable auto-sync for
such applications.

Honoring rollbacked applications correctly is on our roadmap.
