# Release Process And Cadence

## Release Cycle

### Schedule

These are the upcoming releases dates:

| Release | Release Candidate 1   | General Availability | Release Champion                                      | Release Approver                                      |Checklist                                                      |
|---------|-----------------------|----------------------|-------------------------------------------------------|-------------------------------------------------------|---------------------------------------------------------------|
| v2.6    | Monday, Dec. 19, 2022 | Monday, Feb. 6, 2023 | [William Tam](https://github.com/wtam2018)            | [William Tam](https://github.com/wtam2018)            | [checklist](https://github.com/argoproj/argo-cd/issues/11563) |
| v2.7    | Monday, Mar. 20, 2023 | Monday, May 1, 2023  | [Pavel Kostohrys](https://github.com/pasha-codefresh) | [Pavel Kostohrys](https://github.com/pasha-codefresh) | [checklist](https://github.com/argoproj/argo-cd/issues/12762) |
| v2.8    | Monday, Jun. 26, 2023 | Monday, Aug. 7, 2023 | [Keith Chong](https://github.com/keithchong)          | [Keith Chong](https://github.com/keithchong)          | [checklist](https://github.com/argoproj/argo-cd/issues/13742) |
| v2.9    | Monday, Sep. 18, 2023 | Monday, Nov. 6, 2023 | [Leonardo Almeida](https://github.com/leoluz)         | [Leonardo Almeida](https://github.com/leoluz)         | [checklist](https://github.com/argoproj/argo-cd/issues/14078) |
| v2.10   | Monday, Dec. 18, 2023 | Monday, Feb. 5, 2024 | [Katie Lamkin](https://github.com/kmlamkin9)          |                                                       | [checklist](https://github.com/argoproj/argo-cd/issues/16339) |
| v2.11   | Friday, Apr. 5,  2024 | Monday, May 6, 2024  | [Pavel Kostohrys](https://github.com/pasha-codefresh) | [Pavel Kostohrys](https://github.com/pasha-codefresh) | [checklist](https://github.com/argoproj/argo-cd/issues/17726) |
| v2.12   | Monday, Jun. 17, 2024 | Monday, Aug. 5, 2024 | [Ishita Sequeira](https://github.com/ishitasequeira) | [Pavel Kostohrys](https://github.com/pasha-codefresh) | [checklist](https://github.com/argoproj/argo-cd/issues/19063) |
| v2.13   | Monday, Sep. 16, 2024 | Monday, Nov. 4, 2024 | [Regina Voloshin](https://github.com/reggie-k)        | [Pavel Kostohrys](https://github.com/pasha-codefresh) | [checklist](https://github.com/argoproj/argo-cd/issues/19513) |
| v2.14   | Monday, Dec. 16, 2024 | Monday, Feb. 3, 2025 | | | |
| v2.15   | Monday, Mar. 17, 2025 | Monday, May 5, 2025 | | | |

Actual release dates might differ from the plan by a few days.

### Release Process

#### Minor Releases (e.g. 2.x.0)

A minor Argo CD release occurs four times a year, once every three months. Each General Availability (GA) release is
preceded by several Release Candidates (RCs). The first RC is released seven weeks before the scheduled GA date. This
effectively means that there is a seven-week feature freeze.

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

### Security Patch Policy

CVEs in Argo CD code will be patched for all supported versions. Read more about supported versions in the [security policy for Argo CD](https://github.com/argoproj/argo-cd/security/policy#supported-versions).

### Dependencies Lifecycle Policy

Dependencies are evaluated before being introduced to ensure they:

1) are actively maintained
2) are maintained by trustworthy maintainers

These evaluations vary from dependency to dependencies.

Dependencies are also scheduled for removal if the project has been deprecated or if the project is no longer maintained.

CVEs in dependencies will be patched for all supported versions if the CVE is applicable and is assessed by Snyk to be
of high or critical severity. Automation generates a [new Snyk scan weekly](../snyk).
