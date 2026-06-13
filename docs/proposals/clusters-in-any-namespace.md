---
title: Neat-enhancement-idea
authors:
  - "@chetan-rns" # Authors' github accounts here.
sponsors:
  - "@todaywasawesome"        # List all interested parties here.
reviewers:
  - "@alexmt"
  - TBD
approvers:
  - "@alexmt"
  - TBD

creation-date: 2023-11-29
last-updated: 2023-11-29
---

# Clusters in any namespace

Allow users to create and manage cluster secrets in a non-control plane namespace and thereby improve the self-service model of Argo CD.

## Summary

Currently, Argo CD supports creating cluster secrets only in a control-plane namespace where Argo CD is running. Hence, it becomes an admin's responsibility to configure and manage clusters. As organizations are moving towards multi-tenancy where a single Argo CD instance could be shared between multiple tenants, it increases the operational burden and reliability on the admins to configure the clusters for tenants. Although it is possible to improve this process by using [project-scoped clusters](https://argo-cd.readthedocs.io/en/stable/user-guide/projects/#project-scoped-repositories-and-clusters), tenants would still require permission to manage clusters in the Argo CD control plane namespace.

By enabling tenants to create and manage clusters in their namespace, we rely on a self-service approach with minimal admin intervention thereby improving the user experience. As the number of tenants increases, admins need not worry about managing their access to the control plane namespace.

## Motivation

