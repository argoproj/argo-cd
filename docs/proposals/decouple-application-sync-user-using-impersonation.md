---
title: Decouple Control plane and Application Sync privileges
authors:
  - "@anandf"
sponsors:
  - Red Hat
reviewers:
  - "@blakepettersson"
  - "@crenshaw-dev"
  - "@jannfis"
approvers:
  - "@alexmt"
  - "@crenshaw-dev"
  - "@jannfis"

creation-date: 2023-06-23
last-updated: 2024-02-06
---

# Decouple Application Sync using Impersonation

Application syncs in Argo CD have the same privileges as the Argo CD control plane. As a consequence, in a multi-tenant setup, the Argo CD control plane privileges needs to match the tenant that needs the highest privileges. As an example, if an Argo CD instance has 10 Applications and only one of them requires admin privileges, then the Argo CD control plane must have admin privileges in order to be able to sync that one Application. Argo CD provides a multi-tenancy model to restrict what each Application can do using `AppProjects`, even though the control plane has higher privileges, however that creates a large attack surface since if Argo CD is compromised, attackers would have cluster-admin access to the cluster.

The goal of this proposal is to perform the Application sync as a different user using impersonation and use the service account provided in the cluster config purely for control plane operations.

### What is Impersonation

Impersonation is a feature in Kubernetes and enabled in the `kubectl` CLI client, using which, a user can act as another user through impersonation headers. For example, an admin could use this feature to debug an authorization policy by temporarily impersonating another user and seeing if a request was denied.

Impersonation requests first authenticate as the requesting user, then switch to the impersonated user info.

```
kubectl --as <user-to-impersonate> ...
kubectl --as <user-to-impersonate> --as-group <group-to-impersonate> ...
```

## Open Questions [optional]

- Should the restrictions imposed as part of the `AppProjects` be honored if the impersonation feature is enabled ?
>Yes, other restrictions implemented by `AppProject` related to whitelisting/blacklisting resources must continue to be honoured.
- Can an Application refer to a service account with elevated privileges like say  `cluster-admin`, `admin`, and service accounts used for running the ArgoCD controllers itself ?
>Yes, this is possible as long as the ArgoCD admin user explicitly allows it through the `AppProject` configuration.
- Among the destinations configured in the `AppProject`, if there are multiple matches for a given destination, which destination option should be used ?
>If there are more than one matching destination, either with a glob pattern match or an exact match, then we use the first valid match to determine the service account to be used for the sync operation.
- Can the kubernetes audit trail events capture the impersonation.
>Yes, kubernetes audit trail events capture both the actual user and the impersonating user details and hence its possible to track who executed the commands and as which user permissions using the audit trails.
- Would the Sync hooks be using the impersonation service account.
>Yes, if the impersonation feature is enabled and customers use Sync hooks, then impersonation service account would be used for executing the hook jobs as well.
- If application resources have hardcoded namespaces in the git repository, would different service accounts be used for each resource during the sync operation ?
>The service account to be used for impersonation is determined on a per Application level rather than on per resource level. The value specified in `Application.spec.destination.namespace` would be used to determine the service account to be used for the sync operation of all resources present in the `Application`.

## Summary

In a multi team/multi tenant environment, an application team is typically granted access to a namespace to self-manage their Applications in a declarative way. Current implementation of ArgoCD requires the ArgoCD Administrator to create an `AppProject` with access settings configured to replicate the RBAC resources that are configured for each team. This approach requires duplication of effort and also requires syncing the access between both to maintain the security posture. It would be desirable for users to use the existing RBAC rules without having to revert to Argo CD API to create and manage these Applications. One namespace per team, or even one namespace per application is what we are looking to address as part of this proposal.

## Motivation

This proposal would allow ArgoCD administrators to manage the cluster permissions using kubernetes native RBAC implementation rather than using complex configurations in `AppProjects` to restrict access to individual applications. By decoupling the privileges required for application sync from the privileges required for ArgoCD control plane operations, the security requirement of providing least privileges can be achieved there by improving the security posture of ArgoCD. For implementing multi team/tenant use cases, this decoupling would be greatly beneficial.

