---
title: Neat-enhancement-idea
authors:
  - "@ishitasequeira" # Authors' github accounts here.
sponsors:
  - TBD        # List all intereste parties here.
reviewers:
  - "@jannfis"
  - "@crenshaw-dev"
  - "@alexmt"
approvers:
  - "@jannfis"
  - "@alexmt"
  - "@crenshaw-dev"

creation-date: 2022-01-28
last-updated: 2022-04-01
---

# Multiple Sources for application

Support more than one source for creating an Application.

Related Issues:
* [Proposal: Support multiple sources for an application](https://github.com/argoproj/argo-cd/issues/677)
* [Helm chart + values from Git](https://github.com/argoproj/argo-cd/issues/2789)

## Open Questions
* Adding external sources to the Application resource would add additional latencies for creation/reconciliation process. Should we add it to the doc in Risks?

## Summary

Currently, Argo CD supports Applications with only a single ApplicationSource. In certain scenarios, it would be useful to support more than one source for the application. For example, consider a need to create multiple deployments of the same application and manifests but manifests can come from different sources. Today we would have to copy manifest files from one application to another.

For example, from [one of the comments on this proposal PR](https://github.com/argoproj/argo-cd/pull/8322/files#r799624767)
```
An independent support of the Helm charts and their values files. This opens a door to such highly requested scenarios like multiple deployments of the same (possibly external) Helm chart with different values files or an independent migration to a newer Helm chart version for the same applications installed in Test and Production environments.
```

Creating applications from multiple sources would allow users to configure multiple services stored at various sources within the same application.

## Motivation

The main motivation behind this enhancement proposal is to allow users to create an application using services that are stored in various sources.

### Goals

The goals of the enhancement are:

#### **Supporting multiple sources for creating an application**

Users should be able to specify multiple sources to add services to the application. Argo CD should compile all the sources and reconcile each source individually for creating the application.

#### **Allow specifying external value files for Helm repositories**

Users should be able to specify different sources for Helm charts and values files. The Helm charts specified by the user could be available in Git or Helm repository and the value files are stored in Git. Argo CD should track changes in both the Helm charts and the value files repository and reconcile the application.

#### Changes to UI

The UI should allow users to add multiple sources while creating the application. For each source, UI should allow users to add multiple external values files Helm projects. We would need a separate proposal for changes to UI.

#### Changes to cli

The cli would need to support adding a list of resources instead of just one while creating the application. `values` field should allow referencing value files from other sources. We would need a separate proposal for changes to cli.

### Non-goals
*

## Proposal

### Add new `sources` field in Application Spec

The proposal is to add a new field `sources` which would allow users to input list of `ApplicationSource`s. We would mark field `source` as deprecated and would ignore the details under `source` with details under `sources` field.

Below example shows how the yaml would look like for `source` and `sources` field. We would ignore the `source` field and apply the resources mentioned under `sources` field.

```yaml
spec:
  source:
    repoURL: https://github.com/elastic/helm-charts/tree/main/elasticsearch
    targetRevision: 6.8
    helm:
      valueFiles:
        - values.yaml
  sources:                                          # new field
    - repoURL: https://github.com/helm/charts
      targetRevision: master
      path: incubator/elasticsearch
      helm:
        valueFiles:
          - values.yaml
```

### Make `path/chart` field optional

While adding sources to the application, users can decide not to add `path/chart` field in the application yaml. The controller will not generate manifests for the sources that do not have `path/chart` field specified. For example, in the below application spec, controller will generate the manifest for `elasticsearch` source but not for source `my-repo`.

```
spec:
  sources:
    - repoURL: https://github.com/my-org/my-repo # path is missing so no manifests are generated
      targetRevision: master
      ref: myRepo                                 # repo is available via symlink "my-repo"
    - repoURL: https://github.com/helm/charts
      targetRevision: master
      path: incubator/elasticsearch               # path "incubator/elasticsearch" is used to generate manifests
      helm:
        valueFiles:
          - $myRepo/values.yaml                   # values.yaml is located in source with reference name $myRepo
```

### Add optional `ref` field to Application Source

For making files accessible to other sources, add a new `ref` field to the source. For example, in below ApplicationSpec, we have added `ref: myRepo` to the `my-repo` repository and have used reference `$myRepo` to the `elasticSearch` repository.

```yaml
spec:
  sources:
    - repoURL: https://github.com/my-org/my-repo  # path is missing so no manifests are generated
      targetRevision: master
      ref: myRepo                                 # repo is available via symlink "myRepo"
    - repoURL: https://github.com/helm/charts
      targetRevision: master
      path: incubator/elasticsearch               # path "incubator/elasticsearch" is used to generate manifests
      helm:
        valueFiles:
          - $myRepo/values.yaml                   # values.yaml is located in source with reference name $myRepo
```

### Combined Example Application yaml

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: grafana
  namespace: argocd
spec:
  destination:
    namespace: monitoring
    server: https://some.k8s.url.com:6443
  project: default
  source:
    repoURL: https://github.com/helm/charts
    targetRevision: master
    helm:
    valueFiles:
        - values.yaml
    chart: incubator/elasticsearch
  sources:                                          # new field
  # application that consists of MongoDB and ElasticSearch resources
    - repoURL: https://github.com/helm/charts
      targetRevision: master
      path: incubator/mongodb
    - repoURL: https://github.com/helm/charts
      targetRevision: master
      path: incubator/elasticsearch
    - repoURL: https://github.com/my-org/my-repo  # path is missing so no manifests are generated
      targetRevision: master
      ref: myRepo                                 # repo is available via symlink "my-repo"
    - repoURL: https://github.com/helm/charts
      targetRevision: master
      path: incubator/elasticsearch               # path "incubator/elasticsearch" is used to generate manifests
      helm:
        valueFiles:
          - $myRepo/values.yaml                   # values.yaml is located in source with reference name $myRepo
  syncPolicy:
    automated: {}
```

In scenarios, where you have same resource mentioned multiple times in the application yaml, the last resource in the source list will override the previous resources.

### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

#### Use case 1:
As a user, I have an Application that uses the [elasticsearch](https://github.com/helm/charts/tree/master/incubator/elasticsearch) helm chart as source. Today, user needs to create a separate Application to configure the [elasticsearch-exporter](https://github.com/helm/charts/tree/master/stable/elasticsearch-exporter
) to expose Prometheus metrics.
https://github.com/argoproj/argo-cd/issues/677

#### Use case 2:
As per one of the [comment]((https://github.com/argoproj/argo-cd/issues/2789#issuecomment-562495307)) on the issue [Helm chart + values files from Git](https://github.com/argoproj/argo-cd/issues/2789):
```
We have a Helm Chart which is used in 30+ Services and each of them is customized for 3 possible environments.
Replicating this Chart 30 times without a centralized Repo looks dirty. Can be a show stopper for the whole migration.
Modifying the Application definition is not an option since the whole goal is to reduce the rights that the CI-solution has. Giving it the right to update all Application-definitions from various teams in the argocd namespace is a a hard thing to convince people with.
```

### Implementation Details

#### Attach multiple sources to the Application

To allow multiple sources to the Application, we would add a new field `sources` which would allow users to input list of `ApplicationSource`s. As part of this update and to support backward compatibility, we would mark field `source` as deprecated and remove it in later revisions.

To avoid complexity on the deciding the list of sources, if both `source` and `sources` fields are specified, we would override the source under `source` field with all the sources under `sources` field.

**Depracating `source` field:** - Once the `sources` field is implemented in UI and cli as well, we will mark the `source` field as deprecated. At the same time, we will log `WARNING` messages and add UI blurbs about the deprecation. When maintainers feel confident about the adoption of the `sources` field, we will remove the `source` field from later releases.

#### Invalidating existing cache

Argo CD benefits from the assumption of a single repo per application to detect changes and to cache the results. But this enhancement now requires us to look at all the source repo "HEAD"s and invalidate the cache if any one of them changes.

#### Reconcilation of the Application

As we would have multiple sources as part of the same Application, we would need to track updates to each source and reconcile the Application for each source. When one of the sources change, we would need to ensure that target revisions of other sources are up-to-date, e.g. force a simple refresh to see if target revision of the source (e.g. HEAD), still points to revisionX for example.

#### Updates to UI
Today, we allow users to add only one source in the UI. We would need to update the UI to add multiple sources and configure specific

#### Updates to cli

We would need to create new options to the `argocd app create` command for adding multiple sources to the Application. We would also need to introduce allowing `ref` field for sources and to reference the files from symlinked source.

As per the community call on February 3, changes to UI and cli are huge and are not planned to be part of first iteration.

### Security Considerations

Good unit- and end-to-end tests need to be in place for this functionality to ensure we don't accidentally introduce a change that would break the feature.

### Risks and Mitigations

#### Uncontrolled number of sources added to Application

The users might add a huge number of external sources to the Application, with a potential performance impact on the Argo CD creation/reconciliation processes. This would apply even for the external value files for Helm projects.

A possible mitigation to this would be to enforce the number of external sources allowed per Application.

#### Unauthorised access to external resources

The users might reference the source that has not been whitelisted in the project. This might lead to access issues and failure to sync the Application.

A possible solution would be to check for the source repository to be whitelisted in the project before syncing and report appropriate error messages in respective scenarios.


### Upgrade / Downgrade Strategy

Upgrading to a version implementing this proposal should be frictionless and wouldn't require administrators to perform any changes in the configuration to keep the current behaviour. Application created without the new field `.spec.sources` being set will keep their behaviour, since they will allow Application resources to be created the same way they are today.

Downgrading would not be easily possible once users start to make use of the feature and create Applications with the new field `.spec.sources` being set. The Application would no longer be able to recognize the resources and will fail the reconciliation/creation step.


## Drawbacks

* Downgrade/rollback would not be easily possible
