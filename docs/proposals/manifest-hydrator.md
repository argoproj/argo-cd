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

## Terms

* dry manifests: DRY or Don't Repeat Yourself - things like Kustomize overlays and Helm charts that produce Kubernetes manifests but are not themselves Kubernetes Manifests
* hydrated manifests: the output from dry manifest tools, i.e. plain Kubernetes manifests

## Summary

Manifest hydration tools like Helm and Kustomize are indispensable in GitOps. These tools transform "dry" (Don't Repeat Yourself) sources into plain Kubernetes manifests. The effects of a change to dry sources are not always obvious. So storing only dry sources in git leaves the user with an incomplete and confusing history of their application. This undercuts some of the main benefits of GitOps.

The "rendered manifests" pattern has emerged as a way to mitigate the downsides of using hydration tools in GitOps. Today, developers use CI tools to automatically hydrate manifests and push to separate branches. They then configure Argo CD to deploy from the hydrated branches. (For more information, see the awesome [blog post](https://akuity.io/blog/the-rendered-manifests-pattern/) and [ArgoCon talk](https://www.youtube.com/watch?v=TonN-369Qfo) by Nicholas Morey.)

This proposal describes manifest hydration and pushing to git as a first-class feature of Argo CD.

It offers two modes of operation: push-to-deploy and push-to-stage. In push-to-deploy, hydrated manifests are pushed to the same branch from which Argo CD deploys. In push-to-stage, manifests are pushed to a different branch, and Argo CD relies on some external system to move changes to the deployment branch; this provides an integration point for automated environment promotion systems.

### Opinions

This proposal is opinionated. It is based on the belief that, in order to reap the full benefits of GitOps, every change to an application's desired state must originate from a commit to a single GitOps repository. In other words, the full history of the application's desired state must be visible as the commit history on a git repository.

This requirement is incompatible with tooling which injects nondeterministic configuration into the desired state before it is deployed by the GitOps controller. Examples of nondeterministic external configuration are:

1) Helm chart dependencies on unpinned chart versions
2) Kustomize remote bases to unpinned git revisions
3) Config tool parameter overrides in the Argo CD Application `spec.source` fields
4) Multiple sources referenced in the same application (knowledge of combination of source versions is held externally to git)

Injecting nondeterministic configuration makes it impossible to know the complete history of an application by looking at a git branch history. Even if the nondeterministic output is databased (for example, in a hydrated source branch in git), it is impossible for developers to confidently make changes to desired state, because they cannot know ahead of time what other configuration will be injected at deploy time.

We believe that the problems of injecting external configuration are best solved by asking these two questions:

1) Does the configuration belong in the developer's interface (i.e. the dry manifests)?
2) Does the configuration need to be mutable at runtime, or only at deploy time?

If the configuration belongs in the developer's interface, write a tool to push the information to git. Image tags are a good example of such configuration, and the Argo CD Image Updater is a good example of such tooling.

If the configuration doesn't belong in the developer's interface, and it needs to be updated at runtime, write a controller. The developer shouldn't be expected to maintain configuration which is not an immediate part of their desired state. An example would be an auto-sizing controller which eliminates the need for the developer to manage their own autoscaler config.

If the configuration doesn't belong in the developer's interface and doesn't need to be updated at runtime (only at deploy time), write a mutating webhook. This is a great option for injecting cluster-specific configuration that the developer doesn't need to directly control.

With these three options available (git-pushers, controllers, and mutating webhooks), we believe that it is not generally necessary to inject nondeterministic configuration into the manifest hydration process. Instead, we can have a full history of the developer's minimal intent (dry branch) and the full expression of that intent (hydrated branch) completely recorded in a series of commits on a git branch.

By respecting these limitations, we unlock the ability to manage change promotion/reversion entirely via git. Change lineage is fully represented as a series of dry commit hashes. This makes it possible to write reliable rules around how these hashes are promoted to different environments and how they are reverted (i.e. we can meaningfully say "`prod` may never be more than one dry hash ahead of `test`"). If information about the lineage of an application is scattered among multiple sources, it is difficult or even impossible to meaningfully define rules about how one environment's lineage must relate to that of another environment.

Being opinionated unlocks the full benefits of GitOps as well as the ability to build a reasonable, reliable preview/promotion/reversion system. 

These opinions will lock out use cases where configuration injection cannot be avoided by writing git-pushers, controllers, or mutating webhooks. We believe that the benefits of making an opinionated system outweigh the costs of compromising those opinions.

## Motivation

