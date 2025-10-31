---
title: Source Verification Policies
authors:
  - "@olivergondza" # Authors' github accounts here.
sponsors:
  - "@jannfis"
  - TBD
reviewers:
  - "@jannfis"
  - TBD
approvers:
  - TBD

creation-date: 2023-08-08
---

# Source Verification Policies

This proposal introduces an evolution to the existing GnuPG commit verification feature.

## Summary

Argo CD has had a feature to verify OpenPGP signatures on Git commits using GnuPG for a while.
However, this feature is limited and in some ways may behave unexpectedly.
Argo CD would ever only verify the signature on the commit that's pointed to by the `HEAD` of the `Application`'s resolved `targetRevision`.

Source verification policies are an evolution of the legacy signature verification in Argo CD.
It brings new verification modes, the possibility of treating multiple sources in an Application with different strictness.
It also sets the foundation to implement more verification methods in the future that are not gpg, nor git specific.

## Motivation

As a deployment tool, Argo CD sits in a prominent spot to enforce secure supply chain requirements.
The cryptographic verification of sources and artifacts becomes more and more important to organizations and is heavily regulated in some fields.

Verifying the `HEAD` commit before syncing is not a sufficient level of verification for many organizations and individuals anymore.

Also, the current approach is an "all-or-nothing" for any given AppProject.
Different repositories might have different trust levels, different sets of contributors or contribution/signing guidelines.
Especially with the advent of multi-source applications, the current approach of defining the trusted signers for all project repositories/Applications is not flexible enough.

### Goals

* Increase confidence in Git commit verification
* Manage signer trust based on source repositories (more fine-grained compared to application projects)
* Be flexible enough to support other verification providers in the future (e.g. Helm provenance, Git signed using sigstore, etc.)
* Make it easy for users to migrate to a more secure verification policy
* Fully backwards compatibility to existing GnuPG commit verification mechanisms

### Non-Goals

* Implement other verification methods than GnuPG for Git commits.
  * This may come in later, and the design of SVPs would allow those to integrate rather easily.

## Proposal

### Use cases

1. As an Argo CD user, I need to **ensure that my applications only sync if every single commit in my source repository has been cryptographically signed** by a trusted contributor.
    - This effectively prevents unsigned/untrusted commits in the git history.
1. As an Argo CD user, I need to apply a **different level of trust to different source repositories**, especially with multi-source applications.
    - Permitting different source repositories to have a different signing policy, and their flexible evolution in time (gradually introduce signing to multiple repositories, for example).
1. As an Argo CD admin, I need to restrict **distinct sets of contributors in different repositories**.
    - Rather than trusting all the contributors in all project's repositories. This becomes another line of defense in the event of a key compromise.

## Source verification policy

### Overview

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
spec:
  sourceIntegrity:
    # Out of scope, demonstrating the declaration structure permits extending to different
    # source types and whatever integrity verification turns out suitable in the future
    # helm: {}
    git:
      policies:
        - repos:
            - "https://github.com/foo/*"
          gpg:
            mode: "none|head|strict"
            keys:
             - "0xDEAD"
             - "0xBEEF"
```

#### Policy repos

List of glob-style patterns matched against the URL of the source to verify.
When the pattern matches a source, it will be verified, otherwise verification of the source will be skipped.

#### GPG verification policy

Right now, this is the only supported verification method.
It will use GnuPG to verify PGP signatures on commits in Git.

Using this method, the list of allowed signers is a list of PGP key IDs.
Commits signed by keys other than those specified in the list of allowed signers will not verify successfully.
In addition to the key IDs to be listed as allowed signers, the keys themselves need to be imported into Argo CD as well using existing mechanisms.

The `mode` defines how thorough the GPG verification is:

* `none`: No gpg verification performed.
* `head`: Verify the commit pointed to by the HEAD of the target revision.
* `strict`: Verify all ancestor commits from target revisions to init. This makes sure there is no unsigned change whatsoever.

`keys` lists the set of key IDs to trust for signed commits
If a commit in the repository is signed by an ID not specified in the list of trusted signers, the verification will fail.
If no trusted keys are configured, all signers from `argocd-gpg-keys-cm` ConfigMap are trusted.

### Verification modes explained

As a simple example, consider the following revision history:

```
                   HEAD
       1.0         2.0
        +           +
        |           |
