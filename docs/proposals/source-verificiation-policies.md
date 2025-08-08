---
title: Source Verification Policies
authors:
  - "@jannfis" # Authors' github accounts here.
sponsors:
  - TBD        # List all interested parties here.
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2023-08-08
last-updated: 2023-08-08
---

# Source Verification Policies

This proposal introduces an evolution to the existing GnuPG commit verification feature.

## Open Questions [optional]

* Do we really need `progressive` verification level (see below)?
* Should repository pattern matching be regexp based instead of shell-glob?

## Summary

Argo CD has had a feature to verify OpenPGP signatures on Git commits using GnuPG for a while. However, this feature is limited and in some ways may behave unexpectedly. Argo CD would ever only verify the signature on the commit that's pointed to by the `HEAD` of the `Application`'s resolved `targetRevision`. This allows for unsigned commits to slip in, and requires thorough verification of the source by the user prior to signing a commit.

Source verification policies are an evolution of the legacy signature verification in Argo CD. A source verification policy defines the cryptographic requirements to be met in order for a source to be allowed to sync, and with it, brings new verification levels, the possibility of treating multiple sources in an Application with different policies and it sets the foundation for the ability to implement more verification methods in the future.

## Motivation

As a deployment tool, Argo CD sits in a prominent spot to enforce secure supply chain requirements. The cryptographic verification of sources and artifacts become more and more important to organisations and are heavily regulated in some fields.

While having Argo CD verifying the `HEAD` commit to be synced is of course better than nothing, it is not a sufficient level of verification for many organisations and individuals anymore.

Also, the current approach is an "all-or-nothing" approach for any given AppProject. The reality however is, that different repositories might have different trust levels, and different people working on them and being trusted. Especially with the advent of multi-source applications, the current approach of defining the trusted signers for all Applications in a given AppProject is not viable anymore.

### Goals

* Increase confidence in Git commit verification
* Attach signer trust to sources, instead of at the app project level
* Properly support multi-source applications as well as unique settings per source in multi-source applications
* Be flexible enough to support other verification providers (e.g. Helm provenance, Git signed using sigstore, etc)
* Make it easy for users to migrate to a more secure verification policy
* Fully backwards compatibility to existing GnuPG commit verification mechanisms

### Non-Goals

* Implement other verification methods than GnuPG for Git commits. This may come in at a later stage, and the design of SVPs would allow those to integrate rather easily.

## Proposal


### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

#### Use case 1:

As an Argo CD user, I would like to ensure that my applications only syncs if all commits in the history of the repository that led to the application's target revision has been cryptographically signed by a trusted party.

#### Use case 2:

As an Argo CD user, I need to apply a different level of trust to different source repositories, especially with multi-source applications.

## Implementation Details/Notes/Constraints

There is a PoC implementation for this proposal, which is linked from the issue's description. This section describes the inner-workings of that PoC as envisioned by the authors.