Many organizations have implemented their own manifest hydration system. By implementing it in Argo CD, we can lower the cost to our users of maintaining those systems, and we can encourage best practices related to the pattern.

### Goals

1) Make manifest hydration easy and intuitive for Argo CD users
2) Make it possible to implement a promotion system which relies on the manifest hydration's push-to-stage mode
3) Emphasize maintaining as much of the system's state as possible in git rather than in the Application CR (e.g. source hydrator config values, such as Helm values)
4) Every deployed change must have a corresponding dry commit - i.e. git is always the source of any changes
5) Developers should be able to easily reproduce the manifest hydration process locally, i.e. by running some commands

#### Hydration Reproducibility

One goal of this proposal is to make hydration reproducibility easy. Reproducibility brings a couple benefits: easy iteration/debugging and reliable previews.

##### Easy Iteration/Debugging

The hydration system should enable developers to easily reproduce the hydration process locally. The developer should be able to run a short series of commands and perform the exact same tasks that Argo CD would take to hydrate their manifests. This allows the developer to verify that Argo CD is behaving as expected and to quickly tweak inputs and see the results. This lets them iterate quickly and improves developer satisfaction and change velocity.

To provide this experience, the hydrator needs to provide the developer with a few pieces of information:

1) The input repo URL, path, and commit SHA
2) The hydration tool CLI version(s) (for example, the version of the Helm CLI used for hydration)
3) A series of commands and arguments which the developer can run locally

Equipped with this information, the developer can perform the exact same steps as Argo CD and be confident that their dry manifest changes will produce the desired output.

Ensuring that hydration is deterministic assures the developer that the output for a given dry state will be the same next week as it is today.

###### Avoiding Esoteric Behavior

We should avoid the developer needing to know Argo CD-specific behavior in order to reproduce hydration. Tools like Helm, Kustimize, etc. have excellent public-facing documentation which the developer should be able to take advantage of without needing to know quirks of Argo CD.

##### Reliable Previews

Deterministic hydration output allows Argo CD to produce a reliable change preview when a developer proposes a change to the dry manifests via a PR.

If output is not deterministic, then a preview generated today might not be valid/correct a week, day, or even hour later. Non-determinism makes it so that developers can't trust that the change they review will be the change actually applied.

### Non-Goals

1) Implementing a change promotion system

## Open Questions

* The `sourceHydrator` field is mutually exclusive with the `source` and the `sources` field. Should we throw an error if they're both configured, or should we just pick one and ignore the others?
* How will/should this feature relate to the image updater? Is there an opportunity to share code, since both tools involve pushing to git?
* Should we enforce a naming convention for hydrated manifest branches, e.g. `argo/...`? This would make it easier to recommend branch protection rules, for example, only allow pushes to `argo/*` from the argo bot.
* Should we enforce setting a `sourceHydrator.syncSource.path` to something besides `.`? Setting a path makes it easier to add/remove other apps later if desired.

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
    syncSource:
      targetBranch: environments/e2e
      path: .
    # The hydrateTo field is optional. If specified, Argo CD will write hydrated manifests to this branch instead of the
    # syncSource.targetBranch. This allows the user to "stage" a hydrated commit before actually deploying the changes
    # by merging them into the syncSource branch. A complete change promotion system can be built around this feature. 
    hydrateTo:
      targetBranch: environments/e2e-next
      # The path is assumed to be the same as that in syncSource.
```

When the Argo CD application controller detects a new commit on the `drySource`, it queue up the hydration process.

When the application controller detects a new (hydrated) commit on the `syncSource.targetBranch`, it will sync the manifests.

### Processing a New Dry Commit

On noticing a new dry commit, Argo CD will first collect all Applications which have the same `drySource` repo and targetRevision.

Argo CD will then group those sources by the configured `syncSource` targetBranch.

```go
package hydrator

import "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

type DrySource struct {
	repoURL        string
	targetRevision string
}

type SyncSource struct {
	targetBranch string
}

