---
title: Argo CD Scalability Benchmarking
authors:
  - "@andklee"
  - "@csantanapr"
  - "@morey-tech"
  - "@nimakaviani"
sponsors:
  - 
reviewers:
  - 
  - 
approvers:
  - 
  - 
creation-date: 2023-02-28
last-updated: 2023-02-28
---

# Argo CD Scalability Benchmarking

## Open Questions [optional]
* What are the scalability factors to prioritize when benchmarking Argo CD?
    * Reconciliation time for all, one, or several applications in the cluster.

## Motivation
Users of Argo CD are interested to know how to scale Argo CD, what configuration tweaks and deployment options they have, and how far they can push resources (in terms of the number of supported applications, Git repositories, managing clusters, etc.).

While the Argo CD documentation [discusses options](https://argo-cd.readthedocs.io/en/stable/operator-manual/high_availability/#scaling-up) to scale up, the actual process is not clear and, as articulated [in this thread](https://github.com/argoproj/argo-cd/issues/9633), oftentimes a point of confusion for users.

By running large-scale benchmarking, we aim at helping the Argo CD community with the following:

* Give confidence to organizations running at a significant scale (_needs to be quantified_) that Argo CD can support their use-case, with empirical evidence.
* Create clear guidelines for scaling Argo CD based on the key scalability factors.
* Provide recommendations for which topology is best suited for users based on their needs.
* Determine what work, if any, is needed to improve the scalability of Argo CD.

### Goals
1. Create a standard set of repeatable benchmarking procedures to objectively measure the limitations of Argo CD.
    1. This may result in a new `argo-cd-benchmarking` repo under `argoproj-labs` so that anyone can easily replicate it and the development of the procedures can happen outside of the lifecycle for Argo CD (unlike [the current `gen-resources` hack](https://github.com/argoproj/argo-cd/tree/master/hack/gen-resources) in the main project).
    2. Include detailed test scenarios that account for key scalability factors that allow for easy tweaking of the parameters to simplify testing of alternative scenarios.
2. Determine the baseline for when tweaking is required on the default configuration (resource allocations and replicas).
    1. One cluster, Applications in In-cluster, default resource allocations.
3. Quantify how tweaking existing parameters (replicas, sharding, parallelism, etc) impacts the performance of Argo CD.
4. Provide a set of metrics and thresholds that will provide a basis for automatically scaling the Argo CD components and alerting for when performance is being impacted by limitations.
5. All tooling, recommendations, examples, and scenarios will be vendor-agnostic.
### Non-Goals
* This proposal does not intend to cover the implementation of any auto-scaling based on the metrics and thresholds determined by the scalability benchmarking. A separate proposal will be used for any auto-scaling enhancements.
* This proposal does not intend to add testing that determines how a change impacts the scalability of Argo CD based on the benchmarks.
* We do not intend to analyze the cost implications of running different topologies and purely focus on scalability requirements from a technology perspective.

### Initial Members
The initial members for this proposal and their affiliations are:
| Name                                                | Company   |
|-----------------------------------------------------|-----------|
| [Andrew Lee](https://github.com/andklee)            | AWS       |
| [Carlos Santana](https://github.com/csantanapr)     | AWS       |
| [Nicholas Morey](https://github.com/morey-tech)     | Akuity    |
| [Nima Kaviani](https://github.com/nimakaviani)      | AWS       |

With the introduction of [the proposed Scalability SIG](https://github.com/argoproj/argoproj/pull/192), the members participating in the proposal may change.

Any community member is welcome to participate in the work for this proposal. Either by joining the Scalability SIG or through contributing to the proposed `argoproj-labs/argo-cd-benchmarking` repository containing the tooling.

## Proposal
1.  Create new `argo-cd-benchmarking` repo under `argoproj-labs` and add the authors of this proposal as maintainers.
2. Create a set of key scalability factors to use as testing parameters. For example:
    1. Number of Applications.
    2. Number of resources managed by an Application.
    3. Number of resources in a cluster.
    4. The size of the resources in a cluster and managed by an Application.
    5. Churn rate for resources in the cluster (how often resources change).
    6. Number of clusters.
    7. Number of repositories being monitored.
    8. Size of the repositories.
    9. The tooling (e.g., directory/raw manifests, Helm, Kustomize).
3. Determine the metrics that reflect limitations in scalability factors.
    1. Application sync time for x number of apps
    2. Emptying the queues (app_reconciliation_queue, app_operation_processing_queue)
4. Create automated testing procedures for Argo CD that take the key scalability factors as testing parameters.
5. Test the default installation of Argo CD to determine the limit based on the key scalability factors.
6. Create test scenarios that reflect the common topologies (Argo CD 1-1 with clusters, Argo CD 1-many with clusters).
7. Determine the thresholds for the metrics identified earlier to capture when performance is being impacted.
    1. Contribute back Grafana thresholds and alerts for Prometheus

### Use cases
Each use case will cover a specific topology with N permutations based on the key scalability factors. They will be measured using the metrics given in the proposal.

We intend to focus on two key topologies: one Argo CD per cluster and one Argo CD for all remote clusters. All other variations are an extension of these.

#### Topology 1: One Argo CD per cluster.
The exact key scalability factors used in each permutation will be determined once we get to the testing.

#### Topology 2: One Argo CD for all remote clusters.
This will capture the impact of network throughput on performance.

#### Topology 3: One Namespaced Argo CD.
This instance will only manage namespace-level resources to determine the impact of monitoring cluster-scoped resources (related to the effect of resource churn in the cluster).

### Implementation Details/Notes/Constraints [optional]
There is already some [tooling in the Argo CD repository](https://github.com/argoproj/argo-cd/pull/8037/files) for scalability testing. We plan to build on the existing effort and further push the boundaries of testing it.

* Automatically set up supporting tooling for capturing metrics (Grafana, Prometheus)
* Use a local Gitea in the cluster to support many repositories used in testing, and avoid performance variance by depending on the performance of an external git SaaS (ie GitHub)
* Simulate the cluster and nodes using vcluster or kwok, in addition to using a cloud provider with a real cluster fleet.
    * The simulated clusters and nodes are intended to make the testing accessible, but ultimately the infrastructure should be easily changed to test more realistic scenarios. Once the benchmarking tooling is functional, we can determine if the simulated components skew the results.

Once we have the benchmarking tooling, we can determine if the simulated components skew the results compared to the real world.

AWS intends to provide the infrastructure required to benchmark large-scale scenarios.

### Security Considerations
There is no intention to change the security model of Argo CD and therefore this project has no direct security considerations.

### Risks and Mitigations

## Consider including folks that also work outside your immediate sub-project.

## Drawbacks

## Alternatives
* Implementing Argo CD into an environment than waiting for scaling issues to arise. Monitoring the metrics to understand what the limitations are to address them. Using arbitrary resource allocation and replica counts to avoid running into limitations.
