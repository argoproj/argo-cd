# Argo CD Image Updater

A tool to automatically update the container images of Kubernetes workloads
that are managed by
[Argo CD](https://github.com/argoproj/argo-cd).

!!!warning "A note on the current status"
    Argo CD Image Updater is under active development.

    You are welcome to test it out on non-critical environments, and of
    course to contribute by many means.

    There will be (probably a lot of) breaking changes from release to
    release as development progresses. We will do our
    best to indicate any breaking change and how to un-break it in the
    respective
    [release notes](https://github.com/argoproj/argo-cd/releases)

## Overview

The Argo CD Image Updater can check for new versions of the container images
that are deployed with your Kubernetes workloads and automatically update them
to their latest allowed version using Argo CD. It works by setting appropriate
application parameters for Argo CD applications, i.e. similar to
`argocd app set --helm-set image.tag=v1.0.1` - but in a fully automated
manner.

Usage is simple: You annotate your Argo CD `Application` resources with a list
of images to be considered for update, along with a version constraint to
restrict the maximum allowed new version for each image. Argo CD Image Updater
then regularly polls the configured applications from Argo CD and queries the
corresponding container registry for possible new versions. If a new version of
the image is found in the registry, and the version constraint is met, Argo CD
Image Updater instructs Argo CD to update the application with the new image.

Depending on your Automatic Sync Policy for the Application, Argo CD will either
automatically deploy the new image version or mark the Application as Out Of
Sync, and you can trigger the image update manually by syncing the Application.
Due to the tight integration with Argo CD, advanced features like Sync Windows,
RBAC authorization on Application resources etc. are fully supported.

## Features

!!!warning "Renamed image update strategies"
    The `latest` strategy has been renamed to `newest-build`, and `name` strategy has been renamed to `alphabetical`. 
    Please switch to the new convention as support for the old naming convention will be removed in future releases.

* Updates images of apps that are managed by Argo CD and are either generated
  from *Helm* or *Kustomize* tooling
* Update app images according to different
  [update strategies](./basics/update-strategies.md)
    * `semver`: update to highest allowed version according to given image
    constraint,
    * `latest/newest-build`: update to the most recently created image tag,
    * `name/alphabetical`: update to the last tag in an alphabetically sorted list
    * `digest`: update to the most recent pushed version of a mutable tag
* Support for 
  [widely used container registries](./configuration/registries.md#supported-registries)
* Support for private container registries via 
  [configuration](./configuration/registries.md#custom-registries)
* Can write changes
  [back to Git](./basics/update-methods.md#method-git)
* Ability to filter list of tags returned by a registry using matcher functions
* Support for custom, per-image 
  [pull secrets](./basics/authentication.md#auth-registries) (using generic K8s
  secrets, K8s pull secrets, environment variables or external scripts)
* Runs in a 
  [Kubernetes cluster](./install/installation.md#install-kubernetes) or can be
  used stand-alone from the command line
* Ability to perform parallel update of applications

## Limitations

The two most important limitations first. These will most likely not change
anywhere in the near future, because they are limitations by design.

Please make sure to understand these limitations, and do not send enhancement
requests or bug reports related to the following:

* The applications you want container images to be updated **must** be managed
  using Argo CD. There is no support for workloads not managed using Argo CD.

* Argo CD Image Updater can only update container images for applications whose
  manifests are rendered using either *Kustomize* or *Helm* and - especially
  in the case of Helm - the templates need to support specifying the image's
  tag (and possibly name) using a parameter (i.e. `image.tag`).

Otherwise, current known limitations are:

* Image pull secrets must exist in the same Kubernetes cluster where Argo CD
  Image Updater is running in (or has access to). It is currently not possible
  to fetch those secrets from other clusters.

## Questions, help and support

If you have any questions, need some help in setting things up or just want to
discuss something, feel free to

* open an issue on our GitHub issue tracker or

* join us in the `#argo-cd-image-updater` channel on the
  [CNCF slack](https://argoproj.github.io/community/join-slack/)