var appGroups map[DrySource]map[SyncSource][]v1alpha1.Application
```

Then Argo CD will loop over the apps in each group. For each group, it will run manifest hydration on the configured `drySource.path` and write the result to the configured `syncSource.path`. After looping over all apps in the group and writing all their manifests, it will commit the changes to the configured `syncSource` repoURL and targetBranch (or, if configured, the `hydratedTo` targetBranch). Finally, it will push those changes to git. Then it will repeat this process for the remaining groups.

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
    syncSource:
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
    syncSource:
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
    syncSource:
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
    syncSource:
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
    syncSource:
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
    syncSource:
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

### `spec.destination.namespace` Behavior

The Application `spec.destination.namespace` field is used to set the `metadata.namespace` field of any namespace resources for which that field is not set in the manifests.

The hydrator will not inject `metadata.namespace` into the hydrated manifests pushed to git. Instead, Argo CD's behavior of injecting that value immediately before applying to the cluster will continue to be used with the `spec.sourceHydrator.syncSource`.

### Build Environment Support

For sources specified in `spec.source` or `spec.sources`, Argo CD [sets certain environment variables](https://argo-cd.readthedocs.io/en/stable/user-guide/build-environment/) before running the manifest hydration tool.

Some of these environment variables may change independently of the dry source and therefore break the reproducibility of manifest hydration (see the [Opinions](#opinions) section). Therefore, only some environment variables will be populated for the `spec.sourceHydrator` source.

These environment variables will **not** be set:

* `ARGOCD_APP_NAME`
* `ARGOCD_APP_NAMESPACE`
* `KUBE_VERSION`
* `KUBE_API_VERSIONS`

These environment variables will be set because they are commit SHAs and are directly and immutably tied to the dry manifest commit:

* `ARGOCD_APP_REVISION`
* `ARGOCD_APP_REVISION_SHORT`

These environment variables will be set because they are inherently tied to the manifest hydrator configuration. If these fields set in `spec.sourceHydrator.drySource` change, we are breaking the connection to the original hydrator configuration anyway.

* `ARGOCD_APP_SOURCE_PATH`
* `ARGOCD_APP_SOURCE_REPO_URL`
* `ARGOCD_APP_SOURCE_TARGET_REVISION`

### Support for Helm-Specific Features

#### App Name / Release Name

By default, Argo CD's `source` and `sources` fields use the Application's name as the release name when hydrating Helm manifests.

To centralize the source of truth when using `spec.sourceHydrator`, the default release name will be an empty string, and any different release name should be specified in the `helm.releaseName` field in `.argocd-source.yaml`.

#### Kube API Versions

`helm install` supports dynamically reading Kube API versions from the destination cluster to adjust manifest output. `helm template` accepts a list of Kube API versions to simulate the same behavior, and Argo CD's `spec.source` and `spec.sources` fields set those API versions when running `helm template`.

To centralize the source of truth when using `spec.sourceHydrator`, the Kube API versions will not be populated by default.

Instead, a new field will be added to the Application's `spec.source.helm` field:

```yaml
kind: Application
spec:
  source:
    helm:
      apiVersions:
        - admissionregistration.k8s.io/v1/MutatingWebhookConfiguration
        - admissionregistration.k8s.io/v1/ValidatingWebhookConfiguration
        - ... etc.
```

That field will also be available in `.argocd-source.yaml`:

```yaml
helm:
  apiVersions:
    - admissionregistration.k8s.io/v1/MutatingWebhookConfiguration
    - admissionregistration.k8s.io/v1/ValidatingWebhookConfiguration
    - ... etc.
```

So the appropriate way to set Kube API versions for the source hydrator will be to populate the `.argocd-source.yaml` file.

#### Hydrated Environment Branches

Representing the dry manifests of environments as branches has well-documented downsides for developer experience. Specifically, it's toilsome for developers to manage moving changes from one branch to another and avoid drift.

So environments-as-directories has emerged as the standard for good GitOps practices. Change management across directories in a single branch is much easier to perform and reason about.

**This proposal does not suggest using branches to represent the dry manifests of environments.** As a matter of fact, this proposal codifies the current best practice of representing the dry manifests as directories in a single branch.

This proposal recommends using different branches for the _hydrated_ representation of environments only. Using different branches has some benefits:

1) Intuitive grouping of "changes to ship at once" - for example, if you have app-1-east and app-1-west, it makes sense to merge a single hydrated PR to deploy to both of those apps at once
2) Easy-to-read history of a single environment via the commits history
3) Easy comparison between environments using the SCMs' "compare" interfaces

In other words, branches make a very nice _read_ interface for _hydrated_ manifests while preserving the best-practice of using _directories_ for the _write_ interface.

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

To request a commit to the hydrated branch, the application controller will make a call to the CommitManifests service.

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

### Push access

The hydrator will need to push to the git repository. This will require a secret containing the git credentials.

Write access will be configured via a Kubernetes secret with the following structure:

```yaml
apiVersion: v1
kind: Secret
metadata:
  labels:
    argocd.argoproj.io/secret-type: repository-write
stringData:
  url: 'https://github.com/argoproj/argocd-example-apps'
  githubAppID: '123456'
  githubInstallationID: '123456'
  githubAppPrivateKey: |
    -----
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
