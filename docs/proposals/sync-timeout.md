---
title: Neat-enhancement-idea
authors:
  - "@alexmt"
sponsors:
  - "@jessesuen"
reviewers:
  - "@ishitasequeira"
approvers:
  - "@gdsoumya"

creation-date: 2023-12-16
last-updated: 2023-12-16
---

# Sync Operation Timeout & Termination Settings

The Sync Operation Timeout & Termination Settings feature introduces new sync operation settings that control automatic sync operation termination.

## Summary


The feature includes two types of settings:

* The sync timeout allows users to set a timeout for the sync operation. If the sync operation exceeds this timeout, it will be terminated.

* The Termination settings are an advanced set of options that enable terminating the sync operation earlier when a known resource is stuck in a
certain state for a specified amount of time.

## Motivation

Complex synchronization operations that involve sync hooks and sync waves can be time-consuming and may occasionally become stuck in a specific state
for an extended duration. In certain instances, these operations might indefinitely remain in this state. This situation becomes particularly inconvenient when the
synchronization is initiated by an automation tool like a CI/CD pipeline. In these scenarios, the automation tool may end up waiting indefinitely for the
synchronization process to complete.

To address this issue, this feature enables users to establish a timeout for the sync operation. If the operation exceeds the specified time limit,
it will be terminated, preventing extended periods of inactivity or indefinite waiting in automated processes.

### Goals

The following goals are intended to be met by this enhancement:

#### [G-1] Synchronization timeout

The synchronization timeout feature should allow users to set a timeout for the sync operation. If the sync operation exceeds this timeout, it will be terminated.

#### [G-2] Termination settings

The termination settings would allow users to terminate the sync operation earlier when a known resource is stuck in a certain state for a specified amount of time.

## Proposal

The proposed additional synchronization settings are to be added to the `syncPolicy.terminate` field within the Application CRD. The following features are to be added:

* `timeout` - The timeout for the sync operation. If the sync operation exceeds this timeout, it will be terminated.
* `resources` - A list of resources to monitor for termination. If any of the resources in the list are stuck in a
  certain state for a specified amount of time, the sync operation will be terminated.

Example:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
spec:
  ... # standard application spec

  syncPolicy:
    terminate:
      timeout: 10m # timeout for the sync operation
      resources:
        - kind: Deployment
          name: guestbook-ui
          timeout: 5m # timeout for the resource
          health: Progressing # health status of the resource
```

### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

#### Normal sync operation:
As a user, I would like to trigger a sync operation and expect it to complete within a certain time limit.

#### CI triggered sync operation:
As a user, I would like to trigger a sync operation from a CI/CD pipeline and expect it to complete within a certain time limit.

#### Preview Applications:
As a user, I would like to leverage ApplicationSet PR generator to generate preview applications and expect the auto sync operation fails automatically
if it exceeds a certain time limit.

### Implementation Details/Notes/Constraints [optional]

The application CRD status field already has all required information to implement sync timeout.

* Global sync timeout: only the operation start time is required to implement this functoinality. It is provided be the `status.operationState.startedAt` field.
* Resources state based termination. This part is a bit more complex and requires information about resources affected/created during the sync operation. Most of
the required information is already available in the Application CRD status field. The `status.operationState.syncResult.resources` field contains a list of resources
affected/created during the sync operation. Each `resource` list item includes the resource name, kind, and the resource health status. In order to provide accurate
duration of the resource health status it is proposed to add `modifiedAt` field to the `resource` list item. This field will be updated every time the resource health/phase
changes.

### Security Considerations

Proposed changes don't expand the scope of the application CRD and don't introduce any new security concerns.

### Risks and Mitigations

The execution of a synchronization operation is carried out in phases, which involve a series of Kubernetes API calls and typically take up to a few seconds.
There is no easy way to terminate the operation during the phase. So the operation might take few seconds longer than the specified timeout. It does not seems
reasonable to implement a more complex logic to terminate the operation during the phase. So it is proposed to just document that the operation might be terminated
few seconds after the timeout is reached.

### Upgrade / Downgrade Strategy

The proposed changes don't require any special upgrade/downgrade strategy. The new settings are optional and can be used by users only if they need them.

## Drawbacks

Slight increase of the application syncrhonization logic complexity.

## Alternatives

Rely on the external tools to terminate the sync operation. For example, the CI/CD pipeline can terminate the sync operation if it exceeds a certain time limit.