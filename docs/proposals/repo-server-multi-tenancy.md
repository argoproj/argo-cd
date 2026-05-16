---
title: Multi-tenant aware manifest generation in Argo CD
authors:
  - "@jannfis" # Authors' github accounts here.
sponsors:
  - TBD        # List all interested parties here.
reviewers:
  - TBD
approvers:
  - TBD

creation-date: yyyy-mm-dd
last-updated: yyyy-mm-dd
---

# Multi-tenant aware manifest generation in Argo CD

## Open Questions [optional]

* Question ...

## Summary

This proposal describes a way to enable isolated, multi-tenancy aware manifest generation in Argo CD to increase confidentiality and security when handling manifests for multiple parties that do not mutually trust each other.

## Motivation

As of today, Argo CD's multi-tenancy capacities are somewhat limited. While Argo CD has the concept of limiting the permissions an `Application` is granted to any target cluster by associating that `Application` to an `AppProject`, the current manifest generation mechanisms do not adopt that concept. 

The repository server itself, simply spoken, is "just a process in a Kubernetes pod", exposed via a private gRPC API through a Kubernetes service, that performs manifests render requests on behalf of the application controller or the API server. The repository server is not aware of the concepts of an `Application`, an `AppProject`, or any tenancy model at all. In fact, it does not have access to the Kubernetes cluster it runs on, or knows that it runs in Kubernetes at all. All it cares about is rendering manifests and returning the results to the caller.

This eventually leads to several gaps in the multi-tenancy model, because all users of an Argo CD installation share the same instance(s) of the repository server. This can lead to some serious gaps in isolation of tentants, especially when tools such as Kustomize, Helm or custom management plugins are being used to render the manifests. Most of these tools are not designed to be executed in a way that supports multi-tenancy or isolation. In a Kubernetes context, the pod executes as an unprivileged user account with privilege escalation disabled, making isolation on the file system level (e.g. using different file and directory permissions or any other user based restrictions) practically impossibly as of now.

The initial design assumption for the repository server was, that it will not have access to any confidential data (e.g. treat manifests and contents in Git as non-confidential), and as noted above, it does not even have access to the Kubernetes cluster it is running on. With this assumption, it wouldn't matter if a tool executed on behalf of user A could also read manifest data from user B, due to the semi-public nature of the manifests. However, this premise has changed over the course of time, and the Argo CD community came up with solutions for secrets management and has integrated tools like git-crypt, helm-secrets and others, all of which require a private key to be known inside the repository server. Furthermore, tools like Helm and Kustomize have evolved to allow all kinds of things, such as executing commands as plugins, refering to files outside the current repository, and more. This would make it rather easy to read any secret stored within the repo server's file system for users who can control the source of any application rendered by these tools.

There have been some changes made recently to increase the difficulty for any tool executed on behalf of user A to gain access to resources of user B, including creating random, unpredictable file system path names. However, this is more _security by obscurity_ than anything else. Also, the Config Management Plugin (CMP) v2 approach implements some level of isolation, but it does not go far enough to provide a strict isolation required by tenants that do not trust each other.

### Goals

* Provide proper isolation for the repository server when Argo CD is setup in a scenario where different users/tenants do not trust each other.
* Allow different parties (i.e. "tenants") the safe usage of tools that require secret data, such as a private key mounted into the repo server, to be available in order to render manifest data
* Limit the impact of rogue or misbehaving tools
* Open the door for more use cases in the future, like managing confidential data (such as private PGP keys) on a per-tenant (AppProject) basis

### Non-Goals

* There is no intention to replace CMP v2, but rather allow it to be leveraged in non-trusting multi-tenancy environments.
* There are no intentions for architectural changes to the repository server itself, or the gRPC API for communication with the repository server

## Proposal

### Overview

We propose a change of how the repository server workloads are integrated with Argo CD.

Currently, there is a 1:1 relationship between the `argocd-server` and `argocd-application-controller` workloads to the `argocd-repo-server` workload. This relationship is established using the command-line parameter `--repo-server <address>:<port>`, which specifies which repository server to use.

