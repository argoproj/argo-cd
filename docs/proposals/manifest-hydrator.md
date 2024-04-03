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
* Should we enforce a naming convention for hydrated manifest branches, e.g. `argo/...`? This would make it easier to recommend branch protection rules, for example, only allow pushes to `argo/*` from the argo bot.

## Summary

Manifest hydration tools like Helm and Kustomize are indispensable in GitOps. These tools transform "dry" (Don't Repeat Yourself) sources into plain Kubernetes manifests. The effects of a change to dry sources are not always obvious. So storing only dry sources in git leaves the user with an incomplete and confusing history of their application. This undercuts some of the main benefits of GitOps.

The "rendered manifests" pattern has emerged as a way to mitigate the downsides of using hydration tools in GitOps. Today, developers use CI tools to automatically hydrate manifests and push to separate branches. They then configure Argo CD to deploy from the hydrated branches. (For more information, see the awesome [blog post](https://akuity.io/blog/the-rendered-manifests-pattern/) and [ArgoCon talk](https://www.youtube.com/watch?v=TonN-369Qfo) by Nicholas Morey.)

This proposal describes manifest hydration and pushing to git as a first-class feature of Argo CD.

It offers two modes of operation: push-to-deploy and push-to-stage. In push-to-deploy, hydrated manifests are pushed to the same branch from which Argo CD deploys. In push-to-stage, manifests are pushed to a different branch, and Argo CD relies on some external system to move changes to the deployment branch; this provides an integration point for automated environment promotion systems.

## Motivation

Many organizations have implemented their own manifest hydration system. By implementing it in Argo CD, we can lower the cost to our users of maintaining those systems, and we can encourage best practices related to the pattern.

### Goals

1) Make manifest hydration easy and intuitive for Argo CD users
2) Make it possible to implement a promotion system which relies on the manifest hydration's push-to-stage mode
3) Emphasize maintaining as much of the system's state as possible in git rather than in the Application CR (e.g. source hydrator config values, such as Helm values)
4) Every deployed change must have a corresponding dry commit - i.e. git is always the source of any changes

### Non-Goals

1) Implementing a change promotion system

## Proposal

Today, Argo CD watches one or more git repositories (configured in the `spec.source` or `spec.sources` field). When a new commit appears, Argo CD updates the desired state by rendering the manifests with the configured manifest hydration tool. If auto-sync is enabled, Argo CD applies the new manifests to the cluster.

With the introduction of this change, Argo CD will watch two revisions in the same git repository: the first is the "dry source", i.e. the git repo/revision where the un-rendered manifests reside, and the second is the "hydrated source," where the rendered manifests are places and retrieved for syncing to the cluster.

### New `spec.sourceHydrator` Application Field

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
    drySource:
      repoURL: https://github.com/argoproj/argocd-example-apps
      targetRevision: main
      # This assumes the Application's environments are modeled as directories.
      path: environments/e2e
    writeTo:
      targetBranch: environments/e2e-next
      path: .
    # The hydratedSource field is optional. If omitted, the `writeTo` repo/branch is used.
    # In this example, we write to a "staging" branch and then rely on an external promotion system to move the change 
    # to the configured hydratedSource.
    hydratedSource:
      targetBranch: environments/e2e
      # The path is assumed to be the same as that in writeTo.
```

When the Argo CD application controller detects a new commit on the first source listed under `drySources`, it queue up the hydration process.

### Processing a New Dry Commit

On noticing a new dry commit, Argo CD will first collect all Applications which have the same `drySources[0]` repo and targetRevision.

Argo CD will then group those sources by the configured `writeTo` targetBranch.

```go
package hydrator

import "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

type DrySource struct {
	repoURL        string
	targetRevision string
}

type HydratedSource struct {
	targetBranch string
}

var appGroups map[DrySource]map[HydratedSource][]v1alpha1.Application
```

Then Argo CD will loop over the apps in each group. For each group, it will run manifest hydration on the configured `drySources[0].path` and write the result to the configured `writeTo.path`. After looping over all apps in the group and writing all their manifests, it will commit the changes to the configured `writeTo` repoURL and targetBranch. Finally, it will push those changes to git. Then it will repeat this process for the remaining groups. 

The actual push operation should be delegated to the [commit server](./manifest-hydrator/commit-server/README.md).

To understand how this would work for a simple dev/test/prod setup with two regions, consider this example:

```yaml
### DEV APPS ###
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: dev-west
spec:
  sourceHydrator:
    drySource:
      repoURL: https://github.com/argoproj/argocd-example-apps
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
    drySource:
      repoURL: https://github.com/argoproj/argocd-example-apps
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
    drySource:
      repoURL: https://github.com/argoproj/argocd-example-apps
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
    drySource:
      repoURL: https://github.com/argoproj/argocd-example-apps
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
    drySource:
      repoURL: https://github.com/argoproj/argocd-example-apps
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
    drySource:
      repoURL: https://github.com/argoproj/argocd-example-apps
      targetRevision: main
      path: environments/prod/east
    writeTo:
      targetBranch: environments/prod
      path: east