A---B---C---D---E---F
-   -   -   +   +   +
```

In the above diagram, `A` through `F` represent the commits making up the repository's history, and `1.0` and `2.0` are annotated tags pointing to respective commits.
In this example, `HEAD` is pointing to `F`, aka `2.0`.
The `+`/`-` indicate if the commit or tag are signed by a trusted key - commits `D`, `E` and `F` are signed, and so are the two tags.

#### Verification mode "none"

With this mode, an Application can sync to any of the above revisions regardless of whether there are valid signatures on the source.
Verification is not performed.

#### Verification mode "head"

If the `targetRevision` is set to one of the commits `A`, `B` or `C` it will not be trusted, because the commits are not signed.
On commit `D`, `E` or `F`, it will.

Specifying `HEAD` (tip of the named branch), `1.0` or `2.0` will successfully validate.
`HEAD` because the revision it points to is signed.
`1.0` and `2.0` because they are signed tags.

#### Verification mode "strict"

Strict mode is not going to trust any point of this git history, because it contains unsigned commits `A`, `B` and `C`.

### Significance of policy order

Policies are not accumulative.
Only one policy is applied per source repository, and therefore the order of definitions is significant.

Note that a multi-source application can have its source repositories validated based on different SVPs each.

Argo CD will select the first policy that matches a repository source URL, and will ignore any other policies that follow.
Policies are being evaluated top-down, so your more specific policies must come before the more broad ones.

As an example, consider you want to verify that all commits across all repositories you source from `github.com` have a valid signature from GitHub's web signing key.
However, on a specific repository, you want to have a different policy, using `strict` mode and allowing a different signer in addition.
You would set up the more specific policy first, and then the broad one:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: gpg
  namespace: argocd
spec:
  sourceIntegrity:
    git:
      policies:
        - repos: ['https://github.com/example/super-secure']
          gpg:
            mode: strict
            keys:
              - 4AEE18F83AFDEB23
              - D56C4FCA57A46444
        - repos: ['https://github.com/*']
          gpg:
            mode: head
            keys:
              - 4AEE18F83AFDEB23
```

If the policies had been defined in the opposite order, the specific policy for `https://github.com/example/super-secure` would never match, because that repository URL is already matched by the other policy's `https://github.com/*` pattern, thus that policy would be applied.

There needs to be a visual indicator in Argo CD CLI and UI, pointing out project repositories that do not perform GPG verification.
This is to provide an administrator with feedback on the repository matching evaluation.

### Security Considerations

Implementing this proposal would significantly increase resilience against breached Git repositories:

- An unauthorized contributor has commit access.
- Signing key was compromised.

When someone compromises a developer's credentials as well as signature keys, the adversaries can force Argo CD to trust their commits only in those repositories, where the compromised keys had been added, but not elsewhere.

### Upgrade / Downgrade Strategy

#### Upgrade

Upgrade will be seamless.
When Argo CD detects the prior configuration for Git commit verification (i.e. `.spec.signatureKeys` is populated in the `AppProject`), the new implementation will behave like the current one.

Internally, this will create a single verification policy similar to the one illustrated earlier taking the keys from `.spec.signatureKeys`.

If a user wants to migrate from the current implementation to the new source verification policies, they will first have to remove `.spec.signatureKeys` and can then go ahead and define desired policies in `.spec.sourceIntegrity.git.policies`.

To achieve the legacy Argo CD verification behavior in a project, use the following config:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
spec:
  sourceIntegrity:
    git:
      policies:
        - repos: ["*"] # For any repository in the project
          gpg:
            mode: "head" # Verify only the HEAD of the targetRevision
            keys:
              - "..." # Keys from .spec.signatureKeys
