# Frequently Asked Questions

## General

**Q**: What's the backstory behind this?

**A**: The announcement blog posts do quite a good job of explaining where our two projects come from and why it was important for us to take the next step and join our efforts:

- Jay Pipes on the [AWS blog](https://aws.amazon.com/de/blogs/containers/help-us-write-a-new-chapter-for-gitops-kubernetes-and-open-source-collaboration/)
- Pratik Wadher on the [Intuit blog](https://www.intuit.com/blog/technology/introducing-argo-flux/)
- Tamao Nakahara on the [Weaveworks blog](https://www.weave.works/blog/argo-flux-join-forces)

----

**Q**: What are you hoping to get out of this collaboration?

**A**: Our primary motives for coming together to do this are:

- ArgoCD and Flux are two of the main GitOps projects, solving very similar problems, having very similar views on implementing GitOps.
- We want to offer a shared vision for GitOps and the best possible GitOps experience for everyone.
- We hope to bring a bigger community together than we can on our own.
- We want to learn from each other's approaches and offer the best in breed GitOps solution out there.

----

**Q**: What can current Flux users look forward to from this collaboration with Argo?

**A**: Here are a few of our favourites:

- Syncing will be more efficient. Instead of polling, flux will use Kubernetes Informers to get information from the cluster.
- Users will see a great (if not huge) reduction in K8S API calls and etcd traffic
We can increase the syncing frequency.
- Advanced syncing features such as pre-post sync hooks and sync waves
- Overall performance and efficiency improvements (registry scanning excluded) are the major gains for Flux users.
- Given the concept GitOps is quite young, and all people involved were involved early on (or present during the birth), I think people get an even more experienced team working on core features.

----

**Q**: What can current Argo users look forward to from this collaboration with Flux?

**A**: We hope Argo CD might get the following:

- Docker registry monitoring feature. It would be fantastic if we could extract existing Flux code into a reusable component which works for both Argo CD and Flux.
- Better cluster management experience. Right now Argo CD users use app of apps pattern which is not perfect. Perhaps we can learn from Flux community and contribute to GitOps engine to improve both Argo CD and Flux.
- Advanced Git related features like GPG commit verification, git secrets.
- Simplified installations/management.

----

**Q**: Does this project scope just synchronization of environment (git sync operator) or does it include progressive delivery?

**A**: We will be starting to work on a spec 2020.

----

**Q**: What comes after the GitOps Engine?

**A**: The ultimate goal is to merge user experiences and eventually have one project.
Right now nobody knows how exactly that project will look like and how exactly we get there. We will start with baby steps e.g. get rid of code duplication (gitops engine), merge documentation, slacks/issues tracking and then incrementally try to merge CLI/UI to get one user experience.

We also want to highlight that before we even start doing this we want to be properly introduced to each others communities, and understand each others use cases. As without this knowledge we won't be able to create something that will serve all of you.

----

**Q**: Was it hard to put together the design for the GitOps Engine? What was most important to you when you started putting it together?

**A**: It was hard for the right reasons. We wanted to design something which ticked off all these points:

1. Realistic both in theory and practice (e.g. starting a new project from scratch didn't make sense, creating a Frankenstein-like project from pieces of both projects also didn't make sense).
1. Would allow us keep us nurturing the communities of ArgoCD and Flux, towards a common product, without jeopardizing them (e.g. without disrespecting the communities).
1. Useful. Both projects would benefit from it as a first step towards a joint product/solution.

In addition to that finding a common language, without falling into project specific terms (keeping it really abstract), was also quite a challenge, e.g. what a repository is to Flux, is an application to Argo.
