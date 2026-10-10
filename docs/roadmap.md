# Roadmap

Argo CD does not publish a dated feature roadmap. Minor versions ship on a fixed quarterly schedule, and a feature
ships in the next minor release cut from `master` after it is merged. Larger features need a
[design document](developer-guide/code-contributions.md#design-documents) first.

The links below always show the current state of the project.

## Upcoming releases

Release dates, release champions and release checklists are listed in the
[Release Process and Cadence](developer-guide/release-process-and-cadence.md#schedule) document.

Items that maintainers have committed to review for a specific release are tracked in its
[GitHub milestone](https://github.com/argoproj/argo-cd/milestones). The feature freeze starts with the first release
candidate, seven weeks before general availability. Items that are not merged before the freeze should be moved to the
next release's milestone.

## Candidate breaking changes for v4.0

Breaking changes that cannot ship in a 3.x release are collected in the
[v4.0 milestone](https://github.com/argoproj/argo-cd/milestone/35). Some of them are decided and some are still open
questions. Review the milestone if you want to know which deprecated fields, flags and defaults may change.

## Features under design

Enhancements that are complex enough to require a design document are labelled
[`proposal:required`](https://github.com/argoproj/argo-cd/issues?q=is%3Aissue%20state%3Aopen%20label%3Aproposal%3Arequired).
Design documents are discussed in
[open proposal pull requests](https://github.com/argoproj/argo-cd/pulls?q=is%3Apr%20is%3Aopen%20proposal%20in%3Atitle),
and accepted ones are stored in the
[`docs/proposals`](https://github.com/argoproj/argo-cd/tree/master/docs/proposals) directory.

> [!NOTE]
> An accepted proposal is not a commitment to a release date. It is implemented when a contributor picks it up.

## Most requested enhancements

See the
[most-upvoted open enhancement requests](https://github.com/argoproj/argo-cd/issues?q=is%3Aissue%20state%3Aopen%20label%3Aenhancement%20sort%3Areactions-%2B1-desc).
Add a 👍 reaction to an issue to show that you need it. Avoid "+1" comments, since they notify every subscriber
without adding information.

## Released features

Each minor release is announced on the [Argo Project blog](https://blog.argoproj.io/) and in the
[GitHub releases](https://github.com/argoproj/argo-cd/releases).

## Influencing the direction of the project

To propose a new feature, open an
[enhancement proposal](https://github.com/argoproj/argo-cd/issues/new?template=enhancement_proposal.md).
To discuss priorities with the maintainers, add an item to the agenda of the
[contributor meeting](developer-guide/code-contributions.md#regular-contributor-meeting).
