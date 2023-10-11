---
name: Argo CD Release
about: Used by our Release Champion to track progress of a minor release
title: 'Argo CD Release vX.X'
labels: 'release'
assignees: ''
---

Target RC1 date: ___. __, ____
Target GA date: ___. __, ____

 - [ ] Create new section in the [Release Planning doc](https://docs.google.com/document/d/1trJIomcgXcfvLw0aYnERrFWfPjQOfYMDJOCh1S8nMBc/edit?usp=sharing)
 - [ ] Schedule a Release Planning meeting roughly two weeks before the scheduled Release freeze date by adding it to the community calendar (or delegate this task to someone with write access to the community calendar)
     - [ ] Include Zoom link in the invite
 - [ ] Post in #argo-cd and #argo-contributors one week before the meeting
 - [ ] Post again one hour before the meeting
 - [ ] At the meeting, remove issues/PRs from the project's column for that release which have not been “claimed” by at least one Approver (add it to the next column if Approver requests that)
 - [ ] 1wk before feature freeze post in #argo-contributors that PRs must be merged by DD-MM-YYYY to be included in the release - ask approvers to drop items from milestone they can’t merge
 - [ ] At least two days before RC1 date, draft RC blog post and submit it for review (or delegate this task)
 - [ ] Cut RC1 (or delegate this task to an Approver and coordinate timing)
 - [ ] Create new release branch
    - [ ] Add the release branch to ReadTheDocs
    - [ ] Confirm that tweet and blog post are ready
    - [ ] Trigger the release
    - [ ] After the release is finished, publish tweet and blog post
    - [ ] Post in #argo-cd and #argo-announcements with lots of emojis announcing the release and requesting help testing
 - [ ] Monitor support channels for issues, cherry-picking bugfixes and docs fixes as appropriate (or delegate this task to an Approver and coordinate timing)
 - [ ] At release date, evaluate if any bugs justify delaying the release. If not, cut the release (or delegate this task to an Approver and coordinate timing)
 - [ ] If unreleased changes are on the release branch for {current minor version minus 3}, cut a final patch release for that series (or delegate this task to an Approver and coordinate timing)
 - [ ] After the release, post in #argo-cd that the {current minor version minus 3} has reached EOL (example: https://cloud-native.slack.com/archives/C01TSERG0KZ/p1667336234059729)
 - [ ] (For the next release champion) Review the [items scheduled for the next release](https://github.com/orgs/argoproj/projects/25). If any item does not have an assignee who can commit to finish the feature, move it to the next release.
 - [ ] (For the next release champion) Schedule a time mid-way through the release cycle to review items again.