# Application Sync using impersonation

!!! warning "Alpha Feature"
    This is an experimental, alpha-quality feature that allows you to control the service account used for the sync operation. The configured service account, could have lesser privileges required for creating resources compared to the highly privileged access required for the control plane operations.

!!! warning
    Please read this documentation carefully before you enable this feature. Misconfiguration could lead to potential security issues.

## Introduction

Argo CD supports syncing `Application` resources using the same service account used for its control plane operations. This feature enables users to decouple service account used for application sync from the service account used for control plane operations.

By default, application syncs in Argo CD have the same privileges as the Argo CD control plane. As a consequence, in a multi-tenant setup, the Argo CD control plane privileges needs to match the tenant that needs the highest privileges. As an example, if an Argo CD instance has 10 Applications and only one of them requires admin privileges, then the Argo CD control plane must have admin privileges in order to be able to sync that one Application. This provides an opportunity for malicious tenants to gain admin level access. Argo CD provides a multi-tenancy model to restrict what each `Application` is authorized to do using `AppProjects`, however it is not secure enough and if Argo CD is compromised, attackers will easily gain `cluster-admin` access to the cluster.

Some manual steps will need to be performed by the Argo CD administrator in order to enable this feature, as it is disabled by default.

!!! note
    This feature is considered alpha as of now. Some of the implementation details may change over the course of time until it is promoted to a stable status. We will be happy if early adopters use this feature and provide us with bug reports and feedback.

### What is Impersonation

Impersonation is a feature in Kubernetes and enabled in the `kubectl` CLI client, using which, a user can act as another user through impersonation headers. For example, an admin could use this feature to debug an authorization policy by temporarily impersonating another user and seeing if a request was denied.

Impersonation requests first authenticate as the requesting user, then switch to the impersonated user info.

## Prerequisites

In a multi-team/multi-tenant environment, a team/tenant is typically granted access to a target namespace to self-manage their kubernetes resources in a declarative way.
A typical tenant onboarding process looks like below:
1. The platform admin creates a tenant namespace and the service account to be used for creating the resources is also created in the same tenant namespace.
2. The platform admin creates one or more Role(s) to manage kubernetes resources in the tenant namespace
3. The platform admin creates one or more RoleBinding(s) to map the service account to the role(s) created in the previous steps.
4. The platform admin can choose to use either the [apps-in-any-namespace](./app-any-namespace.md) feature or provide access to tenants to create applications in the ArgoCD control plane namespace.
5. If the platform admin chooses apps-in-any-namespace feature, tenants can self-service their Argo applications in their respective tenant namespaces and no additional access needs to be provided for the control plane namespace.

## Implementation details

### Overview

In order for an application to use a different service account for the application sync operation, the following steps needs to be performed:

1. The impersonation feature flag should be enabled. Please refer the steps provided in [Enable application sync with impersonation feature](#enable-application-sync-with-impersonation-feature)

2. The `AppProject` referenced by the `.spec.project` field of the `Application` must have the `DestinationServiceAccounts` mapping the destination server and namespace to a service account to be used for the sync operation. Please refer the steps provided in [Configuring destination service accounts](#configuring-destination-service-accounts)


### Enable application sync with impersonation feature

In order to enable this feature, the Argo CD administrator must reconfigure the `application.sync.impersonation.enabled` settings in the `argocd-cm` ConfigMap as below:

```yaml
data:
  application.sync.impersonation.enabled: "true"
```

### Disable application sync with impersonation feature

In order to disable this feature, the Argo CD administrator must reconfigure the `application.sync.impersonation.enabled` settings in the `argocd-cm` ConfigMap as below:

```yaml
data:
  application.sync.impersonation.enabled: "false"
```

!!! note
    This feature is disabled by default.

!!! note
    This feature can be enabled/disabled only at the system level and once enabled/disabled it is applicable to all Applications managed by ArgoCD.

## Configuring destination service accounts

Destination service accounts can be added to the `AppProject` under `.spec.destinationServiceAccounts`. Specify the target destination `server` and `namespace` and provide the service account to be used for the sync operation using `defaultServiceAccount` field. Applications that refer this `AppProject` will use the corresponding service account configured for its destination.

During the application sync operation, the controller loops through the available `destinationServiceAccounts` in the mapped `AppProject` and tries to find a matching candidate. If there are multiple matches for a destination server and namespace combination, then the first valid match will be considered. If there are no matches, then an error is reported during the sync operation. In order to avoid such sync errors, it is highly recommended that a valid service account may be configured as a catch-all configuration, for all target destinations and kept in lowest order of priority.

It is possible to specify service accounts along with its namespace. eg: `tenant1-ns:guestbook-deployer`. If no namespace is provided for the service account, then the Application's `spec.destination.namespace` will be used. If no namespace is provided for the service account and the optional `spec.destination.namespace` field is also not provided in the `Application`, then the Application's namespace will be used.

`DestinationServiceAccounts` associated to a `AppProject` can be created and managed, either declaratively or through the Argo CD API (e.g. using the CLI, the web UI, the REST API, etc).

### Using declarative yaml

For declaratively configuring destination service accounts, create an yaml file for the `AppProject` as below and apply the changes using `kubectl apply` command.

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
    - server: https://kubernetes.default.svc # catch-all configuration
      namespace: '*'
      defaultServiceAccount: default
```

### Using the CLI

Destination service accounts can be added to an `AppProject` using the ArgoCD CLI.

For example, to add a destination service account for `in-cluster` and `guestbook` namespace, you can use the following CLI command:

```shell
argocd proj add-destination-service-account my-project https://kubernetes.default.svc guestbook guestbook-sa
```

Likewise, to remove the destination service account from an `AppProject`, you can use the following CLI command:

```shell
argocd proj remove-destination-service-account my-project https://kubernetes.default.svc guestbook
```

### Using the UI

Similar to the CLI, you can add destination service account when creating or updating an `AppProject` from the UI
