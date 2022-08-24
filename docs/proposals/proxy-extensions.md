---
title: Reverse Proxy Extensions

authors:
- "@leoluz"

sponsors:
- TBD

reviewers:
- TBD

approvers:
- TBD

creation-date: 2022-07-23

---

# Reverse-Proxy Extensions support for ArgoCD

Enable UI extensions to use a backend service.

* [Summary](#summary)
* [Motivation](#motivation)
* [Goals](#goals)
* [Non-Goals](#non-goals)
* [Proposal](#proposal)
    * [Use cases](#use-cases)
    * [Security Considerations](#security-considerations)
    * [Risks and Mitigations](#risks-and-mitigations)
    * [Upgrade / Downgrade](#upgrade--downgrade)
* [Drawbacks](#drawbacks)
* [Open Questions](#open-questions)

---

## Summary

ArgoCD currently supports the creation of [UI extensions][1] allowing
developers to define the visual content of the "more" tab inside
a specific resource. Developers are able to access the resource state to
build the UI. However, currently it isn't possible to use a backend
service to provide additional functionality to extensions. This proposal
defines a new reverse proxy feature in ArgoCD, allowing developers to
create a backend service that can be used in UI extensions.

## Motivation

The initiative to implement the anomaly detection capability in ArgoCD
highlighted the need to improve the existing UI extensions feature. The
new capability will required the UI to have access to data that isn't
available as part of Application's owned resources. It is necessary to
access an API defined by the development team so the proper information
can be displayed.

## Goals

All following goals should be achieved in order to conclude this proposal:

#### [G-1] Enhance the current Extensions framework to configure backend services

#### [G-2] ArgoCD (API Server) must have low performance impact when running extensions

#### [G-3] ArgoCD deployment should be independent from backend services

#### [G-4] ArgoCD admins should be able to define rbacs to define which users can invoke specific extensions

## Non-Goals

TBD

## Proposal

### Use cases

The following use cases should be implemented:

#### [UC-1]: As a developer, I would like to invoke a backend service from my ArgoCD UI extension so I can provide richer UX

### Security Considerations

- ArgoCD must authorize requests coming from UI and check that the
  authenticated user has access to invoke a specific URL belonging to an
  extension

### Risks and Mitigations

### Upgrade / Downgrade

## Drawbacks

- Slight increase in ArgoCD code base complexity.

## Open Questions

[1]: https://argo-cd.readthedocs.io/en/stable/developer-guide/ui-extensions/