### Assumptions

- Namespaces are pre-populated with one or more `ServiceAccounts` that define the permissions for each `AppProject`.
- Many users prefer to control access to kubernetes resources through kubernetes RBAC constructs instead of Argo specific constructs.
- Each tenant is generally given access to a specific namespace along with a service account, role or cluster role and role binding to control access to that namespace. 
- `Applications` created by a tenant manage namespaced resources.
- An `AppProject` can either be mapped to a single tenant or multiple related tenants and the respective destinations that needs to be managed via the `AppProject`, needs to be configured.


### Goals
- Applications may only impersonate ServiceAccounts that live in the same namespace as the destination namespace configured in the application.If the service account is created in a different namespace, then the user can provide the service account name in the format `<namespace>:<service_account_name>` . ServiceAccount to be used for syncing each application is determined by the target destination configured in the `AppProject` associated with the `Application`.
- If impersonation feature is enabled, and no service account name is provided in the associated `AppProject`, then the default service account of the destination namespace of the `Application` should be used.
- Access restrictions implemented through properties in AppProject (if done) must have the existing behavior. From a security standpoint, any restrictions that were available before switching to a service account based approach should continue to exist even when the impersonation feature is enabled.

### Non-Goals

None

## Proposal

As part of this proposal, it would be possible for an ArgoCD Admin to specify a service account name in `AppProjects` CR for a single or a group of destinations. A destination is uniquely identified by a target cluster and a namespace combined.

When applications gets synced, based on its destination (target cluster and namespace combination), the `defaultServiceAccount` configured in the `AppProject` will be selected and used for impersonation when executing the kubectl commands for the sync operation.

We would be introducing a new element `destinationServiceAccounts` in `AppProject.spec`. This element is used for the sole purpose of specifying the impersonation configuration. The `defaultServiceAccount` configured for the `AppProject` would be used for the sync operation for a particular destination cluster and namespace. If impersonation feature is enabled and no specific service account is provided in the `AppProject` CR, then the `default` service account in the destination namespace would be used for impersonation.

```
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: my-project
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
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
```

### Structure of DestinationServiceAccount:
|Parameter| Type | Required/Optional| Description|
| ------ | ------ | ------- | -------- |
| server | string | Required | Server specifies the URL of the target cluster's Kubernetes control plane API. Glob patterns are supported. |
| namespace | string | Required | Namespace specifies the target namespace for the application's resources. Glob patterns are supported. |
| defaultServiceAccount | string | Required| DefaultServiceAccount specifies the service account to be impersonated when performing the `Application` sync operation.|

**Note:** Only server URL for the target cluster is supported and target cluster name is not supported.

### Future enhancements

In a future release, we plan to support overriding of service accounts at the application level. In that case, we would be adding an element called `allowedServiceAccounts` to `AppProject.spec.destinationServiceAccounts[*]`

### Use cases

#### Use case 1:

As a user, I would like to use kubernetes security constructs to restrict user access for application sync
So that, I can provide granular permissions based on the principle of least privilege required for syncing an application.

#### Use case 2:

As a user, I would like to configure a common service account for all applications associated to an AppProject
So that, I can use a generic convention of naming service accounts and avoid associating the service account per application.

### Design considerations

- Extending the `destinations` field under `AppProjects` was an option that was considered. But since the intent of it was to restrict the destinations that an associated `Application` can use, it was not used. Also the destination fields allowed negation operator (`!`) which would complicate the service account matching logic. The decision to create a new struct under `AppProject.Spec` for specifying the service account for each destination was considered a better alternative.

- The field name `defaultServiceAccount` was chosen instead of `serviceAccount` as we wanted to support overriding of the service account at an `Application` at a later point in time and wanted to reserve the name `serviceAccount` for future extension.

