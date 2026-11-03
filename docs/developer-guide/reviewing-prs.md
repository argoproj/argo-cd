# Reviewing PRs

This guide is for anyone who reviews pull requests to Argo CD. It is built around three principles that make reviews easier for contributors to follow and that fit into the way most of us already review, so following them should not cost reviewers extra time. None of them comes with a deadline, because maintainers review in whatever time they have available, and that time changes from week to week.

If you are submitting a PR, read [Submit Your PR](submit-your-pr.md) first, and then use this guide to understand what to expect from the people reviewing it.

## Who reviews

Anyone can review a PR, and reviews from the community are welcome. Approvers (the `@argoproj/argocd-approvers` team and the scoped teams in [`CODEOWNERS`](https://github.com/argoproj/argo-cd/blob/master/CODEOWNERS)) merge PRs, and every PR needs at least one approval from one of them, with the roles themselves defined in the [Argo community membership guide](https://github.com/argoproj/argoproj/blob/main/community/membership.md).

## 1. Be clear about what is a blocker and what is not

When something must be fixed before the PR can merge, say so in the comment and describe what would resolve it, and when something is only a nit, say that as well. That way the contributor knows what to work on first, and anyone who reviews the PR later can tell which of the earlier requests were optional.

These are the kinds of problems that usually block a PR:

* A bug, a regression, a security problem, or anything that could lose or corrupt user data
* Missing tests, unless the PR description explains why tests are not feasible
* Docs that are missing, wrong, or misleading, including missing upgrade notes for a change that affects existing installations
* An approach you disagree with on design grounds, in which case name the alternative you have in mind or the constraint the change breaks, so the author has a concrete direction to work from

Mark everything else with one of these prefixes, so the author knows it is optional:

| Prefix        | Meaning                                                |
|---------------|--------------------------------------------------------|
| `nit:`        | A minor issue such as a typo, naming, or formatting    |
| `suggestion:` | An improvement the author can take or leave            |
| `question:`   | You want to understand something; no change is implied |

If at least one of your comments is a blocker, submit the review as Request changes, and if none of them are, submit it as Comment or Approve instead.

## 2. Trust your judgment when dismissing reviews

An earlier Request changes review sometimes stays on a PR after the author has responded to it, and if you are the approver looking at that PR, use your judgment along these lines:

* If the open comments are only nits, or minor points the author has clearly addressed, treat them as non-blocking and dismiss the review with a comment that explains why.
* If an open comment raises something important, such as a security concern or a design disagreement, check with the original reviewer before dismissing the review, even when the author says the comment has been addressed.
* If you can't resolve it with them, or they haven't replied after a reminder on the PR, ask the other maintainers to decide whether the review can be dismissed, in the `#argo-cd-contributors` channel on [Slack](https://argoproj.github.io/community/join-slack) or at the [contributor meeting](code-contributions.md#regular-contributor-meeting).

If you blocked a PR and know you won't get back to it, say so on the PR so someone else can pick it up.

## 3. Always communicate expectations and next steps

Before you post a comment or submit a review, check that the contributor will be able to tell what happens next, as in these examples:

* You approve but are not an approver: "Looks good to me. It still needs a review from an approver." If you are an Argo member or reviewer, also add the `ready-for-review` label and add the PR to the Argo CD Review project (see the [FAQ](faq.md#how-do-i-get-my-pr-labeled-ready-for-review)).
* You are an approver waiting on another maintainer: "Approved from my side. This now waits for @name to finish their review."
* You requested changes: your blocking comments already say what would resolve each of them.

If the author asks for another look and you can't get to it soon, a 👀 reaction or a short reply lets them know you've seen it.

## Reviewer checklist for Argo CD

Besides general code quality, check the following for every PR:

* Upgrade impact: changed defaults, removed fields, renamed flags, and changed behavior are documented in the upgrade notes for the next release.
* RBAC and security: new API endpoints enforce RBAC, and new settings that touch credentials, TLS, or network access are secure by default.
* Backports: bug fixes that should go to supported release branches get a `cherry-pick/x.y` label (see [Backport Your Fix](backporting-fixes.md)).
* Docs: user-facing changes are documented under `docs/`.
