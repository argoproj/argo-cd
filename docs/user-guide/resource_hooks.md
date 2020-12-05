# Resource Hooks
## Overview

Synchronization can be configured using resource hooks. Hooks are ways to run scripts before, during,
and after a Sync operation. Hooks can also be run if a Sync operation fails at any point. Some use cases for hooks are:

* Using a `PreSync` hook to perform a database schema migration before deploying a new version of the app.
* Using a `Sync` hook to orchestrate a complex deployment requiring more sophistication than the
Kubernetes rolling update strategy.
* Using a `PostSync` hook to run integration and health checks after a deployment.
* Using a `SyncFail` hook to run clean-up or finalizer logic if a Sync operation fails. _`SyncFail` hooks are only available starting in v1.2_

## Usage

Hooks are simply Kubernetes manifests tracked in the source repository of your Argo CD Application annotated with `argocd.argoproj.io/hook`, e.g.:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  generateName: schema-migrate-
  annotations:
    argocd.argoproj.io/hook: PreSync
```

During a Sync operation, Argo CD will apply the resource during the appropriate phase of the
deployment. Hooks can be any type of Kubernetes resource kind, but tend to be Pod,
[Job](https://kubernetes.io/docs/concepts/workloads/controllers/jobs-run-to-completion/)
or [Argo Workflows](https://github.com/argoproj/argo). Multiple hooks can be specified as a comma
separated list.

The following hooks are defined:

| Hook | Description |
|------|-------------|
| `PreSync` | Executes prior to the application of the manifests. |
| `Sync`  | Executes after all `PreSync` hooks completed and were successful, at the same time as the application of the manifests. |
| `Skip` | Indicates to Argo CD to skip the application of the manifest. |
| `PostSync` | Executes after all `Sync` hooks completed and were successful, a successful application, and all resources in a `Healthy` state. |
| `SyncFail` | Executes when the sync operation fails. _Available starting in v1.2_ |

### Generate Name

Named hooks (i.e. ones with `/metadata/name`) will only be created once. If you want a hook to be re-created each time either use `BeforeHookCreation` policy (see below) or `/metadata/generateName`. 

## Selective Sync

Hooks are not run during [selective sync](selective_sync.md).

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
| `BeforeHookCreation` | Any existing hook resource is deleted before the new one is created (since v1.3). |

As an alternative to hook deletion policies, both Jobs and Argo Workflows support the
[`ttlSecondsAfterFinished`](https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/)
field in the spec, which let their respective controllers delete the Job/Workflow after it completes.

```yaml
spec:
  ttlSecondsAfterFinished: 600
```

## Using A Hook To Send A Slack Message

The following example uses the Slack API to send a a Slack message when sync completes or fails: 

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  generateName: app-slack-notification-
  annotations:
    argocd.argoproj.io/hook: PostSync
    argocd.argoproj.io/hook-delete-policy: HookSucceeded
spec:
  template:
    spec:
      containers:
      - name: slack-notification
        image: curlimages/curl
        command:
          - "curl"
          - "-X"
          - "POST"
          - "--data-urlencode"
          - "payload={\"channel\": \"#somechannel\", \"username\": \"hello\", \"text\": \"App Sync succeeded\", \"icon_emoji\": \":ghost:\"}"
          - "https://hooks.slack.com/services/..."
      restartPolicy: Never
  backoffLimit: 2
```

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  generateName: app-slack-notification-fail-
  annotations:
    argocd.argoproj.io/hook: SyncFail
    argocd.argoproj.io/hook-delete-policy: HookSucceeded
spec:
  template:
    spec:
      containers:
      - name: slack-notification
        image: curlimages/curl
        command: 
          - "curl"
          - "-X"
          - "POST"
          - "--data-urlencode"
          - "payload={\"channel\": \"#somechannel\", \"username\": \"hello\", \"text\": \"App Sync failed\", \"icon_emoji\": \":ghost:\"}"
          - "https://hooks.slack.com/services/..."
      restartPolicy: Never
  backoffLimit: 2
```