---
```

Each commit to the dry branch will result in a commit to up to three branches. Each commit to an environment branch will contain changes for west, east, or both (depending on which is affected). Changes originating from a single dry commit are always grouped into a single hydrated commit.

### Handling External Values Files

Since only one source may be used in as the dry source, the multi-source approach to external Helm values files will not work here. Instead, we'll recommend that users use the umbrella chart approach. The main reasons for multi-source as an alternative were convenience (no need to maintain the parent chart) and resolving issues with authentication to dependency charts. We believe the simplification is worth the cost of convenience, and we can address the auth issues as standalone bugs.

An earlier iteration of this proposal attempted to preserve the multi-source style of external value file inclusion by introducing a "magic" `.argocd-hydrator.yaml` file containing `additionalSources` to reference the Helm chart. In the end, it felt like we were re-implementing Helm's dependencies feature or git submodules. It's better to just rely on one of those existing tools.

### `.argocd-source.yaml` Support

The `spec.sourceHydrator.drySource` field contains only three fields: `repoURL`, `targetRevision`, and `path`.

`spec.source` contains a number of fields for configuring manifest hydration tools (`helm`, `kustomize`, and `directory`). That functionality is still available for `spec.sourceHydrator`. But instead of being configured in the Application CR, those values are set in `.argocd-source.yaml`, an existing "override" mechanism for `spec.source`. By requiring that this configuration be set in `.argocd-source.yaml`, we respect the principle that all changes must be made in git instead of in the Application CR.

### Commit Metadata

Each output directory should contain two files: manifest.yaml and README.md. manifest.yaml should contain the plain hydrated manifests. The resources should be sorted by namespace, name, group, and kind (in that order).

The README will be built using the following template:

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
{{ range $i, $command := .commands }}
{{ $command }}
{{ end }}
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
kustomize edit set image my-app:v0.0.2
kustomize build environments/dev/west
```
````

The hydrator will also write a `hydrator.metadata` file containing a JSON representation of all the values available for README templating. This metadata can be used by external systems (e.g. a PR-based promoter system) to generate contextual information about the hydrated manifest's provenance.

```json
{
  "commands": ["kustomize edit set image my-app:v0.0.2", "kustomize build ."],
  "drySHA": "ab2382f",
  "commitAuthor": "Michael Crenshaw <michael@example.com>",
  "commitMessage": "chore: bump Helm dependency chart to 32.1.12",
  "repoURL": "https://github.com/argoproj/argocd-example-apps"
}
```

To request a commit to the hydrated branch, the application controller will make a gRPC call to the CommitManifests service.

A single call will bundle all the changes destined for a given targetBranch.

It's the application controller's job to ensure that the user has write access to the repo before making the call.

```protobuf
// CommitManifests represents the caller's request for some Kubernetes manifests to be pushed to a git repository.
message CommitManifests {
  // repoURL is the URL of the repo we're pushing to. HTTPS or SSH URLs are acceptable.
  required string repoURL = 1;
  // targetBranch is the name of the branch we're pushing to.
  required string targetBranch = 2;
  // drySHA is the full SHA256 hash of the "dry commit" from which the manifests were hydrated.
  required string drySHA = 3;
  // commitAuthor is the name of the author of the dry commit.
  required string commitAuthor = 4;
  // commitMessage is the short commit message from the dry commit.
  required string commitMessage = 5;
  // commitTime is the dry commit timestamp.
  required string commitTime = 6;
  // details holds the information about the actual hydrated manifests.
  repeated CommitPathDetails details = 7;
}

// CommitManifestDetails represents the details about a 
message CommitPathDetails {
  // path is the path to the directory to which these manifests should be written.
  required string path = 1;
  // manifests is a list of JSON documents representing the Kubernetes manifests.
  repeated string manifests = 2;
  // readme is a string which will be written to a README.md alongside the manifest.yaml. 
  required string readme = 3;
}

message CommitManifestsResponse {
}
```

### Use cases

#### Use case 1:

An organization with strong requirements around change auditing might enable manifest hydration in order to generate a full history of changes.

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
