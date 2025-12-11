# Progressive Syncs

> [!WARNING]
> **Beta Feature (Since v2.6.0)**
>

    This feature is in the [Beta](https://github.com/argoproj/argoproj/blob/main/community/feature-status.md#beta) stage. It is generally considered stable, but there may be unhandled edge cases.
    This feature allows you to control the order in which the ApplicationSet controller will create or update the Applications
    owned by an ApplicationSet resource. 

## Use Cases

The Progressive Syncs feature set is intended to be light and flexible. The feature only interacts with the health of managed Applications. It is not intended to support direct integrations with other Rollout controllers (such as the native ReplicaSet controller or Argo Rollouts).

- Progressive Syncs watch for the managed Application resources to become "Healthy" before proceeding to the next stage.
- Deployments, DaemonSets, StatefulSets, and [Argo Rollouts](https://argoproj.github.io/argo-rollouts/) are all supported, because the Application enters a "Progressing" state while pods are being rolled out. In fact, any resource with a health check that can report a "Progressing" status is supported.
- [Argo CD Resource Hooks](../../user-guide/resource_hooks.md) are supported. We recommend this approach for users that need advanced functionality when an Argo Rollout cannot be used, such as smoke testing after a DaemonSet change.

## Enabling Progressive Syncs

As an experimental feature, progressive syncs must be explicitly enabled, in one of these ways.

1. Pass `--enable-progressive-syncs` to the ApplicationSet controller args.
1. Set `ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS=true` in the ApplicationSet controller environment variables.
1. Set `applicationsetcontroller.enable.progressive.syncs: "true"` in the Argo CD `argocd-cmd-params-cm` ConfigMap.

## Strategies

ApplicationSet strategies control both how applications are created (or updated) and deleted. These operations are configured using two separate fields:

- **Creation Strategy** (`type` field): Controls application creation and updates
- **Deletion Strategy** (`deletionOrder` field): Controls application deletion order

### Creation Strategies

The `type` field controls how applications are created and updated. Available values:

- **AllAtOnce** (default)
- **RollingSync**

#### AllAtOnce

This default Application update behavior is unchanged from the original ApplicationSet implementation.

All Applications managed by the ApplicationSet resource are updated simultaneously when the ApplicationSet is updated.

```yaml
spec:
  strategy:
    type: AllAtOnce # explicit, but this is the default
```

#### RollingSync

This update strategy allows you to group Applications by labels present on the generated Application resources.
When the ApplicationSet changes, the changes will be applied to each group of Application resources sequentially.

- Application groups are selected using their labels and `matchExpressions`.
- All `matchExpressions` must be true for an Application to be selected (multiple expressions match with AND behavior).
- The `In` and `NotIn` operators must match at least one value to be considered true (OR behavior).
- The `NotIn` operator has priority in the event that both a `NotIn` and `In` operator produce a match.
- All Applications in each group must become Healthy before the ApplicationSet controller will proceed to update the next group of Applications.
- The number of simultaneous Application updates in a group will not exceed its `maxUpdate` parameter (default is 100%, unbounded).
- RollingSync will capture external changes outside the ApplicationSet resource, since it relies on watching the OutOfSync status of the managed Applications.
- RollingSync will force all generated Applications to have autosync disabled. Warnings are printed in the applicationset-controller logs for any Application specs with an automated syncPolicy enabled.
- Sync operations are triggered the same way as if they were triggered by the UI or CLI (by directly setting the `operation` status field on the Application resource). This means that a RollingSync will respect sync windows just as if a user had clicked the "Sync" button in the Argo UI.
- When a sync is triggered, the sync is performed with the same syncPolicy configured for the Application. For example, this preserves the Application's retry settings.
- If an Application is not selected in any step, it will be excluded from the rolling sync and needs to be manually synced through the CLI or UI.

```yaml
spec:
  strategy:
    type: RollingSync
    rollingSync:
      steps:
        - matchExpressions:
            - key: envLabel
              operator: In
              values:
                - env-dev
        - matchExpressions:
            - key: envLabel
              operator: In
              values:
                - env-prod
          maxUpdate: 10%
```

In the above example, the sync will be performed in two steps:

1. All Applications with the label `envLabel=env-dev` will be selected to sync first. Since `maxUpdate` is not defined, a default of 100% applies and all matched Applications will be synced simultaneously. The controller waits until every selected Application reaches a `Healthy` status
before proceeding to the next step.

2. Next, Applications with the label `envLabel=env-prod` will be selected to sync. Here, only 10% of the matched Applications will be synced at a time.
Once each batch of Applications reaches a `Healthy` status, the next batch is synced until all matched

If there are any applications that don't match the listed expressions, they will not be synced by the RollingSync strategy and must be manually synced as describe above.

### Deletion Strategies

The `deletionOrder` field controls the order in which applications are deleted when they are removed from the ApplicationSet. Available values:

- **AllAtOnce** (default)
- **Reverse**

#### AllAtOnce Deletion

This is the default behavior where all applications that need to be deleted are removed simultaneously. This works with both `AllAtOnce` and `RollingSync` creation strategies.

```yaml
spec:
  strategy:
    type: RollingSync # or AllAtOnce
    deletionOrder: AllAtOnce # explicit, but this is the default
```

#### Reverse Deletion

When using `deletionOrder: Reverse` with RollingSync strategy, applications are deleted in reverse order of the steps defined in `rollingSync.steps`. This ensures that applications deployed in later steps are deleted before applications deployed in earlier steps.
This strategy is particularly useful when you need to tear down dependent services in the particular sequence.

**Requirements for Reverse deletion:**

- Must be used with `type: RollingSync`
- Requires `rollingSync.steps` to be defined
- Applications are deleted in reverse order of step sequence

**Important:** The ApplicationSet finalizer is not removed until all applications are successfully deleted. This ensures proper cleanup and prevents the ApplicationSet from being removed before its managed applications. 

**Note:** ApplicationSet controller ensures there is a finalizer when `deletionOrder` is set as `Reverse` with progressive sync enabled. This means that if the applicationset is missing the required finalizer, the applicationset controller adds the finalizer to ApplicationSet before generating applications.

```yaml
spec:
  strategy:
    type: RollingSync
    deletionOrder: Reverse
    rollingSync:
      steps:
        - matchExpressions:
            - key: envLabel
              operator: In
              values:
                - env-dev # Step 1: Created first, deleted last
        - matchExpressions:
            - key: envLabel
              operator: In
              values:
                - env-prod # Step 2: Created second, deleted first
```

In this example, when applications are deleted:

1. `env-prod` applications (Step 2) are deleted first
2. `env-dev` applications (Step 1) are deleted second

This deletion order is useful for scenarios where you need to tear down dependent services in the correct sequence, such as deleting frontend services before backend dependencies.

#### Example

The following example illustrates how to stage a progressive sync over Applications with explicitly configured environment labels.

Once a change is pushed, the following will happen in order.

- All `env-dev` Applications will be updated simultaneously.
- The rollout will wait for all `env-qa` Applications to be manually synced via the `argocd` CLI or by clicking the Sync button in the UI.
- 10% of all `env-prod` Applications will be updated at a time until all `env-prod` Applications have been updated.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
spec:
  generators:
    - list:
        elements:
          - cluster: engineering-dev
            url: https://1.2.3.4
            env: env-dev
          - cluster: engineering-qa
            url: https://2.4.6.8
            env: env-qa
          - cluster: engineering-prod
            url: https://9.8.7.6/
            env: env-prod
  strategy:
    type: RollingSync
    deletionOrder: Reverse # Applications will be deleted in reverse order of steps
    rollingSync:
      steps:
        - matchExpressions:
            - key: envLabel
              operator: In
              values:
                - env-dev
          #maxUpdate: 100%  # if undefined, all applications matched are updated together (default is 100%)
        - matchExpressions:
            - key: envLabel
              operator: In
              values:
                - env-qa
          maxUpdate: 0 # if 0, no matched applications will be updated
        - matchExpressions:
            - key: envLabel
              operator: In
              values:
                - env-prod
          maxUpdate: 10% # maxUpdate supports both integer and percentage string values (rounds down, but floored at 1 Application for >0%)
  goTemplate: true
  goTemplateOptions: ['missingkey=error']
  template:
    metadata:
      name: '{{.cluster}}-guestbook'
      labels:
        envLabel: '{{.env}}'
    spec:
      project: my-project
      source:
        repoURL: https://github.com/infra-team/cluster-deployments.git
        targetRevision: HEAD
        path: guestbook/{{.cluster}}
      destination:
        server: '{{.url}}'
        namespace: guestbook
```
