# Release Cycle

## Minor releases (e.g. 2.x.0)

A minor Argo CD release occurs four times a year, once every three months. Each General Availability (GA) release is 
preceded by several Release Candidates (RCs). The first RC is released three weeks before the scheduled GA date. This 
effectively means that there is a three-week feature freeze.

These are the approximate release dates:

* The first Monday of January
* The first Monday of April
* The first Monday of July
* The first Monday of October

Dates may be shifted slightly to accommodate holidays. Those shifts should be minimal.

### Release Planning Meeting

Two weeks before the RC date, there will be a meeting to discuss which features are planned for the RC. This meeting is
for contributors to advocate for certain features. Features which have at least one approver (besides the contributor) 
who can assure they will review/merge by the RC date will be included in the release milestone. All other features will
be dropped from the milestone (and potentially shifted to the next one).

Since not everyone will be able to attend the meeting, there will be a meeting doc. Contributors can add their feature
to a table, and approvers can add their name to the table. Features with a corresponding approver will remain in the 
release milestone.

### Release Champion

To help manage all the steps involved in a release, we will have a release champion. The release champion will be
responsible for a checklist of items for their release. The checklist will be an issue template in a new 
argoproj/releases repository.

## Patch releases (e.g. 2.5.x)

Argo CD patch releases occur on an as-needed basis. Only the three most recent minor versions are eligible for patch 
releases. Versions older than the three most recent minor versions are considered EOL and will not receive bug fixes or 
security updates.

## Schedule

These are the upcoming releases dates:

| Release | Release Planning Meeting | Release Candidate 1   | General Availability    | Release Champion | Checklist                                                 |
|---------|--------------------------|-----------------------|-------------------------|------------------|-----------------------------------------------------------|
| v2.6    | Monday, Nov. 28, 2022    | Monday, Dec. 12, 2022 | Tuesday, Jan. 2, 2023   |                  | [checklist](https:/github.com/argoproj/releases/issues/1) |
| v2.7    | Monday, Feb. 27, 2023    | Monday, Mar. 13, 2023 | Monday, Apr. 3, 2023    |
| v2.8    | Monday, May 29, 2023     | Monday, Jun. 12, 2023 | Wednesday, Jul. 5, 2023 |
| v2.9    | Monday, Aug. 28, 2023    | Monday, Sep. 11, 2023 | Monday, Oct. 2, 2023    |

## Feature Acceptance Criteria

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
