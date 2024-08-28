# Submitting code contributions to Argo CD

## Preface

The Argo CD project continuously grows, both in terms of features and community size. It gets adopted by more and more organisations which entrust Argo CD to handle their critical production workloads. Thus, we need to take great care with any changes that affect compatibility, performance, scalability, stability and security of Argo CD. For this reason, every new feature or larger enhancement must be properly designed and discussed before it gets accepted into the code base.

We do welcome and encourage everyone to participate in the Argo CD project, but please understand that we can't accept each and every contribution from the community, for various reasons.

If you want to submit code for a great new feature or enhancement, we kindly ask you to take a look at the
enhancement process outlined below before you start to write code or submit a PR. This will ensure that your idea is well aligned with the project's strategy and technical requirements, and it will help greatly in getting your code merged into our code base.

Before submitting code for a new feature (and also, to some extent, for more complex bug fixes) please
[raise an Enhancement Proposal or Bug Issue](https://github.com/argoproj/argo-cd/issues/new/choose)
first.

Each enhancement proposal needs to go through our
[triage process](#triage-process)
before we accept code contributions. To facilitate triage and to provide transparency, we use
[this GitHub project](https://github.com/orgs/argoproj/projects/18) to keep track of this process' outcome.

_Please_ do not spend too much time on larger features or refactorings before the corresponding enhancement has been triaged. This may save everyone some amount of frustration and time, as the enhancement proposal might be rejected, and the code would never get merged. However, sometimes it's helpful to have some PoC code along with a proposal.

We will do our best to triage incoming enhancement proposals quickly, with one of the following outcomes:

* Accepted
* Rejected
* Proposal requires a design document to be further discussed

Depending on how many enhancement proposals we receive at given times, it may take some time until we can look at yours.

Also, please make sure you have read our
[Toolchain Guide](toolchain-guide.md)
to understand our toolchain and our continuous integration processes. It contains some invaluable information to get started with the complex code base that makes up Argo CD.

## Quick start

If you want a quick start contributing to Argo CD, take a look at issues that are labeled with
[help wanted](https://github.com/argoproj/argo-cd/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22)
or
[good first issue](https://github.com/argoproj/argo-cd/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22).

These are issues that were already triaged and accepted.

If the issue is already attached to next
[version milestone](https://github.com/argoproj/argo-cd/milestones),
we have decided to also dedicate some of our time on reviews to PRs received for these issues.

We encourage our community to pick up issues that are labeled in this way *and* are attached to the next version's milestone, with a promise for them to get a proper review with the clear intention for the incoming PRs to get merged.

## Triage process

### Overview

Our triage process for enhancements proposals ensures that we take a look at all incoming enhancements to determine whether we will accept code submissions to implement them.

The process works as follows:

* New Enhancement Proposals raised on our GitHub issue tracker are moved to the _Incoming_ column of the project's board. These are the proposals that are in the queue for triage.
* The _Active_ column holds the issues that are currently being triaged, or will be triaged next.
* The _Accepted_ column holds the issues that have been triaged and are considered good to be implemented (e.g. the project agreed that the feature would be great to have)
* The _Declined_ column holds the issues that were rejected during triage. The issue will be updated with information about why the proposal has been rejected.
* The _Needs discussion_ column holds the issues that were found to require additional information, or even a design document, during triage.

### Triage cadence

Triage of enhancement proposals is performed transparently, offline using issue comments and online in our weekly contributor's meeting. _Everyone_ is invited to participate in triaging, the process is not limited to participation only by maintainers.

Usually, we will triage enhancement proposals in a First-In-First-Out order, which mean that oldest proposals will be triaged first.

We aim to triage at least 10 proposals a week. Depending on our available time, we may be triaging a higher or lower number of proposals in any given week.

## Proposal states

### Accepted proposals

When a proposal is considered _Accepted_, it was decided that this enhancement would be valuable to the community at large and fits into the overall strategic roadmap of the project.

Implementation of the issue may be started, either by the proposal's creator or another community member (including maintainers of the project).

The issue should be refined enough by now to contain any concerns and guidelines to be taken into consideration during implementation.

### Declined proposals

We don't decline proposals lightly, and we will do our best to give a proper reasoning why we think that the proposal does not fit with the future of the project. Reasons for declining proposals may be - amongst others - that the change would be breaking for many, or that it does not meet the strategic direction of the project. Usually, discussion will be facilitated with the enhancement's creator before declining a proposal.

Once a proposal is in _Declined_ state it's unlikely that we will accept code contributions for its implementation.

### Proposals that need discussion

Sometimes, we can't completely understand a proposal from its GitHub issue and require more information on the original intent or on more details about the implementation. If we are confronted with such an issue during the triage, we move this issue to the _Needs discussion_ column to indicate that we expect the issue's creator to supply more information on their idea. We may ask you to provide this information, either by adding that information to the issue itself or by joining one of our
[regular contributor's meeting](#regular-contributor-meeting)
to discuss the proposal with us.

Also, issues that we find to require a more formal design document will be moved to this column.

## Design documents

For some enhancement proposals (especially those that will change behavior of Argo CD substantially, are attached with some caveats or where upgrade/downgrade paths are not clear), a more formal design document will be required in order to fully discuss and understand the enhancement in the broader community. This requirement is usually determined during triage. If you submitted an enhancement proposal, we may ask you to provide this more formal write down, along with some concerns or topics that need to be addressed.

Design documents are usually submitted as PR and use [this template](https://github.com/argoproj/argo-cd/blob/master/docs/proposals/001-proposal-template.md) as a guide what kind of information we're looking for. Discussion will take place in the review process. When a design document gets merged, we consider it as approved and code can be written and submitted to implement this specific design.

## Regular contributor meeting

Our community regularly meets virtually to discuss issues, ideas and enhancements around Argo CD. We do invite you to join this virtual meetings if you want to bring up certain things (including your enhancement proposals), participate in our triaging or just want to get to know other contributors.

The current cadence of our meetings is weekly, every Thursday at 8:15AM Pacific Time ([click here to check in your current timezone][1]). We use Zoom to conduct these meetings.

* [Agenda document (Google Docs, includes Zoom link)](https://docs.google.com/document/d/1xkoFkVviB70YBzSEa4bDnu-rUZ1sIFtwKKG1Uw8XsY8)

If you want to discuss something, we kindly ask you to put your item on the
[agenda](https://docs.google.com/document/d/1xkoFkVviB70YBzSEa4bDnu-rUZ1sIFtwKKG1Uw8XsY8)
for one of the upcoming meetings so that we can plan in the time for discussing it.

[1]: https://www.timebie.com/std/pacific.php?q=081500
