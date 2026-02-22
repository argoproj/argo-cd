# Submitting PRs

## Prerequisites
1. [Development Environment](development-environment.md)   
2. [Toolchain Guide](toolchain-guide.md)
3. [Development Cycle](development-cycle.md)

## Preface

> [!NOTE]
> **Before you start**
>
> The Argo CD project continuously grows, both in terms of features and community size. It gets adopted by more and more organizations which entrust Argo CD to handle their critical production workloads. Thus, we need to take great care with any changes that affect compatibility, performance, scalability, stability and security of Argo CD. For this reason, every new feature or larger enhancement must be properly designed and discussed before it gets accepted into the code base.
>
> We do welcome and encourage everyone to participate in the Argo CD project, but please understand that we can't accept each and every contribution from the community, for various reasons. If you want to submit code for a great new feature or enhancement, we kindly ask you to take a look at the
> [code contribution guide](code-contributions.md#) before you start to write code or submit a PR.

If you want to submit a PR, please read this document carefully, as it contains important information guiding you through our PR quality gates.

If you need guidance with submitting a PR, or have any other questions regarding development of Argo CD, do not hesitate to [join our Slack](https://argoproj.github.io/community/join-slack) and get in touch with us in the `#argo-cd-contributors` channel!

## Before Submitting a PR

1. Rebase your branch against upstream main:
```shell
git fetch upstream
git rebase upstream/main
```

2. Run pre-commit checks:
```shell
make pre-commit-local
```

## Continuous Integration process

When you submit a PR against Argo CD's GitHub repository, a couple of CI checks will be run automatically to ensure your changes will build fine and meet certain quality standards. Your contribution needs to pass those checks in order to be merged into the repository.

> [!NOTE]
> Please make sure that you always create PRs from a branch that is up-to-date with the latest changes from Argo CD's master branch. Depending on how long it takes for the maintainers to review and merge your PR, it might be necessary to pull in latest changes into your branch again.

Please understand that we, as an Open Source project, have limited capacities for reviewing and merging PRs to Argo CD. We will do our best to review your PR and give you feedback as soon as possible, but please bear with us if it takes a little longer as expected.

The following read will help you to submit a PR that meets the standards of our CI tests:

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

Please prefix the title of your PR with one of the valid categories. For example, if you chose the title your PR `Add documentation for GitHub SSO integration`, please use `docs: Add documentation for GitHub SSO integration` instead.

## PR template checklist

Upon opening a PR, the details will contain a checklist from a template. Please read the checklist, and tick those marks that apply to you.

## Automated builds & tests

After you have submitted your PR, and whenever you push new commits to that branch, GitHub will run a number of Continuous Integration checks against your code. It will execute the following actions, and each of them has to pass:

* Build the Go code (`make build`)
* Generate API glue code and manifests (`make codegen-local`)
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
