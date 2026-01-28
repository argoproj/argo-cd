# Backporting Fixes

After [submitting your PR](submit-your-pr.md) and getting it merged, you might want to backport it to previous releases.
The prerequisites for backporting are:

* Your PR is already merged into the `master` branch
* The changes are a bugfix
* The changes are non-breaking
* The backports are to [actively supported releases](https://github.com/argoproj/argo-cd/security/policy#supported-versions)

## Automated Process

Adding a `cherry-pick/x.y` label to your PR (where `x.y` is the version you want to backport to e.g. `3.1`) will 
trigger the cherry pick workflow to open a PR against the targeted release branch. Anyone with 
[Argoproj membership](https://github.com/argoproj/argoproj/blob/main/community/membership.md) can add the label, but an
Approver must merge the backport PR. If you cannot add the label yourself, ask the person who merged the PR to do it for
you.

## Manual Process

Sometimes the automation fails, generally because the cherry-pick involves merge conflicts. In that case, you can
manually create the backport PR. The process generally goes like this:

```shell
git checkout release-x.y
git pull origin release-x.y
git checkout -b backport-my-fix-to-x.y
git cherry-pick <commit hash of your bugfix's squash commit in master>
# Resolve any merge conflicts
git commit
git push <your fork> backport-my-fix-to-x.y
# Open a PR against release-x.y
```

Then repeat for the other release branches you want to backport to. Sometimes it's easier to cherry-pick the commit with
resolved conflicts from the release branch after you've resolved them for the first backport rather than repeatedly
resolving the same conflicts from the master branch for each backport. For example, after doing the steps above, do:

```shell
git rev-parse HEAD
# Note the commit hash
git checkout release-x.<y-1>
git pull origin release-x.<y-1>
git checkout -b backport-my-fix-to-x.<y-1>
git cherry-pick <commit hash from previous backport>
# Resolve any merge conflicts
git commit
git push <your fork> backport-my-fix-to-x.<y-1>
# Open a PR against release-x.<y-1>
```
