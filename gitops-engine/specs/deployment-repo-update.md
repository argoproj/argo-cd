# Deployment Repo Update Automation

## Summary

The GitOps driven continuous deployment cycle starts with a change in the Git repository that contains resource manifests. Flux provides the
[Automated Image Update](https://docs.fluxcd.io/en/latest/references/automated-image-update.html) feature that continuously monitors the docker registry and automatically
updates deployment repo when a new image is released. This functionality is not available for Argo CD users. Also, some Argo CD users need only functionality related to the
Git repository updating and don't need docker registry monitoring.

This document is meant to collect requirements for the Git repository update functionality. As a next step, we could discuss if it is possible to implement a Golang library or
a service that can be used in combination with Argo CD and Flux.

> Note: Flux already plans to split out the docker registry monitor and image updating feature into a separate component. We should consider re-using the extracted component.

## Requirements

### Manifests updating

When updates are discovered for any image referenced in resource manifests in the configuration repository, new manifests that refer to the updated image tags/versions must be generated.
The manifests might be stored as raw YAML files or as the templating tool package such as Kustomize or Helm. The manifest updating functionality should take new images
set as an input and update manifest files or templating tool configs to use the provided set of images.

### Commit signing

The user might want to use GPG signing for each commit in the deployment repo. The commit signing feature should allow to optionally
sign the commit with the image changes.

### Interaction with Git

The feature provides the following basic functionalities:
* Clone Git repo or update the local copy of a previously cloned copy
* Configure local Git user name and email.
* Push changes back to Git remote repo.
* Rebase remote changes in case of concurrent repository update.
