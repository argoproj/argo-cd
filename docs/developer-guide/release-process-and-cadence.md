# Release Process And Cadence

## Release Cycle

### Schedule

These are the upcoming releases dates:

| Release | Release Planning Meeting | Release Candidate 1   | General Availability | Release Champion                                      | Checklist                                                     |
|---------|--------------------------|-----------------------|----------------------|-------------------------------------------------------|---------------------------------------------------------------|
| v2.6    | Monday, Dec. 12, 2022    | Monday, Dec. 19, 2022 | Monday, Feb. 6, 2023 | [William Tam](https://github.com/wtam2018)            | [checklist](https://github.com/argoproj/argo-cd/issues/11563) |
| v2.7    | Monday, Mar. 6, 2023     | Monday, Mar. 20, 2023 | Monday, May. 1, 2023 | [Pavel Kostohrys](https://github.com/pasha-codefresh) | [checklist](https://github.com/argoproj/argo-cd/issues/12762) |
| v2.8    | Monday, Jun. 20, 2023    | Monday, Jun. 26, 2023 | Monday, Aug. 7, 2023 | [Keith Chong](https://github.com/keithchong)          | [checklist](https://github.com/argoproj/argo-cd/issues/13742) |
| v2.9    | Monday, Sep. 4, 2023     | Monday, Sep. 18, 2023 | Monday, Nov. 6, 2023 | [Leonardo Almeida](https://github.com/leoluz)         | [checklist](https://github.com/argoproj/argo-cd/issues/14078) |
| v2.10   | Monday, Dec. 4, 2023     | Monday, Dec. 18, 2023 | Monday, Feb. 5, 2024 | 


Actual release dates might differ from the plan by a few days.

### Release Process

#### Minor Releases (e.g. 2.x.0)

A minor Argo CD release occurs four times a year, once every three months. Each General Availability (GA) release is
preceded by several Release Candidates (RCs). The first RC is released three weeks before the scheduled GA date. This
effectively means that there is a three-week feature freeze.

These are the approximate release dates:

* The first Monday of February
* The first Monday of May
* The first Monday of August
* The first Monday of November

Dates may be shifted slightly to accommodate holidays. Those shifts should be minimal.

#### Patch Releases (e.g. 2.5.x)

Argo CD patch releases occur on an as-needed basis. Only the three most recent minor versions are eligible for patch
releases. Versions older than the three most recent minor versions are considered EOL and will not receive bug fixes or
security updates.

#### Minor Release Planning Meeting

Roughly two weeks before the RC date, there will be a meeting to discuss which features are planned for the RC. This meeting is
for contributors to advocate for certain features. Features which have at least one approver (besides the contributor)
who can assure they will review/merge by the RC date will be included in the release milestone. All other features will
be dropped from the milestone (and potentially shifted to the next one).

Since not everyone will be able to attend the meeting, there will be a meeting doc. Contributors can add their feature
to a table, and Approvers can add their name to the table. Features with a corresponding approver will remain in the
release milestone.

#### Release Champion

To help manage all the steps involved in a release, we will have a Release Champion. The Release Champion will be
responsible for a checklist of items for their release. The checklist is an issue template in the Argo CD repository.

The Release Champion can be anyone in the Argo CD community. Some tasks (like cherry-picking bug fixes and cutting
releases) require [Approver](https://github.com/argoproj/argoproj/blob/master/community/membership.md#community-membership)
membership. The Release Champion can delegate tasks when necessary and will be responsible for coordinating with the
Approver.

### Feature Acceptance Criteria

To be eligible for inclusion in a minor release, a new feature must meet the following criteria before the release’s RC
date.

If it is a large feature that involves significant design decisions, that feature must be described in a Proposal, and
that Proposal must be reviewed and merged.

The feature PR must include:

* Tests (passing)
* Documentation
* If necessary, a note in the Upgrading docs for the planned minor release
* The PR must be reviewed, approved, and merged by an Approver.

If these criteria are not met by the RC date, the feature will be ineligible for inclusion in the RC series or GA for
that minor release. It will have to wait for the next minor release.
