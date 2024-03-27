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
      # repoURL is optional. If not specified, it's assumed to be the same as drySources[0].repoURL.
      repoURL: https://github.com/argoproj/argocd-example-apps
      targetBranch: environments/e2e-next
      path: .
    # The hydratedSource field is optional. If omitted, the `writeTo` repo/branch is used.
    # In this example, we write to a "staging" branch and then rely on an external promotion system to move the change 
    # to the configured hydratedSource.
    hydratedSource:
      # repoURL is optional. If not specified, it's assumed to be the same as drySources[0].repoURL.
      repoURL: https://github.com/argoproj/argocd-example-apps
      targetBranch: environments/e2e
      # The path is assumed to be the same as that in writeTo.
```

When the Argo CD application controller detects a new commit on the first source listed under `drySources`, it will start the hydration process.

First, Argo CD will collect all Applications which have the same `drySources[0]` repo and targetRevision.

Argo CD will then group those sources by the configured `writeTo` repoURL and targetBranch.

Then Argo CD will loop over the apps in each group. For each group, it will run manifest hydration on the configured `drySources[0].path` and write the result to the configured `writeTo.path`. After looping over all apps in the group and writing all their manifests, it will commit the changes to the configured `writeTo` repoURL and targetBranch. Finally, it will push those changes to git. Then it will repeat this process for the remaining groups. 

The actual push operation should be delegated to some system outside the application controller. Communication may occur via some shared DB (maybe Redis) or via network, e.g. gRPC.

To understand how this would work for a simple dev/test/prod setup with two regions, consider this example:

```yaml
### DEV APPS ###
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: dev-west
spec:
  sourceHydrator:
    drySources:
    - repoURL: https://github.com/argoproj/argocd-example-apps
      targetRevision: main
      path: environments/dev/west
    writeTo:
      targetBranch: environments/dev
      path: west
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: dev-east
spec:
  sourceHydrator:
    drySources:
      - repoURL: https://github.com/argoproj/argocd-example-apps
        targetRevision: main
        path: environments/dev/east
    writeTo:
      targetBranch: environments/dev
      path: east
---
### TEST APPS ###
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-west
spec:
  sourceHydrator:
    drySources:
      - repoURL: https://github.com/argoproj/argocd-example-apps
        targetRevision: main
        path: environments/test/west
    writeTo:
      targetBranch: environments/test
      path: west
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-east
spec:
  sourceHydrator:
    drySources:
      - repoURL: https://github.com/argoproj/argocd-example-apps
        targetRevision: main
        path: environments/test/east
    writeTo:
      targetBranch: environments/prod
      path: east
---
### PROD APPS ###
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: prod-west
spec:
  sourceHydrator:
    drySources:
      - repoURL: https://github.com/argoproj/argocd-example-apps
        targetRevision: main
        path: environments/prod/west
    writeTo:
      targetBranch: environments/prod
      path: west
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: prod-east
spec:
  sourceHydrator:
    drySources:
      - repoURL: https://github.com/argoproj/argocd-example-apps
        targetRevision: main
        path: environments/prod/east
    writeTo:
      targetBranch: environments/prod
      path: east
---
```

Each commit to the dry branch will result in a commit to up to three branches. Each commit to an environment branch will contain changes for west, east, or both (depending on which is affected). Changes originating from a single dry commit are always grouped into a single hydrated commit.

Each output directory should contain two files: manifest.yaml and README.md. manifest.yaml should contain the plain hydrated manifests. The resources should be sorted by namespace, name, group, and kind (in that order).

The README should contain the following:

````gotemplate
{{ if eq (len .applications) 1 }}
{{ $appName := (index .applications 0).metadata.name }}
# {{ $appName }} Manifests

[manifest.yaml](./manifest.yaml) contains the hydrated manifests for the {{ $appName }} application.
{{ end }}
{{ if gt (len .applications) 1 }}
{{ $appName := (index .applications 0).metadata.name }}
# Manifests for {{ len .applications }} Applications

[manifest.yaml](./manifest.yaml) contains the hydrated manifests for these applications:
{{ range $i, $app := .applications }}
- {{ $app.name }}
{{ end }}
{{ end }}

These are the details of the most recent change;
* Author: {{ .commitAuthor }}
* Message: {{ .commitMessage }}
* Time: {{ .commitTime }}

To reproduce the manifest hydration, do the following:

```
git clone {{ .repoURL }}
cd {{ .repoName }}
git checkout {{ .dryShortSHA }}
{{ .command }}
```
````

This template should be admin-configurable.

Example output might look like this:

````markdown
# dev-west Manifests

[manifest.yaml](./manifest.yaml) contains the hydrated manifests for the dev-west application.

These are the details of the most recent change;
* Author: Michael Crenshaw <michael@example.com>
* Message: chore: bumped image tag to v0.0.2
* Time: 2024-03-27 10:32:04 UTC

To reproduce the manifest hydration, do the following:

```
git clone https://github.com/argoproj/argocd-example-apps
cd argocd-example-apps
git checkout ab2382f
kustomize build environments/dev/west
```
````

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
