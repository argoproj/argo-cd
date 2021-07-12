# Release Process And Cadence

Argo CD is being developed using the following process:

* Maintainers commit to work on set of features and enhancements and create GitHub milestone to track the work.
* We are trying to avoid delaying release and prefer moving the feature into the next release if we cannot complete it on time.
* The new release is published every **3 month**.
* Critical bug-fixes are cherry-picked into the release branch and delivered using patch releases as frequently as needed.

## Release Planning

We are using GitHub milestones to perform release planning and tracking. Each release milestone includes two type of issues:

* Issues that maintainers committed to working on. Maintainers decide which features they are committing to work on during the next release based on
  their availability. Typically issues added offline by each maintainer and finalized during the contributors' meeting. Each such issue should be
  assigned to maintainer who plans to implement and test it.
* Nice to have improvements contributed by community contributors. Nice to have issues are typically not critical, smallish enhancements that could
  be contributed by community contributors. Maintainers are not committing to implement them but committing to review PR from the community.

The milestone should have a clear description of the most important features as well as the expected end date. This should provide clarity to end-users
about what to expect from the next release and when.

In addition to the next milestone, we need to maintain a draft of the upcoming release milestone. 

## Community Contributions

We receive a lot of contributions from our awesome community, and we're very grateful for that fact. However, reviewing and testing PRs is a lot of (unplanned) work and therefore, we cannot guarantee that contributions (especially large or complex ones) made by the community receive a timely review within a release's time frame. Maintainers may decide on their own to put work on a PR together with the contributor and in this case, the maintainer will self-assigned the PR and thereby committing to review, eventually merge and later test it on the release scope.

## Release Testing

We need to make sure that each change, both from maintainers and community contributors, is tested well and have someone who is going to fix last-minute
bugs. In order to ensure it, each merged pull request must have an assigned maintainer before it gets merged. The assigned maintainer will be working on
testing the introduced changes and fixing of any introduced bugs.

We have a code freeze period two weeks before the release until the release branch is created. During code freeze no feature PR should be merged and it is ok
to merge bug fixes.

Maintainers assigned to a PR that's been merged should drive testing and work on fixing last-minute issues. For tracking purposes after verifying PR the assigned
the maintainer should label it with a `verified` label.

## Releasing

The releasing procedure is described in [releasing](./releasing.md) document. Before closing the release milestone following should be verified:

- [ ] All merged PRs have the `verified` label
- [ ] Roadmap is updated based one current release changes
- [ ] Next release milestone is created
- [ ] Upcoming release milestone is updated