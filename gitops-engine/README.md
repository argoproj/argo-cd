# GitOps Engine

Various GitOps operators address different use-cases and provide different user experiences but all have similar set of core features. The team behind
[Argo CD](https://github.com/argoproj/argo-cd) has implemented a reusable library that implements core GitOps features:

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

### Contributing to the effort

At this stage we are interested in feedback, use-cases and help on the GitOps Engine.
