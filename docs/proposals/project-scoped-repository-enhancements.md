---
title: Project scoped repository credential enhancements
authors:
  - "@blakepettersson" 
sponsors:
  - TBD
reviewers:
  - "@alexmt"
approvers:
  - "@alexmt"

creation-date: 2024-05-17
last-updated: 2024-05-22
---

# Project scoped repository credential enhancements

## Summary

This is to allow the possibility to have multiple repository credentials which share the same URL. Currently, multiple repository
credentials sharing the same URL is disallowed by the Argo CD API.

## Motivation

This is to allow the possibility to have multiple repository credentials which share the same URL. Currently, multiple repository
credentials sharing the same URL is disallowed by the Argo CD API. If the credentials are added directly to the `argocd`
namespace, we "get around" `argocd-server` returning an error, but this still does not work since the first secret that 
matches a repository URL is the one that gets returned, and the order is also undefined. 

The reason why we want this is due to the fact that in a multi-tenant environment, multiple teams may want to 
independently use the same repositories without needing to ask an Argo CD admin to add the repository for them, and then
add the necessary RBAC in the relevant `AppProject`s to prevent other teams from having access to the repository 
credentials. In other words, this will enable more self-service capabilities for dev teams. 

### Goals

The goal of this proposal is to allow multiple app projects to have the ability to have separate repository credentials 
which happen to share the same URL.

### Non-Goals

- Having multiple repository secrets sharing the same URL _within the same_ `AppProject`.
- Allowing a single repository credential to be used in multiple `AppProject`s. 
- Preventing non project-scoped repository credentials from being used by an Application.
- Extending this to repository credential templates.

## Proposal

There are a few parts to this proposal.

The first part of this proposal is to change how the selected credential gets selected. Currently, the first repository 
secret that matches the repository URL gets returned.

What this proposal instead aims to do is to first find the first `repository` secret which matches the
`project` of the application that is making the request for the repository credentials. If there are no credentials 
which match the requested `project`, it will fall back to returning the first unscoped credential, i.e, the first credential
with an empty `project` parameter.

This change would apply when we retrieve a _single_ repository credential. For listing repository credentials, nothing 
changes - the logic would be the same as today. 

When it comes to mutating a repository credential we need to strictly match the project which the cred belongs to, since 
there would otherwise be a risk of changing (inadvertently or otherwise) a credential not belonging to the correct project.

The third part is specifically for when we imperatively create repository secrets. Currently, when we create a repository
secret in the UI/CLI, a suffix gets generated which is a hash of the repository URL. This mechanism will be extended to 
also hash the repository _project_.

On the API server side no major changes are anticipated to the public API. The only change we need to do from the API 
perspective is to add a `project` parameter when deleting a repository credential. To preserve backwards compatibility
this option is optional and would only be a required parameter if multiple repository credentials are found for the same 
URL.

### Use cases

TODO

#### Use case 1:

TODO

### Security Considerations

Special care needs to be taken in order not to inadvertently expose repository credentials belonging to other `AppProject`s.
Access to repositories are covered by RBAC checks on the project, so we should be good.

### Risks and Mitigations

### Upgrade / Downgrade Strategy

When upgrading no changes need to happen - the repository credentials will work as before. On the other hand, when 
downgrading to an older version we need to consider that the existing order in which multiple credentials with the same
URL gets returned is undefined. This means that deleting the credentials before downgrading to an older version would be
advisable.

## Drawbacks

* It will be more difficult to reason about how a specific repository credential gets selected. There could be scenarios 
where a repository has both a global repository credential and a scoped credential for the project to which the 
application belongs.
* There will be more secrets proliferating in the `argocd` namespace. This has the potential to increase maintenance burden
to keeping said secrets safe, and it also makes it harder to have a bird's eye view from an Argo CD admin's perspective.
* Depending on the number of projects making use of distinct credentials for the same repository URL, loading the correct 
credentials from the repository secrets has the potential to scale linearly with the number of app projects (in the worst case 
scenario we would need to loop through all the credentials before finding the correct credential to load). This is likely 
a non-issue in practice.

## Alternatives

To keep the existing behavior of having a single repository credential shared by multiple `AppProject`s. It would be up 
to the Argo CD admins to ensure that a specific repository credential cannot be used by unauthorized parties.