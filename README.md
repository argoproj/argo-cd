# GitOps Engine

Two of the biggest GitOps projects are joining forces. [Argo CD](https://github.com/argoproj/argo-cd) and [Flux CD](https://github.com/fluxcd/flux) have been out there for years, making the lives of operators and organisations easier. Now the teams are building the best possible GitOps solution together. The first step on this journey is the GitOps Engine.

Argo and Flux already have similar sets of functionality, soon they will start sharing code using the GitOps Engine. The GitOps Engine will be responsible for the following functionality:

- Access to Git repositories
- Kubernetes resource cache
- Manifest Generation
- Resources reconciliation
- Sync Planning

Over time, this might be useful to other projects too.

## Proposals, specifications and ideas

Proposals and ideas are in markdown docs in the [`specs/`](specs/) directory.
To create a new proposal, simply copy the spec [`template`](specs/template.md),
name the file corresponding to the title of your proposal, and place it in the
`specs/` directory.

A good starting point to understand the structure is the [GitOps Engine Design spec](specs/design.md).

## Governance

This project is licensed under the [Apache 2 license](LICENSE).

The GitOps Engine follows the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md).

## Get involved

If you are as excited about GitOps and one common engine for it as much as we are, please get in touch. If you want to write code that's great, if you want to share feedback, ideas and use-cases, that's great too.

Find us on the [#gitops channel](https://kubernetes.slack.com/messages/gitops) on Kubernetes Slack (get an [invite here](https://kubernetes.slack.com/)).