```

#### Downgrade

At downgrade time, people will have to reconfigure `.spec.signatureKeys` into any AppProject where it has been removed and remove `.spec.sourceIntegrity.git.policies` at the same time.

Unless the user motivation for the downgrade is a bug in Argo CD implementation, moving the mode to `head` or `none` (depending on what they are downgrading to) is sufficient.

## Drawbacks

* Configuring source verification policies adds some complexity to both Argo CD implementation and used configuration.
* Configuring or implementing those incorrectly can be a source of security incidents.

## Alternatives

### How to handle local manifests?

The current implementation rejects them when GPG is turned on and projest's signing keys are declared.
Can be done selectively based on source integrity criteria applicability (Git/OCI/Helm & repo).

### Where to configure verification policies?

* A verification policy could be directly configured in a repository configuration instead of being configured in the AppProject.
  This feels like the more natural and logical place however, repository configuration is not strongly typed due to residing completely in a secret.
  This also has the downside of only having base64 encoded values in the secret's data, making a quick inspection of value a little more involved.
   
   Furthermore, a repository configuration might be a user-controllable asset, while source verification is more of a governance topic.
   
* Verification policies could be placed in the `AppProject`'s `sourceRepos`, for example:

   ```
   spec:
     sourceRepos:
       # ...
   ```
   
   However, this would be a breaking change, as currently the type of entry in `sourceRepos` is just `string`, and it would have to become a complex type.
   
We might want to consider moving `sourceIntegrity` into either of those places with the next major Argo CD release.



### Dealing with unsigned commits in the history using `strict` verification mode

**!!! This is a proposal extension. The whole can be approved with, or without it !!!**

Having all commits in a Git repository cryptographically signed from init is an ideal situation.
However, the reality often looks different.
There are a number of reasons why unsigned, or signed but untrusted commits can be found in git history, and it is almost impossible to avoid such situations in large and/or long-running repository.

- User forgot to sign the commit / used other key than intended
- Commit was created unsigned by a (misconfigured) tool
    - Such has merge&squash, automation/IDE created
- Key once approved was rotated after it was compromised, expired, cryptographically obsoleted
- Former contributor is no longer trusted (ex-employee, etc.)
- It is desirable to accept Pull Request from untrusted parties and merge them without resigning by trusted GPG keys.
- Repository policy has not required GPG signing in the past.

Dealing with such commits with the expectation to validate the entire history requires git history rewrites (rebase, force-push) that are problematic on a number of technical and organizational fronts.
Force pushing is often prohibited also as a security measure, so asking users to relax security in order to improve security proposes them with a Sophie's choice.

Some of these can be prevented by requiring gpg signatures on git push, but not all.
Also, preventing force pushing is easier to configure in git hosting sites, than restricting only a fixed set of GPG signing keys can push commits.

While retroactively signing commits without a signature is a somewhat straightforward chore (`git rebase --signoff XXX`),
identifying all commits signed by a particular key and re-signing it by a new/valid one is more cumbersome and error-prone.
Hence, this proposal is treating git history rewriting as an undesirable and possibly even technically disallowed procedure.

In fact, it does not harm UX *and* improves security with:

#### Git history *sealing* for strict verification mode

A sealing commit is a gpg signed commit that works as a "seal of approval" attesting that all its ancestor commits were either signed by a trusted key, or reviewed and trusted by the commit author.
Argo CD verifying gpg signatures would then progres only as far back in the history as the most recent "seal" commits in each individual ancestral branch.

In practice, a commiter reviews all commits that are not signed or signed with untrusted keys from the previous "seal" and creates a (possibly empty) commit with a custom trailer.
Such commits can have the following organization level semantics:

- "From now on, we are going to gpg sign all commits in this repository. There is no point in verifying the unsigned ones from before."
- "I merge these changes from untrusted external contributor, and I approve of them."
- "I am removing the GPG key of Bob. All his previous commits are trusted, but no new ones will be. Happy retirement, Bob!"
- "I am replacing my old key with a new one. Trust my commit signed with the old one before this commit, trust my new one from now on."

To make a "seal" commit, run `git commit --signoff --gpg-sign --trailer="Argocd-gpg-seal: <justification>"` and push to branch pulled by Argo CD.
The advantage is the exact same procedure deals with all the identified situations of unsigned or untrusted commits in the history, eliminating the room for eventual rebasing mistakes that would jeopardize security or correctness.
It is possible to introduce tooling to help identify all previous "seal" commits and all the untrusted commits made since then, so the administrator knows exactly what are they "seal-signing".

Git history sealing can be part of the `strict` mode, eventually enabled through a flag, or be enabled in a separate "less-strict" mode.
Either way, it requires no force-pushing.

Example:

```
## head mode

T     <- verified
| \
|  o  
|  |
o  |
|  |
|  o
| /
o
|
o


## strict mode

T     <- verified
| \
|  o  <- verified
|  |
o  |  <- verified
|  |
|  o  <- verified
| /
o     <- verified
|
o     <- verified


## strict mode - with seal commits

T     <- verified
| \
|  S  <- verified (seal)
|  |
S  |  <- verified (seal)
|  |
|  o
| /
o
|
o

```

#### Comparison with the original `progressive` mode

The "seal-signing" is inspired by the original mode that verified commits from `targetRevision` to a commit of last successful Argo CD Application sync.
They both verify the history backwards only until a certain point, from which it is considered inherently trusted.

The non-trivial N:M mapping between repositories and Applications (or even Argo CD instances) was a cause of a number of corner cases identified in the original proposal.

Seal-signing marks the point(s) from where not to verify commits *inside* the repository itself, and thus is making sure that all Argo CD instances and their applications have a consistent view of what they are, regardless of application removal from Argo CD, Argo CD migration, etc.

Both approaches, in fact, work as an optimization mechanism by limiting the number of commits to verify.
For sealing, a commiter needs to add a seal commit manually even if there are no unsigned changes to speed things up.
Additionally, the implementation can cache the last `strict`-verified commit per repository & strategy, to optimize verification speed on a best effort basis.

#### To merge or not to merge?

While complex, @olivergondza suggests incorporating this extension, as it improves the UX of `strict` mode significantly.
Without it, users that are facing one of the anticipated difficulties (employee leaving, key is rotated, etc.) can be tempted to switch to `head` or `none` mode to escape the risks of the non-trivial maintenance task.
Or keep the old, leaked, unused keys in place compromising their security posture.

**!!! End of proposal extension. !!!**
