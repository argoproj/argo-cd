# GitOps Engine Design - Black Box

## Summary

During the elaboration of [White box](./design-white-box.md) option, it was discovered that some components are similar at a high-level but still have a lot of differences in 
design and implementation. This is not surprising because code was developed by different teams and with a focus on different use-cases. Given that it would require a lot of
effort to resolve such differences it is proposed contributing missing features of one project into an engine of another and use that engine in both projects.

## Proposal

It is proposed to use Argo CD application controller as the base for the GitOps engine and contribute a set of Flux features into it. There are two main reasons to try using Argo CD as
a base:
- Argo CD uses the _Application_ abstraction to represent the desired state and target the Kubernetes cluster. This abstraction works for both Argo CD and Flux.
- The Argo CD controller leverages Kubernetes watch APIs instead of pulling. This enables Argo CD features such as Health assessment, UI and could provide better performance to
Flux as well.

The following Flux features are missing in Argo CD:

- Manifest generation using .flux.yaml files.
- GPG commit signatures verification - an ability to verify the commit signature before pushing changes to the Kubernetes.
- Namespace mode - an ability to control only given namespace in the target cluster. Currently, Argo CD requires read access in all namespaces.

These features must be contributed to Argo CD before moving Argo CD controller into Argo Flux GitOps engine repo.

Flux additionally provides the ability to monitor Docker registry and automatically push changes to the Git repository when a new image is released. Both teams feel the this should not
be a part of GitOps engine. So it is proposed to keep the feature only in Flux for now and then work together to move it into a separate component that would work for both Flux
and Argo CD.

### Implementation Details/Notes/Constraints

The proposed approach is based on the assumption that Argo CD engine is flexible enough to cover all Flux use-cases and can be easily integrated into Argo CD. This assumption might
be wrong. So if we are missing some important Flux feature the approach might be no feasible.

### Risks and Mitigations

To mitigate the risk of let's start from POC to catch possible blockers earlier. 

### GitOps Engine POC

POC deliverables are:

- All POC changes are in separate branches.
- Argo CD controller will be moved to https://github.com/argoproj/gitops-engine.
- Flux will import GitOps engine component from the https://github.com/argoproj/gitops-engine repository and use it to perform cluster state syncing.
- The flux installation and fluxctl behavior will remain the same other than using GitOps engine internally. That means there will be no creation of Application CRD or Argo CD
specific ConfigMaps/Secrets.
- For the sake of saving time POC does not include implementing features mentioned before. So no commit verification, only plain .yaml files support, and full cluster mode.

## Design Details

### Public API

To be documented during POC implementation.


## Alternatives

[White box](./design-white-box.md)
