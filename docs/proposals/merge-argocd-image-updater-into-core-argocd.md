---
title: Merge Argo CD Image Updater into core Argo CD codebase 
authors:
  - "@jaideepr97" # Authors' github accounts here.
sponsors:
  - "@wtam2018"   # List all interested parties here.
  - "@jannfis"
reviewers:
  - TBD
  - TBD
approvers:
  - TBD
  - TBD

creation-date: 2022-08-23
last-updated: 2022-08-24
---

# Merge Argo CD Image Updater into core Argo CD codebase

This proposal is aimed at presenting an approach towards adding first class support for Argo CD image updater into core Argo CD by merging the codebases of the 2 projects.

## Open Questions [optional]

- Do we want the image-updater to be a default workload?
- What can we do to support image updater in the Argo CD CLI and UI in the future?

## Summary

Draft PR to support this proposal: https://github.com/argoproj/argo-cd/pull/10446

## Motivation

The Argo CD Image Updater is a standalone project in the argoproj-labs org. It has developed a healthy amount of interest and users within the community. It solves a common and important use case for users by providing
a mechanism for users to have their workload images automatically updated and deployed on their clusters in a GitOps and Argo CD centric manner. This project provides great value to the users and the wider community, and 
would benefit from tighter coupling with core Argo CD by making it a first class controller alongside other Argo CD workloads

### Goals

One of the main goals of this proposal and the wider migration effort is to try and keep it simple by going step-by-step and refining/optimizing the code as we go, rather than trying to optimize everything on the first attempt
so things may be a little rough around the edges
- Layout proposed approach to merge Argo CD Image updater codebase
- Shed light on major aspects of the code changes involved 
- Discuss open questions 
- Reach consensus on implementation decisions

### Non-Goals

(Non goals for this proposal but relevant future goals for next phase of Image updater)
- Propose update to Argo CD application spec and status fields to create new fields to house image-updater configuration and status. Status would include information like last image attempted to update - which would enable 
  honoring application rollbacks in image-updater. 
- Add support for webhooks in image-updater and move away from periodic polling of image registries for updates
- Discuss eventual possibility of introducing a dedicated CRD for Image updater down the line, and its consequences 

**Update**: After discussing wiht the community, further future goals for image-updater after merge are:
- Instead of introducing another dedicated webhook for image-updater, create mechanism for image-updater and appset to communicate directly with repo-server to be aware of git based events
- Have image-updater, appset and notifications run within a single process for greater optimization of resoruces. 

## Proposal

### Use cases

As a user, I would like to be able to bridge the gap between CI and CD for my workload images natively within Argo CD (using Argo CD image updater)

### Implementation Details/Notes/Constraints [optional]

#### Summary of Directory structure and file changes involved
- Changes to Dockerfile to link argocd-image-updater binary to argocd binary
- Changes to procfiles to spin up image updater process
- Changes to Makefile to add new targets to build image-updater binary
- Add `cmd/argocd-iamge-updater` folder and add call to build image-updater start-up command. Supply "run" arg to start main Image updater loop. Retain support for `argocd-image-updater test` 
  and `argocd-image-udpater template` commands. Drop `argocd-image-updater version` command
- Add `manifests/base/image-updater` folder containing required manifests for deployment of image-updater resources. Add image-updater resources to `manifests/install.yaml` 
  (If Image updater should be a default workload - this is the case in associated draft PR)
- Update import references in various places
- Add Image updater's common constants to `common/common.go`
- Update Client in `util/git/client.go` to have additional methods for git writeback and add `writer.go` file from image-updater
- Add `image-updater` folder containing various packages that house core controller logic and helper methods (moved from `argocd-image-updater/pkg` almost as is)
- Add `docs/operator-manual/image-updater` folder containing all image-updater documentation files. Update navbar in `mkdocs.yml` to reference image-udpater pages
- Move example config maps and grafana-dashboards to `examples` folder.
- Add new e2e tests for image-updater in `tests` package (pending in PR)


Question: what ports should image-updater use? Currently hard-coded 6400 & 6401 for health and metrics

#### Handling git client code overlap
Image updater’s ext/git package borrows heavily from Argo CD’s util/git package for git operations. 
Image updater has a modified git client code and writer.go to also support write back to git. This includes additional methods to support committing and pushing to the repository 
During the migration we would only copy the additional writer.go file and modifications made to the git client in client.go
However, there have been changes to the git interfaces in Argo CD, specifically, Image updater is not aware of submodules and does not pass a `gitCredsStore` variable to any of its functions

Question: How should we handle these new parameters that are required by Argo CD's git interfaces but that Image updater is not aware of and does not care about?
(Associated draft PR currently just gets `submoduleEnabled` from the env var, and passes a new instance of askpass server)

#### Migration of e2e test suite
The image updater currently uses a kuttl test suite to run e2e tests in a declarative way. Merging code bases would mean the existing e2e tests would need to be converted into the custom testing framework
that is being used within Argo CD to achieve compatibility. (pending in related PR)

#### Deprecating existing GitHub repository
As a part of merging Image updater into Argo CD, we will need to inform the image updater community about this migration through a GitHub issue/discussion. We will also update documentation and README to indicate
that the development has shifted to the core Argo CD repository. Commit history likely doesn't need to be preserved. Existing issues/milestones can be moved to/created in the Argo CD repository for continued development
of those features. Existing PRs will need to be closed and re-worked against Argo CD.

What are the caveats to the implementation? What are some important details that didn't come across
above. Go in to as much detail as necessary here. This might be a good place to talk about core
concepts and how they relate.

You may have a work-in-progress Pull Request to demonstrate the functioning of the enhancement you are proposing.

### Detailed examples

### Security Considerations

* No obvious security implications to address

### Risks and Mitigations

Something to note: With the current proposal of just moving the `argocd-image-updater/pkg` folder to `image-updater` folder as is, there may be potential redundancy in utility packages such as `log`, `kube` etc. 
For now all the migrated pkg code is kept under the same folder for better separatability 

**Update**: After discussion with the community, it was agreed that we would like to avoid code duplication and redundancy, as such the associated PR should utilize existing util code packages for `log` and `kube` rather than introduce its own packages. 

### Upgrade / Downgrade Strategy

N/A

## Drawbacks

Additional complexity added to project, will require maintanance
