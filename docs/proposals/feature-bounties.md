---
title: Offering Feature Bounties (Experimental)
authors:
  - "@crenshaw-dev"
  - "@todaywasawesome"
sponsors:
  - "@jannfis"
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2023-06-27
---
# Offering Feature Bounties (Experimental)

## Summary
We'd like to have the ability to offer monetary rewards for significant features to be added to Argo. 

## Motivation
The Argo Project is driven by community contributions and in shared trust with maintainer companies. Sometimes there are important features worth investing in that represent substantial work and are tougher, or take longer to implement. 

By providing a financial incentive, we can spur additional development from the community and indepdent contributors. 

## Proposal
Add the ability to mark a proposal with a bounty and a specific amount. When a PR is successfully merged, release payment to the PR author(s). 

This proposal is experimental, meaning after trying a single bounty, we will review as a project and decide if we would like to continue this program. Accepting this proposal only constitutes the program for a single bounty as an experiment. 

### Guidelines and Rules

#### Creating a Bounty
A bounty is a special proposal created under `docs/proposals/feature-bounties`. 

* A bounty proposal may only be created by an existing Argo maintainer.
* The proposal document must be reviewed in regular maintainer meetings and an invitation for feedback will provide 7-days to comment.
* Bounty should have approval with [lazy-consensus](https://community.apache.org/committers/lazyConsensus.html)
* Once a bounty is created, they must be honored.
* Bounty progress will be tracked in a GitHub issue linked in the proposal.
* Creating a bounty requires the funds be available and not already committed elsewhere. 

#### Claiming a Bounty
* Argo will pay out bounties once a pull request implementing the requested features/changes/fixes is merged.
* A bounty is limited to a single successful PR.
* Those interested in working on the bounty are encouraged to comment on the issue, and users may team up to split a bounty if they prefer but collaboration is not required and users should not shame eachother for their preferences to work alone or together.
* A comment of interest does not constitute a claim and will not be treated as such.
* The first pull request submitted that is ready for merge will be reviewed by maintainers. Maintainers will also consider any competing pull requests submitted within 24-hours. We expect this will be a very rare circumstance. If multiple, high-quality, merge ready pull requests are submitted, 3-5 Approvers for the sub-project will vote to decide the final pull request merged.

### Funding
The Argo Project has a small amount of funds from HackerOne bounties that can provide for a few feature bounties. 