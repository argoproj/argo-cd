# Feature Maturity

Argo CD features may be marked with a certain [status](https://github.com/argoproj/argoproj/blob/main/community/feature-status.md)
to indicate their stability and maturity. These are the statuses of non-stable features in Argo CD:

!!! danger "Using Alpha/Beta features risks"

    Aplha and Beta features do not guarantee backward compatibility and are subject to breaking changes in the future releases.
    It is highly suggested for Argo users not to rely on these features in production environments, especially if you do not have
    control over the Argo CD upgrades.

    Furthermore, removal of Alpha features may modify your resources to an unpredictable state after Argo CD is upgraded.
    You should make sure to document which features are in use and review the [release notes](./upgrading/overview.md) before upgrading.

## Overview

| Feature                                   | Introduced | Status |
| ----------------------------------------- | ---------- | ------ |
| [AppSet Progressive Syncs][2]             | v2.6.0     | Alpha  |
| [Proxy Extensions][3]                     | v2.7.0     | Alpha  |
| [Skip Application Reconcile][4]           | v2.7.0     | Alpha  |
| [AppSets in any Namespace][5]             | v2.8.0     | Beta   |
| [Cluster Sharding: round-robin][6]        | v2.8.0     | Alpha  |
| [Dynamic Cluster Distribution][7]         | v2.9.0     | Alpha  |
| [Server Side Diff][8]                     | v2.10.0    | Beta   |
| [Cluster Sharding: consistent-hashing][9] | v2.12.0    | Alpha  |
| [Service Account Impersonation][10]       | v2.13.0    | Alpha  |

## Unstable Configurations

### Application CRD

| Feature                         | Property                                                                                | Status |
| ------------------------------- | --------------------------------------------------------------------------------------- | ------ |
| [Server Side Diff][8]           | `metadata.annotations[argocd.argoproj.io/compare-options]: ServerSideDiff=true`         | Beta   |
| [Server Side Diff][8]           | `metadata.annotations[argocd.argoproj.io/compare-options]: IncludeMutationWebhook=true` | Beta   |
| [Skip Application Reconcile][4] | `metadata.annotations[argocd.argoproj.io/skip-reconcile]`                               | Alpha  |

### AppProject CRD

| Feature                             | Property                            | Status |
| ----------------------------------- | ----------------------------------- | ------ |
| [Service Account Impersonation][10] | `spec.destinationServiceAccounts.*` | Alpha  |

### ApplicationSet CRD

| Feature                       | Property                     | Status |
| ----------------------------- | ---------------------------- | ------ |
| [AppSet Progressive Syncs][2] | `spec.strategy.*`            | Alpha  |
| [AppSet Progressive Syncs][2] | `status.applicationStatus.*` | Alpha  |

### Configuration

| Feature                                   | Resource                                      | Property / Variable                                         | Status |
| ----------------------------------------- | --------------------------------------------- | ----------------------------------------------------------- | ------ |
| [AppSets in any Namespace][5]             | `Deployment/argocd-applicationset-controller` | `ARGOCD_APPLICATIONSET_CONTROLLER_ALLOWED_SCM_PROVIDERS`    | Beta   |
| [AppSets in any Namespace][5]             | `ConfigMap/argocd-cmd-params-cm`              | `applicationsetcontroller.allowed.scm.providers`            | Beta   |
| [AppSets in any Namespace][5]             | `ConfigMap/argocd-cmd-params-cm`              | `applicationsetcontroller.enable.scm.providers`             | Beta   |
| [AppSets in any Namespace][5]             | `Deployment/argocd-applicationset-controller` | `ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_SCM_PROVIDERS`     | Beta   |
| [AppSets in any Namespace][5]             | `Deployment/argocd-applicationset-controller` | `ARGOCD_APPLICATIONSET_CONTROLLER_NAMESPACES`               | Beta   |
| [AppSets in any Namespace][5]             | `ConfigMap/argocd-cmd-params-cm`              | `applicationsetcontroller.namespaces`                       | Beta   |
| [Server Side Diff][8]                     | `ConfigMap/argocd-cmd-params-cm`              | `controller.diff.server.side`                               | Beta   |
| [Server Side Diff][8]                     | `StatefulSet/argocd-application-controller`   | `ARGOCD_APPLICATION_CONTROLLER_SERVER_SIDE_DIFF`            | Beta   |
| [AppSet Progressive Syncs][2]             | `ConfigMap/argocd-cmd-params-cm`              | `applicationsetcontroller.enable.progressive.syncs`         | Alpha  |
| [AppSet Progressive Syncs][2]             | `Deployment/argocd-applicationset-controller` | `ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS` | Alpha  |
| [Proxy Extensions][3]                     | `ConfigMap/argocd-cmd-params-cm`              | `server.enable.proxy.extension`                             | Alpha  |
| [Proxy Extensions][3]                     | `Deployment/argocd-server`                    | `ARGOCD_SERVER_ENABLE_PROXY_EXTENSION`                      | Alpha  |
| [Proxy Extensions][3]                     | `ConfigMap/argocd-cm`                         | `extension.config`                                          | Alpha  |
| [Dynamic Cluster Distribution][7]         | `Deployment/argocd-application-controller`    | `ARGOCD_ENABLE_DYNAMIC_CLUSTER_DISTRIBUTION`                | Alpha  |
| [Dynamic Cluster Distribution][7]         | `Deployment/argocd-application-controller`    | `ARGOCD_CONTROLLER_HEARTBEAT_TIME`                          | Alpha  |
| [Cluster Sharding: round-robin][6]        | `ConfigMap/argocd-cmd-params-cm`              | `controller.sharding.algorithm: round-robin`                | Alpha  |
| [Cluster Sharding: round-robin][6]        | `StatefulSet/argocd-application-controller`   | `ARGOCD_CONTROLLER_SHARDING_ALGORITHM=round-robin`          | Alpha  |
| [Cluster Sharding: consistent-hashing][9] | `ConfigMap/argocd-cmd-params-cm`              | `controller.sharding.algorithm: consistent-hashing`         | Alpha  |
| [Cluster Sharding: consistent-hashing][9] | `StatefulSet/argocd-application-controller`   | `ARGOCD_CONTROLLER_SHARDING_ALGORITHM=consistent-hashing`   | Alpha  |
| [Service Account Impersonation][10]       | `ConfigMap/argocd-cm`                         | `application.sync.impersonation.enabled`                    | Alpha  |

[2]: applicationset/Progressive-Syncs.md
[3]: ../developer-guide/extensions/proxy-extensions.md
[4]: ../user-guide/skip_reconcile.md
[5]: applicationset/Appset-Any-Namespace.md
[6]: ./high_availability.md#argocd-application-controller
[7]: dynamic-cluster-distribution.md
[8]: ../user-guide/diff-strategies.md#server-side-diff
[9]: ./high_availability.md#argocd-application-controller
[10]: app-sync-using-impersonation.md