As a user of Argo CD, I can create Applications in my non-control-plane namespace but I cannot create clusters in the same namespace alongside my Applications. I need to request the admin(someone with access to the Argo CD's control plane namespace) to configure the clusters for me. As a user who owns the Application, I should be able to create and use clusters in my tenant namespace.

### Goals

* Allow tenants to declaratively manage cluster secrets in a self-service manner without admin intervention. Tenants can manage the cluster secrets declaratively along with their Applications in a non-control-plane namespace.
* Allow tenants to declaratively manage cluster secrets without providing them access to the control plane namespace.

### Non-Goals

* Allow tenants to share cluster secrets across the namespace. Tenants cannot use cluster secrets that are created in a different namespace.

## Proposal

The objective of this proposal is to extend Argo CD's capabilities to reconcile cluster secrets in a non-control-plane namespace. As a prerequisite, users should enable Argo CD to be cluster-scoped to manage cluster secrets across the cluster. The application in any namespace feature should be enabled by specifying the `--application-namespaces` flag. It may not be useful to support clusters in any namespace if Applications cannot be created in any namespace. Also, it is safer to reconcile cluster secrets from these trusted namespaces, since an admin configures the `--application-namespaces` flag.

The cluster secrets in the non-control-plane namespace must be scoped to a project. Argo CD already supports [scoping clusters to a project](https://argo-cd.readthedocs.io/en/stable/user-guide/projects/#project-scoped-repositories-and-clusters). A cluster can be scoped to a project by specifying the `.data.project` field of the cluster secret and enabling `.spec.permitOnlyProjectScopedClusters` of the AppProject. When a cluster is scoped to a particular project it is automatically added to the project’s destination and is verified against the project’s permission. The secret's namespace should be listed in the `.spec.sourceNamespaces` of the project. Argo CD should reject the cluster if it is not scoped or scoped to a project whose `.spec.sourceNamespaces` doesn’t have the secret’s namespace. Clusters in the control plane namespace could either be unscoped or scoped to a particular project.

The cluster secret created by the user must respect the destination rules mentioned in the project. So, if a user tries to use a cluster that is not allowed by the scoped project, Argo CD will throw an error. This adds a layer of security since admins can still control where the users are deploying their resources or restrict them from certain clusters or namespaces.

Here’s a sample cluster secret created in a user namespace but scoped to a project in the `argocd` namespace.  

```yaml
kind: Secret
metadata:
  labels:
    argocd.argoproj.io/secret-type: cluster
  name: test                              
  namespace: user-tenant        # Cluster created in a non-argocd namespace
type: Opaque                   
data:
  server: https://kubernetes-api-server.com:4663
  project: sample             # Project scoped: Indicates that the user can self-service the cluster without help from admin       
----

apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: sample
  namespace: argocd           # Project managed by an argocd admin
spec:
  permitOnlyProjectScopedClusters: true # Restricts apps to only use clusters that are part of this project.      

```

Here is the list of possible scenarios we may have to address while implementing clusters in any namespace:

1. Cluster secrets in any namespace point to different remote clusters. In this scenario, users in each application namespace have their independent target clusters and create cluster secrets that point to these remote clusters. This is an ideal scenario since the remote clusters are unique and there should be no issues.
2. Cluster secrets in any namespace point to the same remote cluster but with different configurations. Users in application namespaces will deploy to the same remote cluster but target different namespaces with different sets of permissions. This is still a common scenario because many tenants/teams share the same cluster with namespaces as a boundary. For example, tenant A wants to deploy to namespaces dev and stage of a target cluster. Whereas, tenant B wants to deploy to namespace prod of the same cluster.
3. Cluster secrets in any namespace point to the same remote cluster and with the same configuration. For example, tenant A wants to deploy to the namespace prod of a target cluster. Whereas, tenant B also wants to deploy to the same namespace prod of the same cluster.

Currently, Argo CD doesn't support multiple cluster secrets pointing to the same cluster preventing us from supporting scenarios 2 and 3. I have added more details in the Implementation section on how to overcome this problem.

Can we use impersonation instead of multiple cluster secrets?

[Impersonation](https://github.com/argoproj/argo-cd/pull/14255) could be used when users have multiple credentials for the same cluster. However, Impersonation is more admin-focused where admins have to manage the cluster secrets and also map different service accounts to the right destination. In contrast, multiple cluster secrets follow the "self-service" approach with each user/tenant managing their cluster secret. So, I believe both solutions can coexist and users could have an option to select one based on their requirements.

### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

#### Use case 1

As a user, I would like to declaratively manage cluster secrets in my tenant namespace without admin intervention.

#### Use case 2

As an Argo CD admin, I would like to enable my tenants to manage cluster secrets in a self-service manner without granting them additional RBAC privileges. However, I should still be able to configure the rules and restrictions because each cluster secret will be scoped to an AppProject.  

#### Use case 3

As a user, I would like to create multiple cluster secrets that target the same cluster. I should not worry if the same cluster is part of a different cluster secret in a different tenant namespace. Argo CD should deploy the resources to the target cluster using the credentials specified in the secret.  

### Implementation Details/Notes/Constraints [optional]

#### Support for watching cluster secrets across the cluster

Argo CD watches the secrets in the `argocd` namespace using an informer. We should update the scope of the informers to watch secrets in any namespace by setting the namespace parameter to an empty string. Argo CD should only reconcile secret events from namespaces that are specified by the `--application-namespaces` flag. If the flag is not set, we should revert back to the old behavior and watch secrets only in the `argocd` namespace. The `argocd-server` and the `application controller` must perform additional validation to reject cluster secrets that are created in the non-control plane namespace but are not scoped to a valid project.

#### Changes to the UI/CLI

The argocd cluster command must accept a namespace flag that indicates the namespace in which the cluster secret will be created. Since there is already a flag called namespace(list of namespaces that are allowed to be managed), we could use `–cluster-namespace` or something along those lines. The cluster details UI should also display the namespace of the cluster secret.

#### Changes to the Cluster Cache

Currently, Argo CD maintains a cache for managing resources in the cluster known as [ClusterCache](https://github.com/argoproj/gitops-engine/blob/master/pkg/cache/cluster.go#L180). Each cluster will be cached independently with the server URL as the key. There would be no issue if users in each namespace were using cluster secrets with a unique URL. However, if clusters can be created in any namespace, Argo CD should be able to handle a situation where users in different namespaces could create cluster secrets with the same URL. One option is to treat each cluster secret as a different cluster and maintain a separate cache. Each cluster cache could be identified as `<secret_name>/<namespace>` instead of only server_url.

There is already a [proposal](https://github.com/argoproj/argo-cd/pull/12755) and possible [implementation](https://github.com/argoproj/argo-cd/pull/10897) that talks in depth about changing the cluster cache key to a combination of name and namespace. But the downside here is Argo CD may maintain duplicate connections to the same cluster. However, if we can make this feature optional, users can assess the tradeoffs(complexity and scalability) and enable it if required. I would be happy to hear more ideas on how to solve this problem.

### Detailed examples

Let us consider an example where two tenants A and B are using a single Argo CD instance. They want to manage cluster secrets in their namespace and deploy resources to different namespaces of a remote cluster. Tenant A will create a cluster secret in the namespace `user-tenant-A` with the credentials to namespaces `dev` and `stage` of the target cluster(`https://kubernetes-api-server.com:4663`). Tenant B will create a cluster secret in the namespace `user-tenant-B` with credentials to the namespaces `stage` and `prod` of the same target cluster. Both the cluster secrets are scoped to an AppProject `sample` in the control-plane namespace. Argo CD should allow both tenants to deploy the resources using the credentials specified in their respective secrets.

```yaml

kind: Secret
metadata:
  labels:
    argocd.argoproj.io/secret-type: cluster
  name: test                              
  namespace: user-tenant-A        
type: Opaque                   
data:
  server: https://kubernetes-api-server.com:4663
  project: sample
  namespaces: dev, stage

----
kind: Secret
metadata:
  labels:
    argocd.argoproj.io/secret-type: cluster
  name: test                              
  namespace: user-tenant-B     
type: Opaque                   
data:
  server: https://kubernetes-api-server.com:4663
  project: sample
  namespaces: stage, prod

----

apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: sample
  namespace: argocd
spec:
  permitOnlyProjectScopedClusters: true
```

### Security Considerations

* How does this proposal impact the security aspects of Argo CD workloads ?
* Are there any unresolved follow-ups that need to be done to make the enhancement more robust ?

### Risks and Mitigations

1. An attacker or a bot might consider a DDOS attack to overwhelm Argo CD by creating a large number of cluster secrets in different namespaces. To mitigate this problem, Argo CD should only reconcile cluster secrets that are created by trusted tenants. We reconcile secrets only from the namespaces specified in the `--application-namespaces` flag. Since this flag is usually set by an Argo CD admin, we can assume a certain level of trust in these namespaces. Even if an attacker has access to one of these valid namespaces the admin could easily disable these namespaces by updating the flag.
2. A user might try deploying to a destination that is prohibited. Every cluster secret created by a tenant is scoped to an AppProject that is managed by an admin. The admin could configure the AppProject rules to prevent users from accessing a prohibited destination.

### Upgrade / Downgrade Strategy

Existing Argo CD users should not be affected during upgrade/downgrade as long as they have clusters only in the control-plane namespace. Once they upgrade to a version with this feature, an admin can enable this feature by specifying the `--application-namespaces` flag. If the flag was already present because of Applications in any namespace feature, Argo CD starts reconciling cluster secrets from these defined namespaces.

Downgrading will affect the users because Argo CD will no longer reconcile the clusters from a non-control-plane namespace. Applications using these clusters might fail because Argo CD doesn't recognize the cluster anymore. Users must make sure to migrate cluster secrets from different namespaces to a control-plane namespace before downgrading to a previous version.

## Open Questions [optional]

1. How should we handle multiple cluster secrets pointing to the same URL?

## Drawbacks

1. The Cluster cache needs to be updated to support cluster secrets with the same URL.
2. Downgrading cannot be easily achieved.
  
## Alternatives

Continue with the present approach of managing cluster secrets in a control-plane namespace.