This proposal suggests moving towards a 1:n relationship, so that there will be multiple, independent replicasets of `argocd-repo-server`, each one accessible by its own Kubernetes service.

We do not propose to remove the default repository server instance, neither from the Argo CD manifests, nor the parametrization from the `argocd-server` and the `argocd-application-controller` workloads. This should keep existing, for ease of use and backwards compatibility (see the section about mapping below). When installing Argo CD, the user should not have a different experience from how it works now.

### Configuring additional repository server services to use

While currently, the repository server to use is configured using the command line for the `argocd-server` and `argocd-application-controller` workloads, this wouldn't be properly suitable to specify multiple instances to be used. The command line could become really large and obscure, and it would have to be synchronized between `argocd-server` and `argocd-application-controller`. Also, any change would require a restart of both, the `argocd-server` and `argocd-application-controller` which should be prevented.

We propose to move this configuration into either the `argocd-cm` or to introduce a new Custom Resource, which would be consumed by the `argocd-server` and `argocd-application-controller`. The configuration for a single repository server instance would include the following items:

```yaml=
# A name for the instance. Will be used as back-reference. The name must
# be compliant to RFC 1123 label names, as described in https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names
name: tenant-a
# The DNS name or IP address of the repo-server's K8s service
hostname: tenant-a-repo-server.argocd
# The port of the repo-server service
port: 8081
# TLS configuration (optional)
tls:
  # Whether to ignore the remote TLS certificate 
  insecure: false
  # A root CA certificate to validate the TLS connection
  caCert: |
    ---- BEGIN CERTIFICATE ----
    ...
    ---- END CERTIFICATE ----

```

However, we do not propose to remove the appropriate command line options. The command line options (or their default values) will be used for the default repository server instance.

### Mapping tenants to repository servers

We propose to use the `AppProject` custom resource to map a project to a specific repository server instance. Given the above configuration example for a repository server instance named `tenant-a`, the `AppProject` configuration could simply look like the following:

```yaml=
kind: AppProject
apiVersion: argoproj.io/v1alpha1
metdata:
  name: tenant-a-project-a
spec:
  repoServer: tenant-a
```

The new field `.spec.repoServer` will be an optional field in the `AppProject` spec.

If the `.spec.repoServer` field is not specified in the `AppProject`, the default repository server (as specified implicit, or on the command line) will be used to generate manifests for Applications associated to this AppProject. This keeps the current behavior of Argo CD, and is fully backward compatible.

If the `.spec.repoServer` field is specified, both `argocd-server` and `argocd-application-controller` would look up the appropriate configuration for the repository server instance to use, and create a gRPC clientset specifically for this instance when submitting requests to the repository server. When the referenced configuration is not found, there is no fallback to the default one. Instead, an error would be produced and manifest generation will fail. This is important for security reasons, so that a typo or misconfiguration would not lead to a potential flaw in isolation.

The relationship of an `AppProject` to a repository server instance is `1:n`. An `AppProject` instance can use exactly one repository server instance, while each repository server instance can be referenced by multiple `AppProjects`. 

### Changes to caching

Redis is used as a central instance to cache manifests generated by the repository server. The manifest cache currently uses keys that do not consider multiple independent repository servers storing manifests in the same cache. Also, by default there is no authentication between the repository server and the cache, and access control is solely implemented using network policies.

There are a couple of challenges to be solved with the cache:

1. Repository server instances must be able to make distinct cache entries, possibly for the same repositories and same revisions.
1. Any repository server instance must not be allowed to gain access to cache entries from another repository server instance

In initial discussions, a possible solution for the first challenge would be to prefix cache keys with the instance name of the repository server (e.g. `instance-1`). This is easy to implement, and only requires minimal changes and efforts to the code.

For the second challenge however, there have been multiple ideas of various complexity.

