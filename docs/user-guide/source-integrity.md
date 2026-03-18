# Overview

Argo CD permits declaring criteria for application sources integrity that, when not met, will prevent an application from syncing with a `ResourceComparison` error.
This is useful to verify the sources have not been tempered with by an unauthorized contributor.

Each Application Project can have its criteria configured in `AppProject`'s `.spec.sourceIntegrity`.
The criteria distinguish a type of verification they perform, and to which sources they apply.

Each application can be a subject or multiple checks, and the sync will be enabled only when all criteria are met.

> [!NOTE]
> Source Integrity Verification is only configured through `AppProject` manifests at this point. CLI and UI are not supported.

> [!NOTE]
> Signature verification is not supported for the Applications Sets populated by the git generator when they have the `project` field templated.

> [!WARNING]
> If source integrity is enforced, you will not be able to sync from local sources (i.e. `argocd app sync --local`) anymore.

## Supported methods

- [Git GnuPG verification](./source-integrity-git-gpg.md) verifies that Git commits are GnuPG Signed. This is a modern method of the commit signature verification originally configured in `AppProjects`'s `signatureKeys`.

## Multi-source applications

Each individual application source can be a subject of a different set of source integrity criteria, if desirable.
This is necessary if the sources are of a different type, such as Git and Helm.
But even different repositories of the same type can utilize different methods of verification, or their different configurations.
This is useful when an application combines sources maintained by different groups of people, or according to different contribution (and signing) guidelines.
