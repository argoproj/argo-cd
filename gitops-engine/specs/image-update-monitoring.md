# Docker Image Update Monitoring

## Summary

Many GitOps users would like to automate Kubernetes manifest changes in the deployment repository
(see [Deployment Repo Update Automation](./deployment-repo-update.md)). The changes might be triggered by
the CI pipeline run or a new image in the Docker registry. Flux provides docker registry monitoring as part of
[Automated Image Update](https://docs.fluxcd.io/en/latest/references/automated-image-update.html) feature.

This document is meant to collect requirements for a component that provides docker registry monitoring functionality and 
can be used by Argo CD and potentially Flux users.

## Requirements

### Configurable Event Handler

When a new docker image is discovered the component should execute an event handler and pass the docker image name/version as a parameter.
The event handler is a shell script. The user should be able to specify the handler in the component configuration.

### Docker Registry WebHooks

Some Docker Registries send a webhook when a new image gets pushed. The component should provide a webhook handler which when invokes an event handler.

### Image Pulling

In addition to the webhook, the component should support images metadata pulling. The pulling should detect the new images and invoke an event handler for each new image.

### Image Credentials Auto-Discovering

If a component is running inside of a Kubernetes cluster together with the deployments then it already has access to the Docker registry credentials. Auto-Discovering functionality
detect available docker registry credentials and use them to access registries instead of requiring users to configure credentials manually.
