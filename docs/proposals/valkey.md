---
title: Replace Redis with ValKey
authors:
  - "@hairmare"
sponsors:
  - TBD        # List all interested parties here.
reviewers:
  - "@alexmt"
  - TBD
approvers:
  - "@alexmt"
  - TBD

creation-date: 2024-04-19
last-updated: 2024-04-19
---

# Replace ValKey with Redis

## Summary

Redis Inc announced that they are changing Redis'  license to the source available BUSL license [here](https://redis.com/blog/redis-adopts-dual-source-available-licensing/) which is in violation of [CNCF policy](https://github.com/cncf/foundation/blob/main/allowed-third-party-license-policy.md) and does not align with the Argo Projects values.

The BSD-3-Clause licensed Redis OSS code has now been forked as [ValKey](https://valkey.io/) in an effort hosted by the Linux Foundation.

## Motivation

Taking swift action when this kind of change happen in our supply chain is at the core of upholding the open source values of the Argo Project.

### Goals

* Redis is replaced with ValKey in the Argo CD project.
* This afford downstream distributions of Argo CD the security the security that they can follow Argo CD's lead

### Non-Goals

Replacing the Redis wire-protocol or any fundamental architecture changes are out of scope for this proposal.
It is assumed that ValKey will take over Redis' leading position and we are merely switch upstream with the community.

## Proposal

Given the nature of this change, fundamentally it should be as simple as replacing the container images in [manifests/base/redis](https://github.com/argoproj/argo-cd/tree/3a46e8c1c7dc20911cb5d87ade8ced26c766e273/manifests/base/redis).

Tracking ValKey's releases instead of Redis' and communicating the change to Argo CD's user base would also need to be taken care of.

### Implementation Details/Notes/Constraints [optional]

At the time of writing this proposal, a stable, generally available (GA) release of ValKey is available in the format needed update Argo CD's manifests.

For downstream Argo CD distributions additional work might be required. For example:

* There is no Bitnami Helm chart that the downstream Helm chart can use
* The RPM packages for an EL distributions are still quite fresh in EPEL

Given the documented non-goals above, this should be fine and help the ValKey project if anything.

### Security Considerations

This proposal aims to replace an important part of Argo CD's supply chain hence this change needs proper vetting and due diligence.

Some early efforts in this area have been made in a (now closed as not planned) [CNCF License Exception Request for Redis](https://github.com/cncf/foundation/issues/750).
The fact that the ValKey project is hosted by the Linux Foundation gives additional assurance.

### Risks and Mitigations

Even with it's history as Redis, the ValKey project is a young project that might not currently fulfill our maturity requirements to e.g. uphold Argo CD's SLSA status.

This need and the resulting requirements can be communicated to the upstream ValKey community and the Linux Foundation as host will also be in favor of and support these efforts.

### Upgrade / Downgrade Strategy

Both Redis and ValKey are currently compatible at the wire protocol level. Other implementations of the same protocol have always existed and will continue to do so. This includies Redis Inc's implementation which provides us an some fallback possibilities given the Redis version currently in use by Argo CD is not affected by the license change.

## Drawbacks

Not implementing this proposal could give downstream projects more agency and allow for them to pick a Redis replacement that fits their specific needs or stick with Redis.

## Alternatives

Other compatible implementations of the Redis wire protocol exist. The fact that ValKey is hosted by the Linux Foundation is an indicator that that helps us delegate this decision to the CNCF's parent foundation thus skipping the processes of analyzing competing offers like Redict, Garnet, or others.

The Argo CD project could also broaden the support matrix and implement support for several Redis wire protocol implementations. This option is not further explored in this proposal but could be considered for future work. The required effort is most likely prohibitive and this responsibilty could be taken over by a downstream project if needed.
