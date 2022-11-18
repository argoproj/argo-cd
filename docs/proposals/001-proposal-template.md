---
title: Neat-enhancement-idea
authors:
  - "@sbose78" # Authors' github accounts here.
sponsors:
  - TBD        # List all interested parties here.
reviewers:
  - "@alexmt"
  - TBD
approvers:
  - "@alexmt"
  - TBD

creation-date: yyyy-mm-dd
last-updated: yyyy-mm-dd
---

# Neat Enhancement Idea

This is the title of the enhancement. Keep it simple and descriptive. A good title can help
communicate what the enhancement is and should be considered as part of any review.


## Open Questions [optional]

This is where to call out areas of the design that require closure before deciding to implement the
design.


## Summary

The `Summary` is required for producing accurate user-focused documentation
such as release notes or a development roadmap. It should be possible to collect this information
before implementation begins in order to avoid requiring implementors to split their attention
between writing release notes and implementing the feature itself. Before you get started with this document,
please feel free to have a conversation on this with the maintainers/community on Github that would help
drive a more organized thought process for the formal proposal here.

## Motivation

This section is for explicitly listing the motivation, goals and non-goals of this proposal.
Describe why the change is important and the benefits to users.

### Goals

List the specific goals of the proposal and their measurable results. How will we know that this has succeeded?

### Non-Goals

What is out of scope for this proposal? Listing non-goals helps to focus discussion and make
progress.

## Proposal

This is where we get down to details of what the proposal is about.

### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

#### Use case 1:
As a user, I would like to understand the drift. (This is an example)

#### Use case 2:
As a user, I would like to take an action on the deviation/drift. (This is an example)

### Implementation Details/Notes/Constraints [optional]

What are the caveats to the implementation? What are some important details that didn't come across
above. Go in to as much detail as necessary here. This might be a good place to talk about core
concepts and how they relate.

You may have a work-in-progress Pull Request to demonstrate the functioning of the enhancement you are proposing.

### Detailed examples

### Security Considerations

* How does this proposal impact the security aspects of Argo CD workloads ?
* Are there any unresolved follow-ups that need to be done to make the enhancement more robust ?

### Risks and Mitigations

What are the risks of this proposal and how do we mitigate. Think broadly.

For example, consider
both security and how this will impact the larger Kubernetes ecosystem.

Consider including folks that also work outside your immediate sub-project.


### Upgrade / Downgrade Strategy

If applicable, how will the component be upgraded and downgraded? Make sure this is in the test
plan.

Consider the following in developing an upgrade/downgrade strategy for this enhancement:

- What changes (in invocations, configurations, API use, etc.) is an existing cluster required to
  make on upgrade in order to keep previous behavior?
- What changes (in invocations, configurations, API use, etc.) is an existing cluster required to
  make on upgrade in order to make use of the enhancement?

## Drawbacks

The idea is to find the best form of an argument why this enhancement should _not_ be implemented.

## Alternatives

Similar to the `Drawbacks` section the `Alternatives` section is used to highlight and record other
possible approaches to delivering the value proposed by an enhancement.