**1. Using authentication and ACLs in Redis**: This would involve setting up users and ACLs in Redis, and augmenting repository server instance configuration with credentials. This is however, complex to setup and maintain and could also prevent some users from using external (managed) Redis instance where this is not supported or would cost additional money.

**2. Moving the manifest cache to the application controller:** Another idea is to move the handling of the cache completely to the application controller, and remove the relationship between repository server and the cache. This would also save some network traffic between the application controller and the repository server, which would be especially efficient when the application controller requests manifests from the repository server and the repository server would just return the cached manifests.

**3. Encryption of cache entries**: The third idea revolves around encrypting the actual manifests stored in the cache, using a static encryption key unique to each repository server instance. This would prevent other repository server instances to gain manifests from other instances, ensuring confidentiality of the manifests in the cache (which would especially benefit manifests that are confidential and are decrypted on generation time, such as git-crypt). This would come at cost of some compute resources, but probably be worth it.

At the current state of discussions, my preference would be a combination of 2 and 3. We could start with implementing encryption of cached resources to satisfy the confidentiality constraint for multiple repository server instances, then in a later step move the cache to the application controller completely.

### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

#### Use case 1:

As a user in a multi-tenant Argo CD setup, I want to rest ensured that my manifest data coming from Git is still safe and sound once processed by Argo CD, and that no confidential data will possibly leak to other users of my instance.

#### Use case 2:

As an Argo CD administrator, I want to ensure that a plugin requiring a secret key (such as argocd-vault-plugin or git-crypt) which I configure for a certain tenant can't be used by another tenant, or that the secret key will not leak to another tenant.

#### Use case 3:

As an Argo CD administrator, I want to be able to provide my tenants with unique custom tooling that does not interfere with tooling set up for other tenants.

#### Use case 4:

As an Argo CD administrator, I want to give different tenants a different set of compute resources or priorities for running their manifest generation tools. Also, I want to make sure that no tenant can block other tenants by running manifest generation tools that consume a large amount of compute resources, or use a large amount of manifests.

### Implementation Details/Notes/Constraints

Setting up additional repo-server instances requires some efforts from the Argo CD administrator. Also, those additional instances will consume additional compute resources on the control plane cluster.

However, we think the improvements this brings is well worth the additional effort in set-up and administration. To ease people with the setup of new repository server instances, we could provide a simple Kustomize base such as:

```yaml=
resources:
- https://github.com/argoproj/argo-cd/manifests/base/repo-server?ref=v2.4.12

namePrefix: tenant-a-

commonLabels:
  app.kubernetes.io/name: tenant-a-argocd-repo-server
```

Please note that the above `kustomization.yaml` is a highly simplified version, and should just serve as an example of the idea. The final configuration needs to take things like configuration environment variables etc into consideration.

The Argo CD Helm chart could also be extended to support setting up multiple repository server instances using new values and templates.

Other methods of installing Argo CD (e.g. the argocd-operator) would have to adapt to multiple repository server instances as well.

### Detailed examples

To be filled.

### Security Considerations

By isolating AppProjects from each other also on the layer of manifest generation will - in our opinion - greatly improve the overall security and integrity of Argo CD in a multi-tenant environment.

Each repository server instance will run in its own set of pods, which is the highest level of isolation a Kubernetes workload currently can assume. The pods of each instance can then also use their own `ServiceAccount` with dedicated K8s RBAC configuration.

### Risks and Mitigations

To be filled.

### Upgrade / Downgrade Strategy

Upgrade to a version implementing this feature should be straight forward and does not require any special attention. Since the new `.spec.repoServer` field in the `AppProject` is optional, and the fallback if not specified is to use the default repository instance, users wouldn't be required to perform any special configuration before or after the upgrade.

Downgrading from the version implementing this feature to a previous version not implementing this feature could be a little bit more involved, due to the new field in the `AppProject` CRD. However, the previous version of Argo CD should be fine using a newer version of the CRD, and Argo CD would just ignore the new field.

## Drawbacks

To be filled.

## Alternatives

To be filled.