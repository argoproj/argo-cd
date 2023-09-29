# GitOps Engine Design

## Summary

Flux and ArgoCD are two popular open-source GitOps implementations. They currently offer different user experiences but, at their core, Flux and ArgoCD have a lot in common.
Therefore, the Flux and ArgoCD maintainers have decided to join forces, with the hypothesis that working on a single project will be more effective, avoiding duplicate work and
ultimately bringing more and better value to the end-user.

Effectively merging Flux and ArgoCD into a single solution is a long term goal. As a first step, both the ArgoCD and Flux teams are going to work on designing and implementing
the GitOps Engine.

![](https://user-images.githubusercontent.com/426437/66851601-ea9a6880-ef2f-11e9-807d-0c5f09fcc384.png)

The maintenance and support of the GitOps Engine will be a joined effort by the Flux and ArgoCD teams.

## Goals

The GitOps Engine:
* should contain core functionality pre-existing in ArgoCD and Flux.

## Non-Goals

* is not intended as a general framework for implementing GitOps services.

## Proposals

Teams have considered two ways to extract common functionality into the GitOps engine:
1. [Bottom-up](./design-bottom-up.md). Identify components that are used in both projects and move them one by one into GitOps engine repository.
1. [Top-down](./design-top-down.md). Take a whole sub-system of one project and make it customizable enough to be suitable for both Argo CD and Flux.