- Not supporting all impersonation options at the moment to keep the initial design to a minimum. Based on the need and feedback, support to impersonate users or groups can be added in future.

### Implementation Details/Notes/Constraints

#### Component : GitOps Engine

- Fix GitOps Engine code to honor Impersonate configuration set in the Application sync context for all kubectl commands that are being executed.

#### Component: ArgoCD API

- Create a new struct type `DestinationServiceAccount` having fields `namespace`, `server` and `defaultServiceAccount`
- Create a new field `DestinationServiceAccounts` under a `AppProject.Spec` that takes in a list of `DestinationServiceAccount` objects.
- Add Documentation for newly introduced struct and its fields for `DestinationServiceAccount` and `DestinationServiceAccounts` under `AppProject.Spec`

#### Component: ArgoCD Application Controller

- Provide a configuration in `argocd-cm`  which can be modified to enable the Impersonation feature. Set `applicationcontroller.enable.impersonation: true` in the Argo CD ConfigMap. Default value of `applicationcontroller.enable.impersonation` would be `false` and user has to explicitly override it to use this feature.
- Provide an option to override the Impersonation feature using environment variables.
Set `ARGOCD_APPLICATION_CONTROLLER_ENABLE_IMPERSONATION=true` in the Application controller environment variables. Default value of the environment variable must be `false` and user has to explicitly set it to `true` to use this feature.
- Provide an option to enable this feature using a command line flag `--enable-impersonation`. This new argument option needs to be added to the Application controller args.
- Fix Application Controller `sync.go` to set the Impersonate configuration from the AppProject CR to the `SyncContext` Object (rawConfig and restConfig field, need to understand which config is used for the actual sync and if both configs need to be impersonated.)

#### Component: ArgoCD UI

- Provide option to create `DestinationServiceAccount` with fields `namespace`, `server` and `defaultServiceAccount`.
- Provide option to add multiple `DestinationServiceAccounts` to an `AppProject` created/updated via the web console.
- Update the User Guide documentation on how to use these newly added fields from the web console.

#### Component: ArgoCD CLI

- Provide option to create `DestinationServiceAccount` with fields `namespace`, `server` and `defaultServiceAccount`.
- Provide option to add multiple `DestinationServiceAccounts` to an `AppProject` created/updated via the web console.
- Update the User Guide and other documentation where the CLI option usages are explained.

#### Component: Documentation

- Add note that this is a Beta feature in the documentation.
- Add a separate section for this feature under user-guide section.
- Update the ArgoCD  CLI command reference documentation.
- Update the ArgoCD  UI command reference documentation.

### Detailed examples

#### Example 1: Service account for application sync specified at the AppProject level for all namespaces

In this specific scenario, service account name `generic-deployer` will get used for the application sync as the namespace `guestbook` matches the glob pattern `*`.

- Install ArgoCD in the `argocd` namespace.
```
kubectl apply -f https://raw.githubusercontent.com/argoproj/argo-cd/master/manifests/install.yaml -n argocd
```

- Enable the impersonation feature in ArgoCD.
```
kubectl set env statefulset/argocd-application-controller ARGOCD_APPLICATION_CONTROLLER_ENABLE_IMPERSONATION=true
```

- Create a namespace called `guestbook` and a service account called `guestbook-deployer`.
```
kubectl create namespace guestbook
kubectl create serviceaccount guestbook-deployer
```

- Create Role and RoleBindings and configure RBAC access for creating `Service` and `Deployment` objects in namespace `guestbook` for service account `guestbook-deployer`.
```
kubectl create role guestbook-deployer-role --verb get,list,update,delete --resource pods,deployment,service
kubectl create rolebinding guestbook-deployer-rb --serviceaccount guestbook-deployer --role guestbook-deployer-role
```

