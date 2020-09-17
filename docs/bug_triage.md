# Bug triage proposal for ArgoCD

## Situation

Lots of issues on our issue tracker. Many of them not bugs, but questions,
or very environment related. It's easy to lose oversight.

Also, it's not obvious which bugs are important. Which bugs should be fixed
first? Can we make a new release with the current inventory of open bugs?
Is there still a bug that should make it to the new release?

## Proposal

We should agree upon a common issue triage process. The process must be lean
and efficient, and should support us and the community looking into the GH
issue tracker at making the following decisions:

* Is it even a real bug?
* If it is a real bug, what is the current status of the bug (next to "open" or "closed")?
* How important is it to fix the bug?
* How urgent is it to fix the bug?
* Who will be working to fix the bug?

We need new methods to classify our bugs, at least into these categories:

* validity: Does the issue indeed represent a true bug
* severity: Denominates what impact the bug has
* priority: Denominates the urgency of the fix

## Triage process

GH issue tracker provides us with the possibility to label issues. Using these
labels is not perfect, but should give a good start. Each new issue created in
our issue tracker should be correctly labeled during its lifecycle, so keeping
an overview would be simplified by the ability to filter for labels.

The triage process could be as follows:

1. A new bug issue is created by someone on the tracker

1. The first person of the core team to see it will start the triage by classifying
   the issue (see below). This will indicate the creator that we have noticed the
   issue, and that it's not "fire & forget" tracker.

1. Initial classification should be possible even when much of the information is
   missing yet. In this case, the issue would be classified as such (see below).
   Again, this indicates that someone has noticed the issue, and there is activity
   in progress to get the required information.

1. Classification of the issue can change over its life-cycle. However, once the
   issue has been initially classified correctly (that is, with something else than
   the "placeholder" classification discussed above), changes to the classification
   should be discussed first with the person who initially classified the issue.

## Classification

We have introduced some new labels in the GH issue tracker for classifying the
bug issues. These labels are prefixed with the string `bug/`, and should be
applied to all new issues in our tracker.

### Classification requires more information

If it is not yet possible to classify the bug, i.e. because more information is
required to correctly classify the bug, you should always set the label
`bug/in-triage` to make it clear that triage process has started but could not
yet be completed.

### Issue type

If it's clear that a bug issue is not a bug, but a question or reach for support,
it should be marked as such:

* Remove any of the labels prefixed `bug/` that might be attached to the issue
* Remove the label `bug` from the issue
* Add the label `inquiry` to the issue

If the inquiry turns out to be something that should be covered by the docs, but
is not, the following actions should be taken:

* The title of the issue should be adapted that it will be clear that the bug
  affects the docs, not the code
* The label `documentation` should be attached to the issue

If the issue is too confusing (can happen), another possibility is to close the
issue and create a new one as described in above (with a meaningful title and
the label `documentation` attached to it).

### Validity

Some reported bugs may be invalid. It could be a user error, a misconfiguration
or something along these lines. If it is clear that the bug falls into one of
these categories:

* Remove any of the labels prefixed `bug/` that might be attached to the issue
* Add the label `invalid` to the issue
* Retain the `bug` label to the issue
* Close the issue

When closing the issue, it is important to let requester know why the issue
has been closed. The optimum would be to provide a solution to his request
in the comments of the issue, or at least pointers to possible solutions.

### Regressions

Sometimes it happens that something that worked in a previous release does
not work now when it should still work. If this is the case, the following
actions should be done

* Add the label `regression` to the issue
* Continue with triage

### Severity

It is important to find out how severe the impact of a bug is, and to label
the bug with this information. For this purpose, the following labels exist
in our tracker:

* `bug/severity:minor`: Bug has limited impact and maybe affects only an
  edge-case. Core functionality is not affected, and there is no data loss
  involved. Something might not work as expected. Example of these kind of
  bugs could be a CLI command that is not working as expected, a glitch in
  the UI, wrong documentation, etc.

* `bug/severity:major`: Malfunction in one of the core components, impacting
  a majority of users or one of the core functionalities in ArgoCD. There is
  no data loss involved, but for example a sync is not working due to a bug
  in ArgoCD (and not due to user error), manifests fail to render, etc.

* `bug/severity:critical`: A critical bug in ArgoCD, possibly resulting in
  data loss, integrity breach or severe degraded overall functionality.

### Priority

The priority of an issue indicates how quickly the issue should be fixed and
released. This information should help us in deciding the target release for
the fix, and whether a bug would even justify a dedicated patch release. The
following labels can be used to classify bugs into their priority:

* `bug/priority:low`: Will be fixed without any specific target release.

* `bug/priority:medium`: Should be fixed in the minor or major release, which
  ever comes first.

* `bug/priority:high`: Should be fixed with the next patch release.

* `bug/priority:urgent`: Should be fixed immediately and might even justify a
  dedicated patch release.

The priority should be set according to the value of the fix and the attached
severity. This means. a bug with a severity of `minor` could still be classified
with priority `high`, when it is a *low hanging fruit* (i.e. the bug is easy to
fix with low effort) and contributes to overall user experience of ArgoCD.

Likewise, a bug classified with a severity of `major` could still have a
priority of `medium`, if there is a workaround available for the bug which
mitigates the effects of the bug to a bearable extend.

Bugs classified with a severity of `critical` most likely belong to either
the `urgent` priority, or to the `high` category when there is a workaround
available.

Bugs that have a `regression`label attached (see Regression above) should
usually be handled with higher priority, so those kind of issues will most
likely have a priority of `high` or `urgent` attached to it.

## Summary

Applying a little discipline when working with our issue tracker could greatly
help us in making informed decision about which bugs to fix when. Also, it
would help us to get a clear view whether we can do for example a new minor
release without having forgot any outstanding issues that should make it into
that release.

If we are able to work with classification of bug issues, we might want to
extend the triage for enhancement proposals and PRs as well.
