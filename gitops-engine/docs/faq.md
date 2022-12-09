# Frequently Asked Questions

## General

**Q**: What's the backstory behind this?

**A**: In November 2019 the teams behind Argo CD and Flux announced that they were going to join efforts. Some of the announcement blog posts explain what the thinking of the time was:

- Jay Pipes on the [AWS blog](https://aws.amazon.com/de/blogs/containers/help-us-write-a-new-chapter-for-gitops-kubernetes-and-open-source-collaboration/)
- Pratik Wadher on the [Intuit blog](https://www.intuit.com/blog/technology/introducing-argo-flux/)
- Tamao Nakahara on the [Weaveworks blog](https://www.weave.works/blog/argo-flux-join-forces)

In the course of the next months, the two engineering teams [met on a regular basis](https://docs.google.com/document/d/17AEZgv6yVuD4HS7_oNPiMKmS7Q6vjkhk6jH0YCELpRk/edit) and scoped out the future of the project. Two options were on the table:

1. Rethink APIs and build the project from the ground up.
1. Extract useful code from Argo into an Engine project.

The latter was deemed to be the most practical solution.

March 2020 the Flux team made a [proof of concept](https://github.com/fluxcd/flux/pull/2886) available, which rebased the Flux on top of the GitOps Engine, but while looking at the breaking changes this was going to introduce the Flux team decided that it was a time for a [more ground-breaking approach](https://www.weave.works/blog/gitops-with-flux-v2) on how to do GitOps. After some experimentation, the GitOps Toolkit was put out as an RFC in June 2020.

A [number of other projects](https://github.com/search?q=argoproj%2Fgitops-engine&type=Code) already started looking at integrating the GitOps Engine.

The Argo and Flux teams decided all of this on good terms. All of these discussions were immensely helpful in shaping both projects' future. You might see each of us stealing good ideas from the other in the future and celebrate each others successes. There might be future collaborations, we'll keep you posted.

----

**Q**: What are you hoping to get out of this collaboration?

**A**: Our primary motives for coming together to do this are:

- Argo CD and Flux CD are two of the main GitOps projects, solving very similar problems, having very similar views on implementing GitOps.
- We want to offer a shared vision for GitOps and the best possible GitOps experience for everyone.
- We hope to bring a bigger community together than we can on our own.
- We want to learn from each other's approaches and offer the best in breed GitOps solution out there.

----

**Q**: What can current Argo CD users look forward to from this collaboration with Flux CD?

**A**: We hope Argo CD might get the following:

- Docker registry monitoring feature. It would be fantastic if we could extract existing Flux CD code into a reusable component which works for both Argo CD and Flux CD.
- Better cluster management experience. Right now Argo CD users use app of apps pattern which is not perfect. Perhaps we can learn from Flux CD community and contribute to GitOps engine to improve both Argo CD and Flux CD.
- Advanced Git related features like GPG commit verification, git secrets.
- Simplified installations/management.

----

**Q**: Does this project scope just synchronization of environment (git sync operator) or does it include progressive delivery?

**A**: We will be starting to work on a spec for progressive delivery alignment between Argo Rollouts and Weave Flagger in 2020.

----

**Q**: What comes after the GitOps Engine?

**A**: The ultimate goal is to merge user experiences and eventually have one project.
Right now nobody knows how exactly that project will look like and how exactly we get there. We will start with baby steps e.g. get rid of code duplication (gitops engine), merge documentation, slacks/issues tracking and then incrementally try to merge CLI/UI to get one user experience.

We also want to highlight that before we even start doing this we want to be properly introduced to each others communities, and understand each others use cases. As without this knowledge we won't be able to create something that will serve all of you.

----

**Q**: Was it hard to put together the design for the GitOps Engine? What was most important to you when you started putting it together?

**A**: It was hard for the right reasons. We wanted to design something which ticked off all these points:

1. Realistic both in theory and practice (e.g. starting a new project from scratch didn't make sense, creating a Frankenstein-like project from pieces of both projects also didn't make sense).
1. Would allow us keep us nurturing the communities of Argo CD and Flux CD, towards a common product, without jeopardizing them (e.g. without disrespecting the communities).
1. Useful. Both projects would benefit from it as a first step towards a joint product/solution.

In addition to that finding a common language, without falling into project specific terms (keeping it really abstract), was also quite a challenge, e.g. what a repository is to Flux CD, is an application to Argo CD.
