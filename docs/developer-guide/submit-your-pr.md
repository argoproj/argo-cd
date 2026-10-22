# Submitting PRs

## Prerequisites

1. [Development Environment](development-environment.md)
2. [Toolchain Guide](toolchain-guide.md)
3. [Development Cycle](development-cycle.md)

## Preface

> [!NOTE]
> **Before you start**
>
> The Argo CD project continuously grows, both in terms of features and community size. It gets adopted by more and more organizations which entrust Argo CD to handle their critical production workloads. Thus, we need to take great care with any changes that affect compatibility, performance, scalability, stability and security of Argo CD. For this reason, every new feature or larger enhancement must be properly designed and discussed before it gets accepted into the codebase.
>
> We do welcome and encourage everyone to participate in the Argo CD project, but please understand that we can't accept each and every contribution from the community, for various reasons. If you want to submit code for a great new feature or enhancement, we kindly ask you to take a look at the
> [code contribution guide](code-contributions.md#) before you start to write code or submit a PR.

If you want to submit a PR, please read this document carefully, as it contains important information guiding you through our PR quality gates.

If you need guidance with submitting a PR, or have any other questions regarding development of Argo CD, do not hesitate to [join our Slack](https://argoproj.github.io/community/join-slack) and get in touch with us in the `#argo-cd-contributors` channel!

## Linking your PR to an issue

Argo CD follows an issue-first contribution model. Before you write code, make sure there is an issue that describes the
problem, so that maintainers can agree on the problem and the approach before effort goes into a review.

1. Find an existing issue, or [open a new one](https://github.com/argoproj/argo-cd/issues/new/choose).
2. If the issue is an enhancement (labeled `enhancement`), wait until a maintainer has accepted it by adding the
   `triage/accepted` label. See the [code contribution guide](code-contributions.md#triage-process) for how enhancements
   are triaged. Bug reports do not need to be accepted.
3. Open your PR and link the issue in the PR description.

A PR is linked to an issue when any of the following is true:

* The description contains a linking phrase followed by the issue, for example `Closes #1234`. The supported phrases are
  `close`, `closes`, `closed`, `fix`, `fixes`, `fixed`, `fix for`, `resolve`, `resolves`, `resolved`, `address`,
  `addresses`, `addressed`, `part of`, `ref`, `refs`, `references`, `relates to`, and `related to`. They are not case
  sensitive and may be followed by a colon.
* The title references the issue, for example `fix: resolve OutOfSync bug on PostDelete hook (#1234)`.
* The issue is linked in the *Development* section of the PR sidebar.

The issue can be written as `#1234`, `argoproj/argo-cd#1234`, or `https://github.com/argoproj/argo-cd/issues/1234`.
Several issues can follow a single phrase, for example `Fixes #1234, #5678`. Use a closing phrase such as `Closes` when
the PR fully resolves the issue, so that GitHub closes it when the PR is merged, and `Part of` otherwise:

```markdown
## Description

Closes #1234

Adds the `--foo` flag to `argocd app sync`.
```

> [!NOTE]
> References inside HTML comments and code blocks are ignored, and so are references in the description that do not
> follow a linking phrase. Writing "similar to #1234" does not link the issue.

### Exceptions

A PR does not need to link to an issue if it changes fewer than 50 lines, counting both added and removed lines,
outside of documentation and generated files. This covers typo corrections, small obvious bug fixes, and
documentation-only changes. The following files are not counted:

* Documentation: everything under `docs/`, Markdown (`*.md`) files anywhere in the repository, and `mkdocs.yml`.
* Generated files: every path marked `linguist-generated` in
  [`.gitattributes`](https://github.com/argoproj/argo-cd/blob/master/.gitattributes), such as `*.pb.go` files, mocks,
  `assets/swagger.json`, the CRDs, and the install manifests. This way, a small API change is not penalized for the
  output of `make codegen`.

The policy is also not enforced for:

* PRs opened by maintainers and other users with write access to the repository.
* PRs opened by bots, such as Renovate and Dependabot.
* PRs that target a branch other than `master`, such as cherry-picks to release branches.
* PRs labeled `policy/exempt`. Only maintainers can add this label.

Exempt PRs must still meet all the other requirements described on this page.

### Automatic enforcement

The [PR Issue Policy](https://github.com/argoproj/argo-cd/blob/master/.github/workflows/pr-issue-policy.yaml) workflow
checks every PR whenever it is opened, reopened, edited, pushed to, labeled, or marked ready for review. If the PR does
not meet the policy, the workflow:

1. Converts the PR to a draft. Maintainers do not routinely review drafts.
2. Adds the `policy/no-linked-issue` label if no issue is linked, or the `policy/issue-not-accepted` label if a linked
   enhancement does not have the `triage/accepted` label.
3. Posts a comment that explains what is missing and how to fix it.

To move forward, link an issue or wait for the enhancement to be accepted, then mark the PR ready for review. If the PR
was converted to a draft by the workflow and now meets the policy, editing the description or pushing a commit is enough:
the workflow marks it ready for review again. A draft that you opened yourself stays a draft until you mark it ready.

> [!WARNING]
> Adding the `triage/accepted` label to an issue does not trigger the workflow on PRs that link to it. After an
> enhancement is accepted, mark the PR ready for review, or edit its description, to have it checked again.

> [!NOTE]
> Maintainers who want to review a PR that does not meet the policy should add the `policy/exempt` label instead of
> marking the PR ready for review. Otherwise, the workflow converts it back to a draft.

### Labels

The policy uses the following labels:

| Label | Applies to | Meaning |
|-------|------------|---------|
| `triage/accepted` | Issues | A maintainer has accepted the enhancement for implementation. Only required on issues labeled `enhancement`. |
| `policy/no-linked-issue` | PRs | Added by the workflow: the PR does not link to an issue. |
| `policy/issue-not-accepted` | PRs | Added by the workflow: a linked enhancement does not have the `triage/accepted` label. |
| `policy/exempt` | PRs | Added by a maintainer: the workflow does not enforce the policy on this PR. |

The workflow removes the `policy/no-linked-issue` and `policy/issue-not-accepted` labels as soon as the PR meets the
policy. You can use them to find the PRs that are waiting on an issue, for example with the
[`label:policy/no-linked-issue`](https://github.com/argoproj/argo-cd/pulls?q=is%3Apr+is%3Aopen+label%3Apolicy%2Fno-linked-issue)
search.

## Before Submitting a PR

1. Rebase your branch against upstream master:
```shell
git fetch upstream
git rebase upstream/master
```

2. Run pre-commit checks:
```shell
make pre-commit-local
```

## Continuous Integration process

When you submit a PR against Argo CD's GitHub repository, a couple of CI checks will be run automatically to ensure your changes will build fine and meet certain quality standards. Your contribution needs to pass those checks in order to be merged into the repository.

> [!NOTE]
> Please make sure that you always create PRs from a branch that is up-to-date with the latest changes from Argo CD's master branch. Depending on how long it takes for the maintainers to review and merge your PR, it might be necessary to pull in latest changes into your branch again.

Please understand that we, as an Open Source project, have limited capacities for reviewing and merging PRs to Argo CD. We will do our best to review your PR and give you feedback as soon as possible, but please bear with us if it takes a little longer than expected. [Reviewing PRs](reviewing-prs.md) explains how reviewers mark what blocks your PR and how they let you know what happens next.

The following guide will help you to submit a PR that meets the standards of our CI tests:

## Title of the PR

Please use a meaningful and concise title for your PR. This will help us to pick PRs for review quickly, and the PR title will also end up in the Changelog.

We use [PR title checker](https://github.com/marketplace/actions/pr-title-checker) to categorize your PR into one of the following categories:

* `ci` - Your PR updates or improves Continuous Integration workflows
* `fix` - Your PR contains one or more code bug fixes
* `feat` - Your PR contains a new feature
* `test` - Your PR adds tests to the code base, or improves existing tests
* `docs` - Your PR improves the documentation
* `chore` - Your PR improves any internals of Argo CD, such as the build process, unit tests, etc
* `refactor` - Your PR refactors the code base, without adding new features or fixing bugs
* `revert` - Your PR reverts a previous commit

Please prefix the title of your PR with one of the valid categories. For example, if you chose the title your PR `Add documentation for GitHub SSO integration`, please use `docs: Add documentation for GitHub SSO integration` instead.

## PR template checklist

Upon opening a PR, the details will contain a checklist from a template. Please read the checklist, and tick those marks that apply to you.

## Automated builds and tests

After you have submitted your PR, and whenever you push new commits to that branch, GitHub will run a number of Continuous Integration checks against your code. It will execute the following actions, and each of them has to pass:

* Build the Go code (`make build`)
* Generate API glue code and manifests (`make codegen`)
* Run a Go linter on the code (`make lint`)
* Run the unit tests (`make test`)
* Run the End-to-End tests (`make test-e2e`)
* Build and lint the UI code (`make lint-ui`)
* Build the `argocd` CLI (`make cli`)

If any of these tests in the CI pipeline fail, it means that some of your contribution is considered faulty (or a test might be flaky, see below).

## Code test coverage

We use [CodeCov](https://codecov.io) in our CI pipeline to check for test coverage, and once you submit your PR, it will run and report on the coverage difference as a comment within your PR. If the difference is too high in the negative, i.e. your submission introduced a significant drop in code coverage, the CI check will fail.

Whenever you develop a new feature or submit a bug fix, please also write appropriate unit tests for it. If you write a completely new module, please aim for at least 80% of coverage.
If you want to see how much coverage just a specific module (i.e. your new one) has, you can set the `TEST_MODULE` to the (fully qualified) name of that module with `make test`, i.e.:

```bash
 make test TEST_MODULE=github.com/argoproj/argo-cd/server/cache
...
ok      github.com/argoproj/argo-cd/server/cache        0.029s  coverage: 89.3% of statements
```

## Cherry-picking fixes

If your PR contains a bug fix, and you want to have that fix backported to a previous release branch, please label your
PR with `cherry-pick/x.y` (example: `cherry-pick/3.1`). If you do not have access to add labels, ask a maintainer to add
them for you.

If you add labels before the PR is merged, the cherry-pick bot will open the backport PRs when your PR is merged.

Adding a label after the PR is merged will also cause the bot to open the backport PR.
