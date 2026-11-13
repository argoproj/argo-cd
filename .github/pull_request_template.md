<!--
Pull requests must link to an issue, and enhancements must be labeled `triage/accepted`. Pull requests that do not are
converted to drafts. See https://argo-cd.readthedocs.io/en/latest/developer-guide/submit-your-pr/#linking-your-pr-to-an-issue
for how to link an issue and for exemptions.

Note on DCO:

If the DCO action in the integration test fails, one or more of your commits are not signed off. Please click on the *Details* link next to the DCO action for instructions on how to resolve this.
-->

## Description

Closes #<!-- issue number; use "Part of #" if this PR does not fully resolve the issue, or delete this line if your PR is exempt -->

<!-- Describe what this PR does and why it is necessary. -->

## Checklist

### Eligibility

<!-- Check one. -->

* [ ] Links to an issue. If the issue is an enhancement, it has the `triage/accepted` label.
* [ ] Is exempt: it changes fewer than 50 lines outside of documentation and generated files, or it is a `chore:`.

### Quality

* [ ] The title of the PR states what changed and the related issues number (used for the release note).
* [ ] The title of the PR conforms to the [Title of the PR](https://argo-cd.readthedocs.io/en/latest/developer-guide/submit-your-pr/#title-of-the-pr)
* [ ] I've updated both the CLI and UI to expose my feature, or I plan to submit a second PR with them.
* [ ] Does this PR require documentation updates?
* [ ] I've updated documentation as required by this PR.
* [ ] I have signed off all my commits as required by [DCO](https://github.com/argoproj/argoproj/blob/master/community/CONTRIBUTING.md#legal)
* [ ] I have written unit and/or e2e tests for my change. PRs without these are unlikely to be merged.
* [ ] My build is green ([troubleshooting builds](https://argo-cd.readthedocs.io/en/latest/developer-guide/ci/)).
* [ ] My new feature complies with the [feature status](https://github.com/argoproj/argoproj/blob/master/community/feature-status.md) guidelines.
* [ ] Optional. My organization is added to USERS.md.
* [ ] Optional. For bug fixes, I've indicated what older releases this fix should be cherry-picked into (this may or may not happen depending on risk/complexity).

<!-- Please see [Contribution FAQs](https://argo-cd.readthedocs.io/en/latest/developer-guide/faq/) if you have questions about your pull-request. -->
