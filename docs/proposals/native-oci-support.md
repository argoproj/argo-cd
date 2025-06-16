---
title: Argo CD first-class OCI support
authors:
  - "@sabre1041"
  - "@crenshaw-dev"
  - "@todaywasawesome"
  
sponsors:
  - TBD
reviewers:
  - "@alexmt"
approvers:
  - "@alexmt"

creation-date: 2023-05-09
---

# Argo CD first-class OCI support

Storing and retrieving manifests within in OCI registries

## Summary

Currently, Argo CD supports obtaining manifests from either a Git repository, a Helm chart repository, or a Helm chart stored within an OCI registry. Given that OCI registries are more frequently being used to store content aside from container images, introduce a mechanism for storing and retrieving manifests that can be used by any of the existing supported tools in any of the supported methods of representing assets that are to be applied to a Kubernetes environment.


## Motivation

The industry is seeing a rapid adoption of OCI Artifacts as a method for storing and retrieving content. Adding support for sourcing resources stored in OCI artifacts not only provides immediate benefits, but opens up additional possible integrations in the future.

**Dependency Reduction**

 At the present time, a user must have access to either a Git repository, or a remote Helm chart repository. Most users or enterprise organizations already have access to an OCI registry as it represents the primary source of image related content within a Kubernetes environment. By sourcing assets from OCI registries, no additional infrastructure is required in order to store a variety of content types simplifying the set of requirements in order to begin to fully leverage the capabilities of Argo CD.

**Market Relevance**

Argo CD continues to be one of the most popular GitOps tools in the industry. As the industry continues to evolve, other tools within the GitOps market have already began to adopt OCI artifacts as a source for storing and retrieving GitOps resources.

### Goals

* Enable the retrieval of resources stored as artifacts in OCI registries that are formatted in any of the supported options (Kustomize, Jsonnet, Helm, plain-manifest, CMPs, etc)
* Define a format for storing resources that can be processed by Argo CD as an OCI artifact including the composition and [Media Type(s)](https://github.com/opencontainers/image-spec/blob/main/media-types.md)
* Support the retrieval of artifacts from OCI registries using custom / self signed TLS certificates.
* Support the retrieval of artifacts from OCI registries requiring authentication. 

### Non-Goals

* CLI Integration to package and publish resources in a format for storage in an OCI registry
* Attach metadata to OCI artifact manifest to provide additional details related to the content (such as original Git source [URL, revision])

## Proposal

This is where we get down to details of what the proposal is about.

### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

#### Publishing and retrieval of content from OCI registries:

As a user, I would like to make use of content that is represented by any of the  supported options (Kustomize, Jsonnet, Helm, plain-manifest, etc) or those that could be consumed using a Config Management Plugin from an OCI registry.

#### Authenticating to OCI registries:

As a user, I would like to enforce proper security controls by requiring authentication to an OCI registry and configure Argo CD to be able to interact with this registry.

#### CLI Integration:

As a user, I would like the ability to produce, store and retrieve resources (pull/push) in a OCI registry using the Argo CD CLI.

### Implementation Details/Notes/Constraints

The Argo CD repo-server currently maintains two types of clients - Helm and git. By adding a third client, and invoking it in the same places as the other two, we can support OCI artifacts.

It seems likely that we should create a new, common interface to represent all three clients. Then we can instantiate the client we need, toggling on whatever value in the repo config determines what kind of repo we're fetching from.

#### Format of OCI Artifact

An OCI artifact can contain any type of binary content. It is important that the content be formatted in a manner that can be consumed by Argo CD.

#### Content

Resources that is consumed by Argo CD can be represented by a series of files and folders. To be stored within an OCI artifact, these assets are stored within a compressed tar archive (.tar.gz) OCI layer. The [OCI Image Specification](https://specs.opencontainers.org/image-spec/) allows for metadata to be added through the use of annotations to provide attribute based details describing the included content. This level of detail is important as it satisfies many of the existing capabilities of Argo CD for tracking content, such as Git repository URL, branch name/revision.


#### Media Types

The [OCI Image Specification](https://specs.opencontainers.org/image-spec/) makes extensive use of Media Types to identity the format of content. To provide not only a way that signifies the content of the OCI artifact contains Argo CD manifests, but to define the structure of the content. An understanding of the composition and requirements enable a broad ecosystem of tooling that can be used to produce and consume Argo CD resources within OCI registries.

Two new Media Types will be used for this purpose as defined below:

* `application/vnd.cncf.argoproj.argocd.content.v1.tar+gzip` - Primary asset stored within the OCI artifact containing a gzip compressed tar archive of Argo CD resources. Further details are outlined in the prior section.
* `application/vnd.cncf.argoproj.argocd.config.v1+json` - An [OCI Image Configuration](https://specs.opencontainers.org/image-spec/config/)


### Detailed examples


### Security Considerations

The direct integration with an external endpoint from the core subsystem of Argo CD introduces several considerations as it relates to security. It is worthy to note that Argo CD currently does support sourcing Helm charts that are stored within OCI registries. However, this interaction is performed by Helm and its underlying library, [ORAS](https://oras.land), and not Argo CD itself. Capabilities included within this proposal can make use of the same libraries to facilitate the interaction.

#### Credentials

Security controls may be enforced within the OCI registry to enforce that clients authenticate. The introduction of additional mechanisms to authenticate against target systems is outside the scope of this proposal. However, an integration with existing capabilities and features, such as sourcing from _repository_ credentials is required.


### Risks and Mitigation's

#### Overlap with existing Helm OCI integration

Argo CD already includes support for sourcing Helm Charts from OCI registries and the retrieval is delegated to functionality provided by Helm. Considerations must be taken into account to determine whether the intent by the end user is to consume an OCI artifact containing Argo CD related resources or a Helm chart. One such method for addressing this concern is to inspect the `mediaType` of the OCI artifact.


### Upgrade / Downgrade Strategy

If applicable, how will the component be upgraded and downgraded? Make sure this is in the test
plan.

Consider the following in developing an upgrade/downgrade strategy for this enhancement:

- What changes (in invocations, configurations, API use, etc.) is an existing cluster required to
  make on upgrade in order to keep previous behavior?
- What changes (in invocations, configurations, API use, etc.) is an existing cluster required to
  make on upgrade in order to make use of the enhancement?

## Drawbacks

* Sourcing content from an OCI registry may be perceived to be against GitOps principles as content is not sourced from a Git repository. This concern could be mitigated by attaching additional details related to the content (such as original Git source [URL, revision]). Though it should be noted that the GitOps principles only require a source of truth to be visioned and immutable which OCI registries support.

## Alternatives

### Config Management Plugin

Content stored within OCI artifacts could be sourced using a Config Management Plugin which would not require changes to the core capabilities provided by Argo CD. However, this would be hacky and not represent itself within the Argo CD UI.
