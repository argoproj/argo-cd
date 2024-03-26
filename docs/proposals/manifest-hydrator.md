---
title: Manifest Hydrator
authors:
  - "@crenshaw-dev"
  - "@zachaller"
sponsors:
  - TBD        # List all interested parties here.
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2024-03-26
last-updated: 2024-03-26
---

# Manifest Hydrator

This proposal describes a feature to make manifest hydration (i.e. the "rendered manifest pattern") a first-class feature of Argo CD.

## Open Questions 

* The `sourceHydrator` field is mutually exclusive with the `source` and the `sources` field. Should we throw an error if they're both configured, or should we just pick one and ignore the others?
* How will/should this feature relate to the image updater? Is there an opportunity to share code, since both tools involve pushing to git?

## Summary

Manifest hydration tools like Helm and Kustomize are indispensable in GitOps. These tools transform "dry" (Don't Repeat Yourself) sources into plain Kubernetes manifests. The effects of a change to dry sources are not always obvious. So storing only dry sources in git leaves the user with an incomplete and confusing history of their application. This undercuts some of the main benefits of GitOps.

The "rendered manifests" pattern has emerged as a way to mitigate the downsides of using hydration tools in GitOps. Today, developers use CI tools to automatically hydrate manifests and push to separate branches. They then configure Argo CD to deploy from the hydrated branches.

This proposal describes manifest hydration and pushing to git as a first-class feature of Argo CD.

It offers two modes of operation: push-to-deploy and push-to-stage. In push-to-deploy, hydrated manifests are pushed to the same branch from which Argo CD deploys. In push-to-stage, manifests are pushed to a different branch, and Argo CD relies on some external system to move changes to the deployment branch; this provides an integration point for automated environment promotion systems.

## Motivation

Many organizations have implemented their own manifest hydration system. By implementing it in Argo CD, we can lower the cost to our users of maintaining those systems, and we can encourage best practices related to the pattern.

### Goals

1) Make manifest hydration easy and intuitive for Argo CD users
2) Make it possible to implement a promotion system which relies on the manifest hydration's push-to-stage mode

### Non-Goals

1) Implementing a change promotion system

## Proposal

A `sourceHydrator` field will be added to the Argo CD Application spec:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: example
spec:
  # The sourceHydrator field is mutually-exclusive with `source` and with `sources`. If this field is configured, we 
  # should either throw an error or ignore the other two.
  sourceHydrator:
    drySources:
    - repoURL: https://github.com/argoproj/argocd-example-apps
      targetRevision: main
      # This assumes the Application's environments are modeled as directories.
      path: environments/e2e
      #chart: my-chart # if it’s a Helm chart
      # Hydrator-specific fields like “helm,” “kustomize,” “directory,” and
      # “plugin” are not available here. Those source details must be in git,
      # in a .argocd-source.yaml file.
      # This is because every change to the manifests must have a 
      # corresponding dry commit.
    writeTo:
      repoURL: https://github.com/argoproj/argocd-example-apps
      targetBranch: environments/e2e-next
    # The hydratedSource field is optional. If omitted, the `writeTo` repo/branch is used.
    # In this example, we write to a "staging" branch and then rely on an external promotion system to move the change 
    # to the configured hydratedSource.
    hydratedSource:
      repoURL: https://github.com/argoproj/argocd-example-apps
      targetBranch: environments/e2e
```

### Use cases

#### Use case 1:

#### Use case 2:

### Implementation Details/Notes/Constraints

### Detailed examples

### Security Considerations

This proposal would involve introducing a component capable of pushing to git. 

We'll need to consider what git permissions setup to recommend, what security features we should recommend enabling (e.g. branch protection), etc.

We'll also need to consider how to store the git push secrets. It's probable that they'll need to be stored in a namespace separate from the other Argo CD components to provide a bit extra protection.

### Risks and Mitigations

### Upgrade / Downgrade Strategy

## Drawbacks

## Alternatives