- Create the `Application` in the `argocd` namespace and the required `AppProject` as below 
```
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
spec:
  project: my-project
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    path: guestbook
  destination:
    server: https://kubernetes.default.svc
    namespace: guestbook
---
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: my-project
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  description: Example Project
  # Allow manifests to deploy from any Git repos
  sourceRepos:
    - '*'
  destinations:
    - namespace: *
      server: https://kubernetes.default.svc
  destinationServiceAccounts:
    - namespace: *
      server: https://kubernetes.default.svc 
      defaultServiceAccount: generic-deployer
```

#### Example 2: Service account for application sync specified at the AppProject level for specific namespaces

In this specific scenario, service account name `guestbook-deployer` will get used for the application sync as the namespace `guestbook` matches the target namespace `guestbook`.

- Install ArgoCD in the `argocd` namespace.
```
kubectl apply -f https://raw.githubusercontent.com/argoproj/argo-cd/master/manifests/install.yaml -n argocd
```

- Enable the impersonation feature in ArgoCD.
```
kubectl set env statefulset/argocd-application-controller ARGOCD_APPLICATION_CONTROLLER_ENABLE_IMPERSONATION=true
```

- Create a namespace called `guestbook` and a service account called `guestbook-deployer`.
```
kubectl create namespace guestbook
kubectl create serviceaccount guestbook-deployer
```
- Create Role and RoleBindings and configure RBAC access for creating `Service` and `Deployment` objects in namespace `guestbook` for service account `guestbook-deployer`.
```
kubectl create role guestbook-deployer-role --verb get,list,update,delete --resource pods,deployment,service
kubectl create rolebinding guestbook-deployer-rb --serviceaccount guestbook-deployer --role guestbook-deployer-role
```

In this specific scenario, service account name `guestbook-deployer` will get used as it matches to the specific namespace `guestbook`.
```
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
spec:
  project: my-project
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    path: guestbook
  destination:
    server: https://kubernetes.default.svc
    namespace: guestbook
---
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: my-project
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  description: Example Project
  # Allow manifests to deploy from any Git repos
  sourceRepos:
    - '*'
  destinations:
    - namespace: guestbook
      server: https://kubernetes.default.svc
    - namespace: guestbook-ui
      server: https://kubernetes.default.svc
  destinationServiceAccounts:
    - namespace: guestbook
      server: https://kubernetes.default.svc
      defaultServiceAccount: guestbook-deployer
    - namespace: guestbook-ui
      server: https://kubernetes.default.svc
      defaultServiceAccount: guestbook-ui-deployer
```

#### Example 3: Remote destination with cluster-admin access and using different service account for the sync operation

**Note**: In this example, we are relying on the default service account `argocd-manager` with `cluster-admin` privileges which gets created when adding a remote cluster destination using the ArgoCD CLI.

- Install ArgoCD in the `argocd` namespace.
```
kubectl apply -f https://raw.githubusercontent.com/argoproj/argo-cd/master/manifests/install.yaml -n argocd
```

- Enable the impersonation feature in ArgoCD.
```
kubectl set env statefulset/argocd-application-controller ARGOCD_APPLICATION_CONTROLLER_ENABLE_IMPERSONATION=true
```

- Add the remote cluster as a destination to argocd
```
argocd cluster add remote-cluster --name remote-cluster
```
**Note:** The above command would create a service account named `argocd-manager` in `kube-system` namespace and `ClusterRole` named `argocd-manager-role` with full cluster admin access and a `ClusterRoleBinding` named `argocd-manager-role-binding` mapping the `argocd-manager-role` to the service account `remote-cluster`

- In the remote cluster, create a namespace called `guestbook` and a service account called `guestbook-deployer`.
```
kubectl ctx remote-cluster
kubectl create namespace guestbook
kubectl create serviceaccount guestbook-deployer
```

- In the remote cluster, create `Role` and `RoleBindings` and configure RBAC access for creating `Service` and `Deployment` objects in namespace `guestbook` for service account `guestbook-deployer`.

