# Contribution FAQ

## General

### Can I discuss my contribution ideas somewhere?

Sure thing! You can either open an Enhancement Proposal in our GitHub issue tracker or you can [join us on Slack](https://argoproj.github.io/community/join-slack) in channel #argo-dev to discuss your ideas and get guidance for submitting a PR.

### Noone has looked at my PR yet. Why?

As we have limited man power, it can sometimes take a while for someone to respond to your PR. Especially, when your PR contains complex or non-obvious changes. Please bear with us, we try to look at every PR that we receive.

### Why has my PR been declined? I put much work in it!

We appreciate that you have put your valuable time and know how into a contribution. Alas, some changes do not fit into the overall ArgoCD philosophy, and therefore can't be merged into the official ArgoCD source tree.

To be on the safe side, make sure that you have created an Enhancement Proposal for your change before starting to work on your PR and have gathered enough feedback from the community and the maintainers.

## Failing CI checks 

### One of the CI checks failed. Why?

You can click on the "Details" link next to the failed step to get more details about the failure. This will take you to CircleCI website.

![CircleCI pipeline](ci-pipeline-failed.png)

### Can I retrigger the checks without pushing a new commit?

Since the CI pipeline is triggered on Git commits, there is currently no (known) way on how to retrigger the CI checks without pushing a new commit to your branch.

If you are absolutely sure that the failure was due to a failure in the pipeline, and not an error within the changes you commited, you can push an empty commit to your branch, thus retriggering the pipeline without any code changes. To do so, issue

```bash
git commit --allow-empty -m "Retrigger CI pipeline"
git push origin <yourbranch>
```

### Why does the build step fail?

Chances are that it fails for two of the following reasons in the CI while running fine on your machine:

* Sometimes, CircleCI kills the build step due to excessive memory usage. This happens rarely, but it has happened in the past. If you see a message like "killed" in the log output of CircleCI, you should retrigger the pipeline as described above. If the issue persists, please let us know.

* If the build is failing at the `Ensuring Gopkg.lock is up-to-date` step, you need to update the dependencies before you push your commits. Run `make dep-ensure` and `make dep` and commit the changes to `Gopkg.lock` to your branch.

### Why does the codegen step fail?

If the codegen step fails with "Check nothing has changed...", chances are high that you did not run `make codegen`, or did not commit the changes it made. You should double check by running `make codegen` followed by `git status` in the local working copy of your branch. Commit any changes and push them to your GH branch to have the CI check it again.

A second common case for this is, when you modified any of the auto generated assets, as these will be overwritten upon `make codegen`.

Generally, this step runs `codegen` and compares the outcome against the Git branch it has checked out. If there are differences, the step will fail.

### Why does the lint step fail?

The lint step is most likely to fail for two reasons:

* The `golangci-lint` process was OOM killed by CircleCI. This happens sometimes, and is annoying. This is indicated by a `Killed.` message in the CircleCI output.
  If this is the case, please re-trigger the CI process as described above and see if it runs through.

* Your code failed to lint correctly, or modifications were performed by the `golangci-lint` process. You should run `make lint` on your local branch and fix all the issues.

### Why does the test or e2e steps fail?

You should check for the cause of the failure on the CircleCI web site, as described above. This will give you the name of the test that has failed, and details about why. If your test are passing locally (using the virtualized toolchain), chances are that the test might be flaky and will pass the next time it is run. Please retrigger the CI pipeline as described above and see if the test step now passes.