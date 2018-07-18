# Resource Hooks

## Overview

Hooks are ways to interject custom logic before, during, and after a Sync operation. Some use cases
for hooks are:
* Using a `PreSync` hook to perform a database schema migration before deploying a new version of the app.
* Using a `Sync` hook to orchestrate a complex deployment requiring more sophistication than the
kubernetes rolling update strategy (e.g. a blue/green deployment).
* Using a `PostSync` hook to run integration and health checks after a deployment.

## Usage
Hooks are simply kubernetes manifests annotated with the `argocd.argoproj.io/hook` annotation. To
make use of hooks, simply add the annotation to any resource:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  generateName: schema-migrate-
  annotations:
    argocd.argoproj.io/hook: PreSync
```

During a Sync operation, ArgoCD will create the resource during the appropriate stage of the
deployment. Hooks can be any type of Kuberentes resource kind, but tend to be most useful as
[Kubernetes Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/jobs-run-to-completion/)
or [Argo Workflows](https://github.com/argoproj/argo). Multiple hooks can be specified as a comma
separated list.

## Available Hooks
The following hooks are defined:

| Hook | Description |
|------|-------------|
| `PreSync` | Executes prior to the apply of the manifests. |
| `Sync`  | Executes after all `PreSync` hooks completed and were successful. Occurs in conjuction with the apply of the manifests. |
| `Skip` | Indicates to ArgoCD to skip the apply of the manifest. This is typically used in conjunction with a `Sync` hook which is presumably handling the deployment in an alternate way (e.g. blue-green deployment) |
| `PostSync` | Executes after all `Sync` hooks completed and were successful, a succcessful apply, and all resources in a `Healthy` state. |


## Hook Deletion Policies

Hooks can be deleted in an automatic fashion using the annotation: `argocd.argoproj.io/hook-delete-policy`.

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  generateName: integration-test-
  annotations:
    argocd.argoproj.io/hook: PostSync
    argocd.argoproj.io/hook-delete-policy: OnSuccess
```

The following policies define when the hook will be deleted.

| Policy | Description |
|--------|-------------|
| `OnSuccess` | The hook resource is deleted after the hook succeeded (e.g. Job/Workflow completed successfully). |
| `OnFailure` | The hook resource is deleted after the hook failed. |