```
kubectl ctx remote-cluster
kubectl create role guestbook-deployer-role --verb get,list,update,delete --resource pods,deployment,service
kubectl create rolebinding guestbook-deployer-rb --serviceaccount guestbook-deployer --role guestbook-deployer-role
```

- Create the `Application` and `AppProject` for the `guestbook` application.
```
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
spec:
  project: my-project
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    path: guestbook
  destination:
    server: https://kubernetes.default.svc
    namespace: guestbook
---
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: my-project
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  description: Example Project
  # Allow manifests to deploy from any Git repos
  sourceRepos:
    - '*'
  destinations:
    - namespace: guestbook
      server: https://kubernetes.default.svc
      serviceAccountName: guestbook-deployer
  destinationServiceAccounts:
    - namespace: guestbook
      server: https://kubernetes.default.svc
      defaultServiceAccount: guestbook-deployer
```

#### Example 4: Remote destination with a custom service account for the sync operation

**Note**: In this example, we are relying on a non default service account `guestbook` created in the target cluster and namespace for the sync operation. This use case is for handling scenarios where the remote cluster is managed by a different administrator and providing a service account with `cluster-admin` level access is not feasible.

- Install ArgoCD in the `argocd` namespace.
```
kubectl apply -f https://raw.githubusercontent.com/argoproj/argo-cd/master/manifests/install.yaml -n argocd
```

- Enable the impersonation feature in ArgoCD.
```
kubectl set env statefulset/argocd-application-controller ARGOCD_APPLICATION_CONTROLLER_ENABLE_IMPERSONATION=true
```

- In the remote cluster, create a service account called `argocd-admin`
```
kubectl ctx remote-cluster
kubectl create serviceaccount argocd-admin
kubectl create clusterrole argocd-admin-role --verb=impersonate --resource="users,groups,serviceaccounts"
kubectl create clusterrole argocd-admin-role-access-review --verb=create --resource="selfsubjectaccessreviews"
kubectl create clusterrolebinding argocd-admin-role-binding --serviceaccount argocd-admin --clusterrole  argocd-admin-role
kubectl create clusterrolebinding argocd-admin-access-review-role-binding --serviceaccount argocd-admin --clusterrole  argocd-admin-role
```

- In the remote cluster, create a namespace called `guestbook` and a service account called `guestbook-deployer`.
```
kubectl ctx remote-cluster
kubectl create namespace guestbook
kubectl create serviceaccount guestbook-deployer
```

- In the remote cluster, create `Role` and `RoleBindings` and configure RBAC access for creating `Service` and `Deployment` objects in namespace `guestbook` for service account `guestbook-deployer`.
```
kubectl create role guestbook-deployer-role --verb get,list,update,delete --resource pods,deployment,service
kubectl create rolebinding guestbook-deployer-rb --serviceaccount guestbook-deployer --role guestbook-deployer-role
```

In this specific scenario, service account name `guestbook-deployer` will get used as it matches to the specific namespace `guestbook`.
```
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
spec:
  project: my-project
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    path: guestbook
  destination:
    server: https://kubernetes.default.svc
    namespace: guestbook
---
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: my-project
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  description: Example Project
  # Allow manifests to deploy from any Git repos
  sourceRepos:
    - '*'
  destinations:
    - namespace: guestbook
      server: https://kubernetes.default.svc
    - namespace: guestbook-ui
      server: https://kubernetes.default.svc
  destinationServiceAccounts:
    - namespace: guestbook
      server: https://kubernetes.default.svc
      defaultServiceAccount: guestbook-deployer
    - namespace: guestbook-ui
      server: https://kubernetes.default.svc
      defaultServiceAccount: guestbook-ui-deployer
```

### Special cases

#### Specifying service account in a different namespace

By default, the service account would be looked up in the Application's destination namespace configured through `Application.Spec.Destination.Namespace` field. If the service account is in a different namespace, then users can provide the namespace of the service account explicitly in the format <namespace>:<service_account_name>
eg:
```
  ...
  destinationServiceAccounts:
    - server: https://kubernetes.default.svc
      namespace: *
      defaultServiceAccount: mynamespace:guestbook-deployer
  ...
```

