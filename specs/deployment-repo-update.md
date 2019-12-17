# Deployment Repo Update Automation

## Summary

The GitOps driven continuous deployment cycle starts with a change in the Git repository that contains resource manifests. The [Flux] provides the
[Automated Image Update](https://docs.fluxcd.io/en/latest/references/automated-image-update.html) feature that continuously monitors the docker registry and automatically
updates deployment repo when a new image is released. Obviously that functionality is not available for  Argo CD users.
Also cannot or don't want to use docker-registry monitoring and only need functionality related to the Git repository update.

This document is meant to collect requirements for the Git repository update functionality. As a next step, we could discuss if it is possible to implement a Golang library or
a service that can be used in combination with Argo CD and Flux.

## Requirements

### Manifests updating.

After new images are discovered the resource manifests of each workflow referencing the image have to be updated. The manifests might be stored as raw YAML files or as the templating tool package such as Kustomize or Helm. The manifest updating functionality should take new images
set as an input and update manifest files or templating tool configs to use the provided set of images.

### Commit signing

The user might want to use GPC signing for each commit in the deployment repo. The commit signing feature should allow to optionally
sign the commit with the image changes.

### Git interaction

The Git interaction feature provides the following basic functionalities:
* Clone Git repo or update the local copy of a previously cloned copy
* Configure local Git user name and email.
* Push changes back to Git remote repo.
* Rebase remote changes in case of concurrent repository update.
