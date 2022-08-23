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
    * [Non-Functional Requirements](#non-functional-requirements)
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

## Goals

All following goals should be achieve in order to conclude this proposal:

## Non-Goals

TBD

## Proposal


### Use cases

### Security Considerations

TBD

### Risks and Mitigations

### Upgrade / Downgrade

## Drawbacks

- Slight increase in ArgoCD code base complexity.

## Open Questions

[1]: https://argo-cd.readthedocs.io/en/stable/developer-guide/ui-extensions/
