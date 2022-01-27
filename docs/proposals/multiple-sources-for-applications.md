---
title: Neat-enhancement-idea
authors:
  - "@ishitasequeira" # Authors' github accounts here.
sponsors:
  - TBD        # List all intereste parties here.
reviewers:
  - "@jannfis"
  - TBD
approvers:
  - "@jannfis"
  - TBD

creation-date: 2022-01-28
last-updated: 2022-01-28
---

# Multiple Sources for application

Support more than one source for creating an Application.

Related Issues: 
* https://github.com/argoproj/argo-cd/issues/677
* https://github.com/argoproj/argo-cd/issues/2789

## Open Questions
* Adding external sources to the Application reource would add additional latencies for creation/reconcilation process. Should we add it to the doc in Risks?
* 

## Summary

Currently, Argo CD supports Applications with only a single ApplicationSource. In certain scenarios, it would be useful to support more than one source for the application. For example, consider a need to create multiple applications with the same manifests. Today we would have to copy manifest files from one application to another.

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

The UI should allow users to add multiple sources while creating the application. For each source, UI should allow users to add multiple external values files Helm projects.

#### Changes to cli

The cli would need to support adding a list of resources instead of just one while creating the application. Also, just like `--values` field for adding values files, we would need an option to allow external value files to the application.

### Non-goals
* Allow reconciliation from Argo CD Application resources that are located at various sources. We believe this would be possible with some adaptions in the reconcilation workflow.

## Proposal

### Add new `sources` field in Application Spec

The proposal is to add a new field `sources` which would allow users to input list of `ApplicationSource`s. We would mark field `source` as deprecated and would override the details under `source` with details under `sources` field.

```yaml
spec:
  source:
    repoURL: https://charts.bitnami.com/bitnami
    targetRevision: 8.5.8
    helm:
    valueFiles:
        - values.yaml
    chart: mysql
  sources:                                          # new field
    - repoURL: https://charts.bitnami.com/bitnami
      targetRevision: 8.5.8
      helm:
        valueFiles:
          - values.yaml
      chart: mysql
```

### Add `externalValues` field in helm section of Application Spec

Along with new `sources` field, add a new field for accepting external value files `externalValueFiles` under the `helm` section of each ApplicationSource.

```yaml
sources:
    - repoURL: https://charts.bitnami.com/bitnami
      targetRevision: 8.5.8
      helm:
        valueFiles:
          - values.yaml
        externalValueFiles:                         # new field
          - repoURL: https://github.com/KaiReichart/argo-test-values.git
            targetRevision: main
            valueFiles:
              - values.yaml
      chart: mysql
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
    repoURL: https://charts.bitnami.com/bitnami
    targetRevision: 8.5.8
    helm:
    valueFiles:
        - values.yaml
    chart: mysql
  sources:                                          # new field
    - repoURL: https://charts.bitnami.com/bitnami
      targetRevision: 8.5.8
      helm:
        valueFiles:
          - values.yaml
        externalValueFiles:                         # new field
          - repoURL: https://github.com/KaiReichart/argo-test-values.git
            targetRevision: main
            valueFiles:
              - values.yaml
      chart: mysql
    - repoURL: https://charts.bitnami.com/bitnami1
      targetRevision: 8.5.8
      helm:
        valueFiles:
          - values.yaml
        externalValueFiles:
          - repoURL: https://github.com/KaiReichart/argo-test-values.git
            targetRevision: main
            valueFiles:
              - values.yaml
      chart: mysql
  syncPolicy:
    automated: {}
```

### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

#### Use case 1: 
As a user, I have an Application that uses the [elasticsearch](https://github.com/helm/charts/tree/master/incubator/elasticsearch) helm chart as source. Today, user needs to create a separate Application to configure the [elasticsearch-exporter](https://github.com/helm/charts/tree/master/stable/elasticsearch-exporter
) to expose Prometheus metrics.
https://github.com/argoproj/argo-cd/issues/677

#### Use case 2: 
We have a Helm Chart which is used in 30+ Services and each of them is customized for 3 possible environments.
Replicating this Chart 30 times without a centralized Repo looks dirty. Can be a show stopper for the whole migration.
Modifying the Applica`tion definition is not an option since the whole goal is to reduce the rights that the CI-solution has. Giving it the right to update all Application-definitions from various teams in the argocd namespace is a a hard thing to convince people with.


### Implementation Details

#### Attach multiple sources to the Application

To allow multiple sources to the Application, we would add a new field `sources` which would allow users to input list of `ApplicationSource`s. As part of this update and to support backward compatibilty, we would mark field `source` as deprecated and remove it in later revisions.

To avoid complexity on the deciding the list of sources, if both `source` and `sources` fields are specified, we would override the source under `source` field with all the sources under `sources` field.

#### Invalidating existing cache

Argo CD benefits from the assumption of a single repo per application to detect changes and to cache the results. But this enhancement now requires us to look at all the source repo "HEAD"s and invalidate the cache if any one of them changes.

#### Reconcilation of the Application

As we would have multiple sources as part of the same Application, we would need to track updates to each source and reconcile the Application for each source. When one of the sources change, we would need to ensure that target revisions of other sources are up-to-date, e.g. force a simple refresh to see if target revision of the source (e.g. HEAD), still points to revisionX for example.

#### Updates to UI
Today, we allow users to add only one source in the UI. We would need to update the UI to add multiple sources and configure specific 

#### Updates to cli

We would need to create new options to the `argocd app create` command for adding multiple sources to the Application.

For supporting external Helm value files, we would need to introduce a new option similar to existing `--values` option for Helm projects to support external values files. This options would need to be added individually to each source.


### Security Considerations

Good unit- and end-to-end tests need to be in place for this functionality to ensure we don't accidentally introduce a change that would allow any form of uncontrolled association between an Application and its resources.

### Risks and Mitigations

#### Uncontrolled number of sources added to Application

The users might add a huge number of external sources to the Application, with a potential performance impact on the Argo CD creation/reconcilation processes. This would apply even for the external value files for Helm projects.

A possible mitigation to this would be to enforce the number of external sources allowed per Application.

#### Unauthorised access to external resources

The users might reference the source that has not been whitelisted in the project. This might lead to access issues and failure to sync the Application.

A possible solution would be to check for the source repository to be whitelisted in the project before syncing and report appropriate error messages in respective scenarios.


### Upgrade / Downgrade Strategy

Upgrading to a version implementing this proposal should be frictionless and wouldn't require administrators to perform any changes in the configuration to keep the current behaviour. Application created without the new field `.spec.sources` being set will keep their behaviour, since they will allow Application resources to be created the same way they are today.

Downgrading would not be easily possible once users start to make use of the feature and create Applications with the new field `.spec.sources` being set. The Application would no longer be able to recognize the resources and will fail the reconcilation/creation step.


## Drawbacks

* Downgrade/rollback would not be easily possible

## Alternatives

