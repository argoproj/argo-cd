# GitOps Engine
<div align="center">

![image](https://user-images.githubusercontent.com/426437/82109570-f6c7ed80-96eb-11ea-849c-2bd5fe89b571.png)

</div>

Various GitOps operators address different use-cases and provide different user experiences but all have similar set of core features. The teams behind
[Argo CD](https://github.com/argoproj/argo-cd) and [Flux CD](https://github.com/fluxcd/flux) have implemented a reusable library that implements core GitOps features:

- Kubernetes resource cache ✅
- Resources reconciliation ✅
- Sync Planning ✅
- Access to Git repositories
- Manifest Generation

## Proposals, specifications and ideas

Do you want to propose one more feature and want to enhance the existing one?
Proposals and ideas are in markdown docs in the [`specs/`](specs/) directory.
To create a new proposal, simply copy the spec [`template`](specs/template.md),
name the file corresponding to the title of your proposal, and place it in the
`specs/` directory.

A good starting point to understand the structure is the [GitOps Engine Design spec](specs/design.md).

We tried to answer frequently asked question in a [separate FAQ document](docs/faq.md).

## Governance

This project is licensed under the [Apache 2 license](LICENSE).

The GitOps Engine follows the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md).

## Get involved

If you are as excited about GitOps and one common engine for it as much as we are, please get in touch. If you want to write code that's great, if you want to share feedback, ideas and use-cases, that's great too.

Find us on the [#gitops channel][gitops-slack] on Kubernetes Slack (get an [invite here][kube-slack]).

[gitops-slack]: https://kubernetes.slack.com/archives/CBT6N1ASG
[kube-slack]: https://slack.k8s.io/

### Meetings

The developer team meets regularly, every 1st and 3rd Tuesday of the month, [16:00 UTC](http://time.unitarium.com/utc/16). Instructions, agenda and minutes can be found in [the meeting doc](https://docs.google.com/document/d/17AEZgv6yVuD4HS7_oNPiMKmS7Q6vjkhk6jH0YCELpRk/edit#). The meetings will be recorded and added to this [Youtube playlist](https://www.youtube.com/playlist?list=PLbx4FZ4kOKnvSQP394o5UdF9wL7FaQd-R).

We look forward to seeing you at our meetings and hearing about your feedback and ideas there!

### Contributing to the effort

At this stage we are interested in feedback, use-cases and help on the GitOps Engine.
