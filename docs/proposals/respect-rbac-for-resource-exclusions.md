---
title: Respect RBAC for Resource Inclusions/Exclusions

authors:
- "@gdsoumya"
- "@alexmt"

sponsors:
- TBD

reviewers:
- TBD

approvers:
- TBD

creation-date: 2023-05-03

---

# Enhancement Idea

This is a proposal to provide the ability to configure argocd controller, to respect the current RBAC permissions 
when handling resources besides the already existing resource inclusions and exclusions.

## Summary

Argo CD administrator will be able to configure in `argocd-cm`, whether to enable or disable(default) the feature where the controller will 
only monitor resources that the current service account allows it to read.

## Motivation

Some users restrict the access of the argocd to specific resources using rbac and this feature will enable them to continue 
using argocd without having to manually configure resource exclusions for all the resources that they don't want argocd to be managing.

## Proposal 

The configuration for this will be present in the `argocd-cm`, we will add new boolean field `resource.respectRBAC` in the
cm which can be set to `true` to enable this feature, by default the feature is disabled.

The feature will also modify `gitops-engine` pkg to add a `SelfSubjectAccessReview` request before adding any resource to the watch list, 
which will make sure that argocd only monitors resources that it has access to.

## Security Considerations and Risks

There are no particular security risks associated with this change, this proposal rather improves the argocd controller 
to not access/monitor resources that it does not have permission to access.

## Upgrade / Downgrade Strategy

There is no special upgrade strategy needed, all existing argocd configmaps will continue to work 
and old configs without the `resource.respectRBAC` config will cause no change in argocd controllers behavior.

While downgrading to older version, if the user had configured `resource.respectRBAC` previously this would be ignored completely 
and argocd would revert to its default behavior of trying to monitor all resources.