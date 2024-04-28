# Progressive Syncs

!!! warning "Alpha Feature"
    This is an experimental, alpha-quality feature that allows you to control the order in which the ApplicationSet controller will create or update the Applications owned by an ApplicationSet resource. It may be removed in future releases or modified in backwards-incompatible ways.

## Use Cases
The Progressive Syncs feature set is intended to be light and flexible. The feature only interacts with the health of managed Applications. It is not intended to support direct integrations with other Rollout controllers (such as the native ReplicaSet controller or Argo Rollouts).

* Progressive Syncs watch for the managed Application resources to become "Healthy" before proceeding to the next stage.
* Deployments, DaemonSets, StatefulSets, and [Argo Rollouts](https://argoproj.github.io/argo-rollouts/) are all supported, because the Application enters a "Progressing" state while pods are being rolled out. In fact, any resource with a health check that can report a "Progressing" status is supported.
* [Argo CD Resource Hooks](../../user-guide/resource_hooks.md) are supported. We recommend this approach for users that need advanced functionality when an Argo Rollout cannot be used, such as smoke testing after a DaemonSet change.

## Enabling Progressive Syncs
As an experimental feature, progressive syncs must be explicitly enabled, in one of these ways.

1. Pass `--enable-progressive-syncs` to the ApplicationSet controller args.
1. Set `ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS=true` in the ApplicationSet controller environment variables.
1. Set `applicationsetcontroller.enable.progressive.syncs: true` in the Argo CD `argocd-cmd-params-cm` ConfigMap.

## Strategies

* AllAtOnce (default)
* RollingSync

### AllAtOnce
This default Application update behavior is unchanged from the original ApplicationSet implementation.

All Applications managed by the ApplicationSet resource are updated simultaneously when the ApplicationSet is updated.

### RollingSync
This update strategy allows you to group Applications by labels present on the generated Application resources.
When the ApplicationSet changes, the changes will be applied to each group of Application resources sequentially.

* Application groups are selected using their labels and `matchExpressions`.
* All `matchExpressions` must be true for an Application to be selected (multiple expressions match with AND behavior).
* The `In` and `NotIn` operators must match at least one value to be considered true (OR behavior).
* The `NotIn` operator has priority in the event that both a `NotIn` and `In` operator produce a match.
* All Applications in each group must become Healthy before the ApplicationSet controller will proceed to update the next group of Applications.
* The number of simultaneous Application updates in a group will not exceed its `maxUpdate` parameter (default is 100%, unbounded).
* RollingSync will capture external changes outside the ApplicationSet resource, since it relies on watching the OutOfSync status of the managed Applications.
* RollingSync will force all generated Applications to have autosync disabled. Warnings are printed in the applicationset-controller logs for any Application specs with an automated syncPolicy enabled.
* Sync operations are triggered the same way as if they were triggered by the UI or CLI (by directly setting the `operation` status field on the Application resource). This means that a RollingSync will respect sync windows just as if a user had clicked the "Sync" button in the Argo UI.
* When a sync is triggered, the sync is performed with the same syncPolicy configured for the Application. For example, this preserves the Application's retry settings.
* If an Application is considered "Pending" for `applicationsetcontroller.default.application.progressing.timeout` seconds, the Application is automatically moved to Healthy status (default 300).

#### Example
The following example illustrates how to stage a progressive sync over Applications with explicitly configured environment labels.

Once a change is pushed, the following will happen in order.

* All `env-dev` Applications will be updated simultaneously.
* The rollout will wait for all `env-qa` Applications to be manually synced via the `argocd` CLI or by clicking the Sync button in the UI.
* 10% of all `env-prod` Applications will be updated at a time until all `env-prod` Applications have been updated.

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
          maxUpdate: 0      # if 0, no matched applications will be updated
        - matchExpressions:
            - key: envLabel
              operator: In
              values:
                - env-prod
          maxUpdate: 10%    # maxUpdate supports both integer and percentage string values (rounds down, but floored at 1 Application for >0%)
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
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
