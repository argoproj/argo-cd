# GitOps Engine

Various GitOps operators address different use-cases and provide
different user experiences but all have similar set of core features.
This library implements core GitOps features:

- Kubernetes resource cache ✅
- Resources reconciliation ✅
- Sync Planning ✅
- Access to Git repositories
- Manifest Generation

## Usage

This library is mainly designed to be used by the Argo CD project.
However, it can also be used by other projects that need GitOps
features.

To use the library, add it as a dependency in your Go module:

```bash
go get github.com/argoproj/argo-cd/gitops-engine
```
