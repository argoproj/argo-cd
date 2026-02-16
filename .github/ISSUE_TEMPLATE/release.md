---
name: Argo CD Release
about: Used by our Release Champion to track progress of a minor release
title: 'Argo CD Release vX.X'
labels: 'release'
assignees: ''
---

Target RC1 date: ___. __, ____
Target GA date: ___. __, ____

## RC1 Release Checklist

 - [ ] 1wk before feature freeze post in #argo-contributors that PRs must be merged by DD-MM-YYYY to be included in the release - ask approvers to drop items from milestone they can't merge
 - [ ] At least two days before RC1 date, draft RC blog post and submit it for review (or delegate this task)
 - [ ] Create new release branch (or delegate this task to an Approver)
    - [ ] Add the release branch to ReadTheDocs
 - [ ] Cut RC1 (or delegate this task to an Approver and coordinate timing)
    - [ ] Run the [Init ArgoCD Release workflow](https://github.com/argoproj/argo-cd/actions/workflows/init-release.yaml) from the release branch
    - [ ] Review and merge the generated version bump PR
    - [ ] Run `./hack/trigger-release.sh` to push the release tag
    - [ ] Monitor the [Publish ArgoCD Release workflow](https://github.com/argoproj/argo-cd/actions/workflows/release.yaml)
    - [ ] Verify the release on [GitHub releases](https://github.com/argoproj/argo-cd/releases)
    - [ ] Verify the container image on [Quay.io](https://quay.io/repository/argoproj/argocd?tab=tags)
    - [ ] Confirm the new version appears in [Read the Docs](https://argo-cd.readthedocs.io/)
    - [ ] Verify the docs release build in https://app.readthedocs.org/projects/argo-cd/ succeeded and retry if failed (requires an Approver with admin creds to readthedocs)
 - [ ] Announce RC1 release
   - [ ] Confirm that tweet and blog post are ready
   - [ ] Publish tweet and blog post
   - [ ] Post in #argo-cd and #argo-announcements requesting help testing:
     ```
     :mega: Argo CD v{MAJOR}.{MINOR}.{PATCH}-rc{RC_NUMBER} is OUT NOW! :argocd::tada:
     
     Please go through the following resources to know more about the release:
     
     Release notes: https://github.com/argoproj/argo-cd/releases/tag/v{VERSION}
     Blog: {BLOG_POST_URL}
     
     We'd love your help testing this release candidate! Please try it out in your environments and report any issues you find. This helps us ensure a stable GA release.
     
     Thanks to all the folks who spent their time contributing to this release in any way possible!
     ```
 - [ ] Monitor support channels for issues, cherry-picking bugfixes and docs fixes as appropriate during the RC period (or delegate this task to an Approver and coordinate timing)
 - [ ] After creating the RC, open a documentation PR for the next minor version using [this](../../docs/operator-manual/templates/minor_version_upgrade.md) template.

## GA Release Checklist

 - [ ] At GA release date, evaluate if any bugs justify delaying the release
 - [ ] Prepare for EOL version (version that is 3 releases old)
    - [ ] If unreleased changes are on the release branch for {current minor version minus 3}, cut a final patch release for that series (or delegate this task to an Approver and coordinate timing)
    - [ ] Edit the final patch release on GitHub and add the following notice at the top:
     ```markdown
     > [!IMPORTANT]
     > **END OF LIFE NOTICE**
     > 
     > This is the final release of the {EOL_SERIES} release series. As of {GA_DATE}, this version has reached end of life and will no longer receive bug fixes or security updates.
     > 
     > **Action Required**: Please upgrade to a [supported version](https://argo-cd.readthedocs.io/en/stable/operator-manual/upgrading/overview/) (v{SUPPORTED_VERSION_1}, v{SUPPORTED_VERSION_2}, or v{NEW_VERSION}).
     ```
 - [ ] Cut GA release (or delegate this task to an Approver and coordinate timing)
    - [ ] Run the [Init ArgoCD Release workflow](https://github.com/argoproj/argo-cd/actions/workflows/init-release.yaml) from the release branch
    - [ ] Review and merge the generated version bump PR
    - [ ] Run `./hack/trigger-release.sh` to push the release tag
    - [ ] Monitor the [Publish ArgoCD Release workflow](https://github.com/argoproj/argo-cd/actions/workflows/release.yaml)
    - [ ] Verify the release on [GitHub releases](https://github.com/argoproj/argo-cd/releases)
    - [ ] Verify the container image on [Quay.io](https://quay.io/repository/argoproj/argocd?tab=tags)
    - [ ] Verify the `stable` tag has been updated
    - [ ] Confirm the new version appears in [Read the Docs](https://argo-cd.readthedocs.io/)
    - [ ] Verify the docs release build in https://app.readthedocs.org/projects/argo-cd/ succeeded and retry if failed (requires an Approver with admin creds to readthedocs)
 - [ ] Announce GA release with EOL notice
   - [ ] Confirm that tweet and blog post are ready
   - [ ] Publish tweet and blog post
   - [ ] Post in #argo-cd and #argo-announcements announcing the release and EOL:
     ```
     :mega: Argo CD v{MAJOR}.{MINOR} is OUT NOW! :argocd::tada:
     
     Please go through the following resources to know more about the release:
     
     Upgrade instructions: https://argo-cd.readthedocs.io/en/latest/operator-manual/upgrading/{PREV_MINOR}-{MAJOR}.{MINOR}/
     Blog: {BLOG_POST_URL}
     
     :warning: IMPORTANT: With the release of Argo CD v{MAJOR}.{MINOR}, support for Argo CD v{EOL_VERSION} has officially reached End of Life (EOL).
     
     Thanks to all the folks who spent their time contributing to this release in any way possible!
     ```
 - [ ] (For the next release champion) Review the [items scheduled for the next release](https://github.com/orgs/argoproj/projects/25). If any item does not have an assignee who can commit to finish the feature, move it to the next release.
 - [ ] (For the next release champion) Schedule a time mid-way through the release cycle to review items again.