#### Multiple matches of destinations

If there are multiple matches for a given destination, the first valid match in the list of `destinationServiceAccounts` would be used.

eg:
Lets assume that the `AppProject` has the below `destinationServiceAccounts` configured.
```
  ...
  destinationServiceAccounts:
    - server: https://kubernetes.default.svc
      namespace: guestbook-prod
      defaultServiceAccount: guestbook-prod-deployer
    - server: https://kubernetes.default.svc
      namespace: guestbook-*
      defaultServiceAccount: guestbook-generic-deployer
    - server: https://kubernetes.default.svc
      namespace: *
      defaultServiceAccount: generic-deployer
  ...
```
- If the application destination namespace is `myns`, then the service account `generic-deployer` would be used as the first valid match is the glob pattern `*` and there are no other valid matches in the list.
- If the application destination namespace is `guestbook-dev` or `guestbook-stage`, then both glob patterns `*` and `guestbook-*` are valid matches, however `guestbook-*` pattern appears first and hence, the service account `guestbook-generic-deployer` would be used for the impersonation.
- If the application destination namespace is `guestbook-prod`, then there are three candidates, however the first valid match in the list is the one with service account `guestbook-prod-deployer` and that would be used for the impersonation.

#### Application resources referring to multiple namespaces
If application resources have hardcoded namespaces in the git repository, would different service accounts be used for each resource during the sync operation ?

The service account to be used for impersonation is determined on a per Application level rather than on per resource level. The value specified in `Application.spec.destination.namespace` would be used to determine the service account to be used for the sync operation of all resources present in the `Application`.

### Security Considerations

* How does this proposal impact the security aspects of Argo CD workloads ?
* Are there any unresolved follow-ups that need to be done to make the enhancement more robust ?

### Risks and Mitigations

#### Privilege Escalation

There could be an issue of privilege escalation, if we allow users to impersonate without restrictions. This is mitigated by only allowing admin users to configure service account used for the sync operation at the `AppProject` level.

Instead of allowing users to impersonate all possible users, administrators can restrict the users a particular service account can impersonate using the `resourceNames` field in the RBAC spec.


### Upgrade / Downgrade Strategy

If applicable, how will the component be upgraded and downgraded? Make sure this is in the test
plan.

Consider the following in developing an upgrade/downgrade strategy for this enhancement:

- What changes (in invocations, configurations, API use, etc.) is an existing cluster required to
  make on upgrade in order to keep previous behavior?
- What changes (in invocations, configurations, API use, etc.) is an existing cluster required to
  make on upgrade in order to make use of the enhancement?

- This feature would be implemented on an `opt-in` based on a feature flag and disabled by default.
- The new struct being added to `AppProject.Spec` would be introduced as an optional field and would be enabled only if the feature is enabled explicitly by a feature flag. If new property is used in the CR, but the feature flag is not enabled, then a warning message would be displayed during reconciliation of such CRs.


## Drawbacks

- When using this feature, there is an overhead in creating namespaces, service accounts and the required RBAC policies and mapping the service accounts with the corresponding `AppProject` configuration.

## Alternatives

### Option 1
Allow all options available in the `ImpersonationConfig` available to the user through the `AppProject` CRs.

```
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
  - namespace: *
    server: https://kubernetes.default.svc
    namespace: guestbook
    impersonate:
      user: system:serviceaccount:dev_ns:admin
      uid: 1234
      groups:
        - admin
        - view
        - edit
```

### Related issue

https://github.com/argoproj/argo-cd/issues/7689


### Related links

https://kubernetes.io/docs/reference/access-authn-authz/authentication/#user-impersonation

### Prior art

https://github.com/argoproj/argo-cd/pull/3377
https://github.com/argoproj/argo-cd/pull/7651