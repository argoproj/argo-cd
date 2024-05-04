# Application Sync using impersonation

!!! warning "Alpha Feature"
    This is an experimental, alpha-quality feature that allows you to control the service account used for the sync operation. The configured service account, could have lesser privileges required for creating resources compared to the highly privileged access required for the control plane operations.

!!! warning
    Please read this documentation carefully before you enable this feature. Misconfiguration could lead to potential security issues.

## Introduction

As of version 2.10, Argo CD supports syncing `Application` resources using the same service account used for its control plane operations. This feature enables users to decouple service account used for application sync from the service account used for control plane operations.

Application syncs in Argo CD have the same privileges as the Argo CD control plane. As a consequence, in a multi-tenant setup, the Argo CD control plane privileges needs to match the tenant that needs the highest privileges. As an example, if an Argo CD instance has 10 Applications and only one of them requires admin privileges, then the Argo CD control plane must have admin privileges in order to be able to sync that one Application. Argo CD provides a multi-tenancy model to restrict what each Application can do using `AppProjects`, even though the control plane has higher privileges, however that creates a large attack surface since if Argo CD is compromised, attackers would have cluster-admin access to the cluster.

Some manual steps will need to be performed by the Argo CD administrator in order to enable this feature. 

!!! note
    This feature is considered beta as of now. Some of the implementation details may change over the course of time until it is promoted to a stable status. We will be happy if early adopters use this feature and provide us with bug reports and feedback.

### What is Impersonation

Impersonation is a feature in Kubernetes and enabled in the `kubectl` CLI client, using which, a user can act as another user through impersonation headers. For example, an admin could use this feature to debug an authorization policy by temporarily impersonating another user and seeing if a request was denied.

Impersonation requests first authenticate as the requesting user, then switch to the impersonated user info.

```shell
kubectl --as <user-to-impersonate> ...
kubectl --as <user-to-impersonate> --as-group <group-to-impersonate> ...
```
## Prerequisites
- In a multi team/multi tenant environment, an application team is typically granted access to a namespace to self-manage their Applications in a declarative way. 
- The tenant namespace and the service account to be used for creating the resources in that namespace is created.
- Create a Role to manage kubernetes resources in the tenant namespace
- Create a RoleBinding to map the service account to the role created in the previous step.

## Implementation details

### Overview

In order for an application to use a different service account for the application sync operation, the following steps needs to be performed:

1. The impersonation feature flag should be enabled by setting the value of key `application.sync.impersonation.enabled` to `true` in the `argocd-cm` ConfigMap as below:
```yaml
data:
  application.sync.impersonation.enabled: true
```

2. The `AppProject` referenced by the `.spec.project` field of the `Application` must have the `DestinationServiceAccounts` mapping the destination server and namespace to a service account to be used for the sync operation.

`DestinationServiceAccounts` associated to a `AppProject` can be created and managed, either declaratively or through the Argo CD API (e.g. using the CLI, the web UI, the REST API, etc).


### Enable application sync with impersonation feature

In order to enable this feature, the Argo CD administrator must reconfigure the `application.sync.impersonation.enabled` settings in the `argocd-cm` ConfigMap as below:

```yaml
data:
  application.sync.impersonation.enabled: true
```
  
## Configuring destination service accounts

### Declaratively

For declaratively configuring destination service accounts, in the `AppProject`, add a section `.spec.destinationServiceAccounts`. Specify the target destination `server` and `namespace` and the provide the service account to be used for the sync operation using `defaultServiceAccount` field. Applications that refer this `AppProject` would use the corresponding service account configured for its destination. If there are multiple matches, then the first valid match would be considered.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: my-project
  namespace: argocd
spec:
  description: Example Project
  # Allow manifests to deploy from any Git repos
  sourceRepos:
    - '*'
  destinations:
    - *
  destinationServiceAccounts:
    - server: https://kubernetes.default.svc
      namespace: guestbook
      defaultServiceAccount: guestbook-deployer
    - server: https://kubernetes.default.svc
      namespace: guestbook-dev
      defaultServiceAccount: guestbook-dev-deployer
    - server: https://kubernetes.default.svc
      namespace: guestbook-stage
      defaultServiceAccount: guestbook-stage-deployer
    - server: https://kubernetes.default.svc
      namespace: '*'
      defaultServiceAccount: default
```

### Using the CLI

You can use all existing Argo CD CLI commands for adding destination service account

For example, to add a destination service account for `in-cluster` and `guestbook` namespace, you can use the following CLI command:

```shell
argocd proj add-destination-service-account my-project https://kubernetes.default.svc guestbook guestbook-sa
```

Likewise, to remove the destination service account from an `AppProject`, you can use the following CLI command:

```shell
argocd proj remove-destination-service-account my-project https://kubernetes.default.svc guestbook
```

### Using the UI

Similar to the CLI, you can add destination service account when creating an `AppProject` from the UI
