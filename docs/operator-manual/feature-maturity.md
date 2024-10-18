# Feature Maturity

Argo CD features may be marked with a certain [status](https://github.com/argoproj/argoproj/blob/main/community/feature-status.md)
to indicate their stability and maturity. These are the statuses of non-stable features in Argo CD:

TODO: add alpha admonition warning
if it is a spec, annotation or configmap config, they are all subject to breaking changes and removal

## Overview

| Feature                             | Introduced | Status |
| ----------------------------------- | ---------- | ------ |
| [Structured Merge Diff Strategy][1] | v2.5.0     | Beta   |
| [AppSet Progressive Syncs][2]       | v2.6.0     | Alpha  |
| [Proxy Extensions][3]               | v2.7.0     | Alpha  |
| [Skip Application Reconcile][4]     | v2.7.0     | Alpha  |
| [AppSets in any Namespace][5]       | v2.8.0     | Beta   |
| [Dynamic Cluster Distribution][6]   | v2.9.0     | Alpha  |
| [Server Side Diff][7]               | v2.10.0    | Beta   |
| [Service Account Impersonation][8]  | v2.13.0    | Alpha  |

[1]: ../user-guide/diff-strategies.md#structured-merge-diff
[2]: applicationset/Progressive-Syncs.md
[3]: ../developer-guide/extensions/proxy-extensions.md
[4]: ../user-guide/skip_reconcile.md
[5]: applicationset/Appset-Any-Namespace.md
[6]: dynamic-cluster-distribution.md
[7]: ../user-guide/diff-strategies.md#server-side-diff
[8]: app-sync-using-impersonation.md

## Unstable Configurations

### Application CRD

| Feature                             | Property                                                                                | Status |
| ----------------------------------- | --------------------------------------------------------------------------------------- | ------ |
| [Structured Merge Diff Strategy][1] | `metadata.annotations[argocd.argoproj.io/compare-options]: ServerSideDiff=true`         | Beta   |
| [Structured Merge Diff Strategy][1] | `metadata.annotations[argocd.argoproj.io/compare-options]: IncludeMutationWebhook=true` | Beta   |
| TODO                                | `TODO`                                                                                  | TODO   |

### AppProject CRD

| Feature | Property | Status |
| ------- | -------- | ------ |
| TODO    | `TODO`   | TODO   |
| TODO    | `TODO`   | TODO   |

### ApplicationSet CRD

| Feature                       | Property                   | Status |
| ----------------------------- | -------------------------- | ------ |
| [AppSet Progressive Syncs][2] | `spec.strategy`            | Alpha  |
| [AppSet Progressive Syncs][2] | `status.applicationStatus` | Alpha  |
| TODO                          | `TODO`                     | TODO   |

### Configuration

| Feature                             | Object                                        | Property / Variable                                         | Status |
| ----------------------------------- | --------------------------------------------- | ----------------------------------------------------------- | ------ |
| [Structured Merge Diff Strategy][1] | `ConfigMap/argocd-cmd-params-cm`              | `controller.diff.server.side`                               | Beta   |
| [Structured Merge Diff Strategy][1] | `StatefulSet/argocd-application-controller`   | `ARGOCD_APPLICATION_CONTROLLER_SERVER_SIDE_DIFF`            | Beta   |
| [AppSet Progressive Syncs][2]       | `ConfigMap/argocd-cmd-params-cm`              | `applicationsetcontroller.enable.progressive.syncs`         | Alpha  |
| [AppSet Progressive Syncs][2]       | `Deployment/argocd-applicationset-controller` | `ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS` | Alpha  |
