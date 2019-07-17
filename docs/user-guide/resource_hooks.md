# Resource Hooks

!!! warning
    Helm hooks are currently ignored. [Read more](helm.md).

## Overview

Synchronization can be configured using resource hooks. Hooks are ways to interject custom logic before, during,
and after a Sync operation. Some use cases for hooks are:

* Using a `PreSync` hook to perform a database schema migration before deploying a new version of the app.
* Using a `Sync` hook to orchestrate a complex deployment requiring more sophistication than the
kubernetes rolling update strategy (e.g. a blue/green deployment).
* Using a `PostSync` hook to run integration and health checks after a deployment.
* Using a `SyncFail` hook to run clean-up or finalizer logic if a Sync operation fails.

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

During a Sync operation, Argo CD will create the resource during the appropriate stage of the
deployment. Hooks can be any type of Kuberentes resource kind, but tend to be most useful as a Pod,
[Job](https://kubernetes.io/docs/concepts/workloads/controllers/jobs-run-to-completion/)
or [Argo Workflows](https://github.com/argoproj/argo). Multiple hooks can be specified as a comma
separated list.

## Available Hooks
The following hooks are defined:

| Hook | Description |
|------|-------------|
| `PreSync` | Executes prior to the apply of the manifests. |
| `Sync`  | Executes after all `PreSync` hooks completed and were successful. Occurs in conjuction with the apply of the manifests. |
| `Skip` | Indicates to Argo CD to skip the apply of the manifest. This is typically used in conjunction with a `Sync` hook which is presumably handling the deployment in an alternate way (e.g. blue-green deployment) |
| `PostSync` | Executes after all `Sync` hooks completed and were successful, a succcessful apply, and all resources in a `Healthy` state. |
| `SyncFail` | Executes if and only if any part of the Sync operation fails. |


## Selective Sync

Hooks are run during [selective sync](selective_sync.md).

## Hook Deletion Policies

Hooks can be deleted in an automatic fashion using the annotation: `argocd.argoproj.io/hook-delete-policy`.

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  generateName: integration-test-
  annotations:
    argocd.argoproj.io/hook: PostSync
    argocd.argoproj.io/hook-delete-policy: HookSucceeded
```

The following policies define when the hook will be deleted.

| Policy | Description |
|--------|-------------|
| `HookSucceeded` | The hook resource is deleted after the hook succeeded (e.g. Job/Workflow completed successfully). |
| `HookFailed` | The hook resource is deleted after the hook failed. |

As an alternative to hook deletion policies, both Jobs and Argo Workflows support the
[`ttlSecondsAfterFinished`](https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/)
field in the spec, which let their respective controllers delete the Job/Workflow after it completes.

```yaml
spec:
  ttlSecondsAfterFinished: 600
```