Under the hood, getting the revision history is performed using [git rev-list](https://git-scm.com/docs/git-rev-list), which can be consulted for more information.

### Overview

Source verification policies are configured in the `sourceVerificationPolicies` field of an AppProject's spec. The field itself is a list, and you can configure an arbitrary amount of source verification policies.

Each entry in the list has the following mandatory fields:

* `repositoryPattern` defines a glob-style pattern that an Application's source repository URL must be matched against in order for the policy to apply
* `verificationLevel` is the verification level for the policy
* `verificationMethod` is the verification method for the policy
* `trustedSigners` is the list of allowed signers

### Anatomy of a source verification policy

A source verification policy describes requirements for a source, as used by an Argo CD Application, that have to be met in order to be used for a sync.

A source verification policy has the following properties:

* A repository pattern (shell-glob type) that is matched against the URL of the source (mandatory)
* A repository type that is matched against the application's repository type (mandatory)
* A verification level that describes with which level the verification should be performed (mandatory)
* A verification methods that describes how to the source should be verified (mandatory)
* A list of identities of trusted signers (optional, if omitted, any known signer is trusted)

Additionally, there are some optional properties you can set for a policy that depend on the verification level, method and respository type.

Source verification policies are configured within an AppProject and should be considered an admin-level configurable.

#### Repository pattern

This is a glob-style pattern that is matched against the URL of the source to verify. When the pattern matches a source, it will be verified, otherwise verification of the source will be skipped.

#### Repository type

The type of repository. Right now, only "git" is supported.

#### Verification levels

Each source verification policy currently knows about four levels of verification:

* `none`: As the name implies, verification will be disabled at this verification level.
* `head`: This verification level will verify the commit that is pointed to by the HEAD of the target revision. In case target revision is an annotated tag, only the signature of the tag will be verified.
* `progressive`: This verification level will verify all commits between the application's target revision and the revision it was last synced to, but not beyond that. The revision that the application is currently synced to does not necessarily need to be signed, so that a migration path from level `head` to the more secure method `progressive` is possible. However, if the application hasn't synced before (i.e. there is no previous revision in the application's sync history), the `progressive` level will inhibit the same behaviour as the `strict` verification level (see below).
* `strict`: This verification level will always verify all commits in the history of the repository that led to the application's target revision. The `strict` level effectively enforces that all commits in your repository need to be signed from the beginning of your repository's history, without allowing _any_ unsigned commit in the history. This is the most secure policy.

For the `strict` and `progressive` verification levels, if the target revision is an annotated tag, then that tag's signature will also be verified in addition to the specific commits verified as defined by the respective level.

#### Verification method

This specifies the method to use when verifying signatures. Currently, the only available method available is `gpg`, which uses GnuPG to verify OpenPGP signatures on commits in Git.

#### Trusted signers

This is a list of signer IDs that are considered trusted. If a commit in the repository is signed by an ID that is not specified in the list of trusted signers, the verification will fail.

If no trusted signers are configured, all signers known to Argo CD will be trusted.

The format of the key is dependent on the verification method.

### Verification levels

The verification level controls the depth of verification.

As a simple example, consider the following revision history:

```
                   HEAD
       1.0         2.0
        |           |
A---B---C---D---E---F
```

In the above diagram, `A` through `F` represent the commits making up the repository's history, and `1.0` and `2.0` are annotated tags pointing to `C` and `F` respectively. In this example, `HEAD` is pointing to `F`.

#### Verification level "none"

With this level, an Application can sync to any of the above revisions regardless of whether there are valid signatures on the source. Verification will not be performed.

#### Verification level "head"

With verification level `head`, if the `targetRevision` is set to one of the commits `A` through `F` (or a symbolic reference to one, such as `HEAD`), only a valid signature on that particular commit is required.

If `targetRevision` is set to the tag `1.0` or `2.0`, only a valid signature on the particular tag will be required, but not on the commit which the tag points to. 

For example, if `targetRevision` would be `F`, all commits from `A` to `E` won't require a signature for a successful sync. This is the same behaviour as the legacy GnuPG verification mode implements.

#### Verification level "progressive"

In verification level `progressive`, the commits that need to be signed depend on the previous state of the Application. 

Consider we want to sync to target revision `F` (or `HEAD`). If the Application has never synced before, all commits from `A` to `F` will need to be signed by an allowed entity in order for the sync to proceed.

However, if the Application previously has been successfully synced to commit `C` (regardless of whether it was a verified sync), only the commits `D`, `E` and `F` will need to be signed. If the target revision is set to the annotated tag `2.0` instead of to the commit `F` (or a symbolic reference to that commit, such as `HEAD`), the signature on that tag is verified in addition - so all of the tag `2.0` and commits `F`, `E` and `D` need to verified successfully for the application to be allowed to sync.

#### Verification level "strict"

In verification level "strict", all commits in the repository's history that lead to the target revision need to be signed. In above example history, this means all commits `A` through `F` need to be verified successfully.

If the target revision is set to the annotated tag `2.0` instead of to the commit `F` (or a symbolic reference to that commit, such as `HEAD`), the signature on that tag is verified in addition - so all of the tag `2.0` and commits `A` through `F` need to verified successfully for the application to be allowed to sync.

### Verification methods

#### Verification method "gpg"

Right now, this is the only supported verification method. It will use GnuPG to verify PGP signatures on commits in Git.

Using this method, the list of allowed signers is comprised of PGP key IDs. Commits signed by keys other than those specified in the list of allowed signers will not verify successfully. In addition to the key IDs to be listed as allowed signers, the keys themselves need to be imported into Argo CD as well using existing mechanisms.

### Significance of policy order

Policies are not accumulative. Only one policy will be applied per source, and therefore the order of how policies are defined is significant. 

Argo CD will select the first policy that matches an Application's source URL, and will ignore any other policies that follow. Policies are being evaluated top-down, so your more specific policies must come before the more broad ones.

As an example, consider you want to verify that all commits across all repositories you source from `github.com` have a valid signature from GitHub's web signing key. However, on a specific repository, you want to have a different policy, using `strict` level and allowing a different signer in addition. You would set up the more specific policy first, and then the broad one:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: gpg
  namespace: argocd
spec:
  sourceVerificationPolicies:
  - repositoryPattern: 'https://github.com/example/super-secure'
    verificationLevel: strict
    verificationMethod: gpg
    trustedSigners:
    - keyID: D56C4FCA57A46444
    - keyID: 4AEE18F83AFDEB23
  - repositoryPattern: 'https://github.com/*'
    verificationLevel: progressive
    verificationMethod: gpg
    trustedSigners:
    - keyID: 4AEE18F83AFDEB23
```

If the policies would have been defined in the opposite order, the specific policy for `https://github.com/example/super-secure` would never match, because that repository URL is already matched by the other policy's `https://github.com/*` pattern, thus that policy would be applied.

### Creating a new Application from a "mixed-signed" source repository

Having all commits in a Git repository cryptographically signed from the repository's inception is an ideal situation, however the reality often looks different. Often times, the requirement to cryptographically sign commits comes at some point in time of a repository's lifecyle. 

This is what we call "mixed-signed", and is explained in the below diagram:

```
                   HEAD
       1.0         2.0
        |           |
A---B---C---D---E---F
-   -   -   +   +   +
```

Again, the letters `A` through `F` represents commits in the revision history. The dash and plus signs below the commits denote whether a commit is signed (`+`) or unsigned (`-`). So as we can see, the repository is in a "mixed-signed" state, where some commits are signed and others are not.

A revision history such as shown above has a few implications when it comes to commit verification:

1. If an Application under a verification policy that enforces the `strict` verification level tries to sync, it will not be able to succeed, as the `strict` level requires each and every commit in the history to be signed, but commits `A`, `B` and `C` are not.

2. If an Application under a verification policy that enforces the `progressive` verification level tries to sync, it _may_ succeed under certain conditions:

   For example, if the Application is currently synced to revision `C` (potentially before the source verification policy was in place) can sync successfully to target revision `F`, as all of the subsequent commits `D`, `E` and `F` are signed. However, if the Application would currently be synced to revision `B`, it can't sync successfully because `C` is not signed. There just is no way forward.
   
   Similarly, if the Application has never synced before (i.e. is a new Application), it won't be able to sync at all, because the verification level `progressive` would behave the same as level `strict`, thus _all_ commits in the history needs to be signed, but commits `A`, `B` and `C` are not.
   
3. An Application under a policy which enforces the verification level `head` can sync to any of the commits `D`, `E` and `F` regardless of whether it has synced before.

So as it seems from that explanation of the impact of verification level, a newly Application could only ever be synced with a verification level of `head`. 

### Detailed examples

### Security Considerations

Implementing this proposal would significantly increase resilience against breached Git repositories. However, it won't add any resilience against identity theft, e.g. when someone compromises a developer's credentials as well as signature keys.

### Risks and Mitigations

The new verification levels (i.e. `progressive` and `strict`) make source verification a lot more robust and thorough compared to the current implementation. 

However, for `progressive` mode, there are at least two risk factors I can see which I'd like to raise for validation: 

1. The information stored in the sync history needs to be trusted. The PoC implements storing & verifying a HMAC for the _last synced revision_ in the Application's `.status` field. The HMAC is calculated out of the Application's `name` and `namespace` as well as the last synced `revision` itself, and uses Argo CD's `server.secretkey` as the secret key for calculating the HMAC. The `progressive` mode verifies all commits between the resolved `targetRevision` at time of sync and the last synced `revision`. Obviously, the history information can be copied manually when a user has access to the `Application` resource. The idea behind the HMAC is that it _has had to exist_ before, i.e. it's been calculated and written to the CR after a successful sync. 

    Since every commit between the last synced revision and the target revision requires to be signed, an adversary modifying the `Application` CR in a non-intended way could only ever move _backwards_ in time (i.e. take values from a commit that has synced before), there would be no way to slip in an unsigned commit somewhere.

    **Risk:** Using the progressive verification level could allow an adversary to sync 
    
2. The `progressive` level has a "bootstrap" feature to allow creating an `Application` in `progressive` level. Obviously, when an `Application` is created, it does not have any history information unless it is synced initially. The bootstrap feature defines a time window, the `bootstrapPeriod`, in which an `Application` without history can temporarily sync using the `head` level (i.e. only with the HEAD of `targetRevision` being signed). The bootstrap feature assumes that the `.metadata.creationTimestamp` of the `Application` resource is immutable, or rather be reset by the Kubernetes API server upon modification. During the timeframe in the `bootstrapPeriod`, an adversary could delete the `Application`'s history and sync to any signed commit in the repository.

3. The `progressive` level doesn't allow roll-backs, as it verifies the commits between the previous synced state and the target revision. If the application is on `F`, and wants to roll back to `C`, it cannot find any revision history for this path (because `C` existed prior to `F`). This must be properly documented. For `head` and `full`, this is not a concern as long as the level's requirements are met in the repository.

### Upgrade / Downgrade Strategy

#### Upgrade

Upgrade will be seamless. When Argo CD detects the prior configuration for Git commit verification (i.e. `.spec.signatureKeys` is populated in the `AppProject`), the new implementation will behave like the current one. 

Internally, this will create a single verification policy similar to the following and ignore all other configured policies: 

```
repositoryPattern: '*'
repositoryType: git
verificationLevel: head
verificationMethod: gpg
trustedSigners:
- # taken from .spec.signatureKeys
```

If a user wants to migrate from the current implementation to the new source verification policies, they will first have to remove `.spec.signatureKeys` and can then go ahead and define desired policies in `.spec.sourceVerificationPolicies`.

#### Downgrade

At downgrade time, people will have to reconfigure `.spec.signatureKeys` into any AppProject where it has been removed and remove `.spec.sourceVerificationPolicies` at the same time.

## Drawbacks

* Configuring source verificiation policies is a little bit more involved than just configuring the trusted signature keys.
* The algorithms behind source verification policies are a little more complex than those for enforcing commits signed by a hard-coded list of keys. There might be more mistakes in these algorithms that could be exploited in order to allow 

## Alternatives

There are a couple of alternative locations to where verification policies could be configured:

* A verification policy could be directly configured in a repository configuration instead of being configured in the AppProject. This feels like the more natural and logical place, however, repository configuration is not strongly typed due to residing completely in a secret. This also has the downside of only having base64 encoded values in the secret's data, making a quick inspection of value a little more involved.
   
   Furthermore, a repository configuration might be a user-controllable asset, while source verification is more of a governance topic.
   
* Verification policies could be placed in the AppProject's allowed source repositories, for example:

   ```
   spec:
     sourceRepos:
     - repo: https://github.com/*
       verificationLevel: full
       trustedSigners:
       - keyID: a
       # ...
   ```
   
   However, this would be a breaking change, as currently the type of an entry in `sourceRepos` is just `string`, and it would have to become a complex type.
   
We might want to consider moving SVPs into either of those places with Argo CD 3.0. Because of them would be breaking changes, the authors did not consider them for the 2.x branch.
