# Reviewing PRs

This guide is for anyone who reviews pull requests to Argo CD. It defines when a review may block a PR, what a blocking review must contain, and when the reviewer need to re-review.
If you are submitting a PR, read [Submit Your PR](submit-your-pr.md) first; the sections below on blocking reviews tell you what to expect from reviewers.

## Summary

| Situation                                                                 | Review type                                  | Required from the reviewer                                                |
|---------------------------------------------------------------------------|----------------------------------------------|---------------------------------------------------------------------------|
| Bug, regression, security issue, data loss, or undocumented breaking change | Request changes                          | Reason and exit condition on each blocking thread                         |
| Design disagreement                                                       | Request changes                          | Reason, exit condition, and a concrete alternative or violated constraint |
| Missing tests or docs                                                     | Comment                                  | Flag it; approvers do not merge until it is fixed                         |
| Anything else: style, naming, refactoring ideas, questions, optional work | Comment (or Approve with comments)   | Prefix with `nit:`, `suggestion:` or `question:`                          |

The above table doesn't mean that all bugs or all cases should request a change. The reviewer should use their best judgement to decide whether a change is necessary or not.
A reviewer who requests changes must re-review within 5 business days of the author asking for it. After that, any approver may dismiss the review.

## Who can review

Anyone can review a PR, and reviews from the community are welcome. The roles below are defined in the [Argo community membership guide](https://github.com/argoproj/argoproj/blob/main/community/membership.md).

* Members and other contributors may leave Comment reviews. If you find a problem that meets the blocking criteria below, say so in your comment and a reviewer or approver can turn it into a blocking review.
* Reviewers and approvers may submit Request changes reviews, following the rules in this guide.
* Approvers (the `@argoproj/argocd-approvers` team and the scoped teams in [`CODEOWNERS`](https://github.com/argoproj/argo-cd/blob/master/CODEOWNERS)) merge PRs. Every PR needs at least one approver.

## Review in order

Check a PR in this order and stop at the first level that fails.

1. Is the change wanted? Bug fixes need a reproducible bug. Features need an accepted enhancement proposal (see the [code contribution guide](code-contributions.md)).
2. Is the design right? Does it fit the architecture, respect existing APIs and behavior, and match the accepted proposal?
3. Is the code right? Correctness, error handling, concurrency, security, performance, tests and docs.
4. Is it polished? Naming, readability, comments, small simplifications.

## When to request changes

A Request changes review blocks a PR. Use it only for the three reasons below. Everything else is a comment.

### Allowed reasons

1. Bug, security issue, or data loss. The change is incorrect, introduces a regression, opens a security hole, or could lose or corrupt user state (Applications, clusters, repositories, secrets, settings).
2. Undocumented breaking change. The change alters user-visible behavior, an API, a CLI flag, a manifest, or a default, and the PR does not document it in the relevant docs and in the upgrade notes under [`docs/operator-manual/upgrading/`](../operator-manual/upgrading/overview.md).
3. Design disagreement. The approach conflicts with Argo CD's architecture or with an accepted proposal, even if the code works. This reason has extra conditions, described in [Design disagreements](#design-disagreements).

### Not reasons to block

These are always comments, never blocking reviews:

* Style or formatting that the linters do not enforce
* Naming preferences
* Refactoring outside the scope of the PR
* "I would have done it differently" when the PR's approach is not wrong
* Follow-up work that can go in a separate PR
* Missing tests or docs (see [Missing tests and docs](#missing-tests-and-docs))

> [!NOTE]
> If you are not sure, just leave a comment instead. Another reviewer or approver can still block on it if they disagree.

### What a blocking thread must contain

Every thread that blocks the PR must state:

* The reason, as a prefix: `blocking (bug):`, `blocking (breaking):` or `blocking (design):`.
* The exit condition: what the author must change, concretely enough that both of you will agree when it is done.

For example:

```text
blocking (bug): when `spec.source` is nil this dereferences it and panics the
controller. Exit condition: return an error for a nil source, and add a test case
for it in TestReconcile.
```

Approvers treat a Request changes review as a comment review if it has no blocking threads, or if its threads do not follow this format.

### Design disagreements

Design is the reason most open to judgement, so a design block also needs:

* A concrete alternative, or the constraint the PR violates, such as a section of an accepted proposal, an API compatibility rule, or a documented architecture decision. "I don't like this approach" is not enough.
* Escalation after one round. If the author responds and you still disagree, do not keep going back and forth. Either side adds the PR to the agenda of the next [contributor meeting](code-contributions.md#regular-contributor-meeting). The meeting's decision is recorded as a comment on the PR, and both sides follow it.

Design concerns are cheapest to raise before code is written, while an enhancement proposal is being discussed.

## Missing tests and docs

Tests and docs are a merge requirement, not a reason to block. Every fix needs a test that fails without it, and every feature needs tests and docs.

* Reviewers flag what is missing in a comment.
* Approvers do not merge a PR until the tests and docs are in place, or the PR description explains why tests are not feasible.

## Non-blocking feedback

Submit non-blocking feedback as a Comment review, or as Approve if nothing in your review blocks the PR. Prefix each thread so the author knows what you expect:

| Prefix        | Meaning                                                       | Author may resolve without changing code |
|---------------|---------------------------------------------------------------|------------------------------------------|
| `nit:`        | A minor issue such as a typo, naming, or formatting           | Yes                                      |
| `suggestion:` | An improvement the author can take or leave                   | Yes, ideally with a short reply          |
| `question:`   | You want to understand something; no change is implied        | After answering it                       |

Approving with `nit:` or `suggestion:` comments is encouraged. It tells the author that the PR can merge as is and that the comments are optional.

## Re-review commitment

Submitting a Request changes review is a commitment to come back.

* When the author has addressed your blocking threads, they re-request your review using GitHub's Re-request review button, or by mentioning you in a comment. You then have 5 business days to re-review.
* Check your existing blocking threads against their exit conditions. You may raise a new blocking issue only if it is in code changed since your last review, or if it falls under the first [allowed reason](#allowed-reasons) and you missed it the first time. Do not move the goalposts.
* If you cannot make it, say so on the PR before the window ends, and either hand the review to another reviewer by name or dismiss your own review.
* When your comments and your concerns are addressed, you are encouraged to approve the PR, or dismiss your review if someone else will approve it.

### When the re-review is not done within 5 business days

If 5 business days have passed since the author re-requested review and the reviewer has not responded:

1. The author mentions any approver on the PR, or asks in the `#argo-cd-contributors` channel on [Slack](https://argoproj.github.io/community/join-slack).
2. The approver checks that the author responded to every blocking thread, then dismisses the stale review with a comment that links the author's response and notes that the re-review window has passed.
3. The PR then follows the normal review and approval process.

Dismissing a review because the window passed is not a decision on its content. The original reviewer may submit a new Request changes review later, following the same rules.

> [!WARNING]
> Approvers must not merge a PR while a valid Request changes review is still in place. Dismiss it through the process above first.

## Reviewer checklist for Argo CD

Besides general code quality, check the following for every PR:

* Upgrade impact. Changed defaults, removed fields, renamed flags, and changed behavior are documented in the upgrade notes for the next release.
* RBAC and security. New API endpoints enforce RBAC. New settings that touch credentials, TLS, or network access are secure by default.
* Backports. Bug fixes that should go to supported release branches get a `cherry-pick/x.y` label (see [Backport Your Fix](backporting-fixes.md)).
* Docs. User-facing changes are documented under `docs/`.
