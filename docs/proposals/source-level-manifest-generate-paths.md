---
title: Source-Level Manifest Generate Paths
authors:
  - TBD
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2026-03-17
last-updated: 2026-03-17
---

# Source-Level Manifest Generate Paths

Move the `argocd.argoproj.io/manifest-generate-paths` optimization from an Application-level annotation to a per-source field on `ApplicationSource`, enabling fine-grained control in multi-source Applications.

Related Issues:
* [Support multiple sources for an application](https://github.com/argoproj/argo-cd/issues/677)
* [Scalability benchmarking](https://github.com/argoproj/argo-cd/blob/master/docs/proposals/004-scalability-benchmarking.md)

Related Proposals:
* [Multiple Sources for Applications](https://github.com/argoproj/argo-cd/blob/master/docs/proposals/multiple-sources-for-applications.md)

## Open Questions

* Should the annotation be formally deprecated immediately, or should there be a multi-release deprecation window?
* Should the field support glob patterns beyond what the annotation currently supports?

## Summary

The `argocd.argoproj.io/manifest-generate-paths` annotation is an important performance optimization for monorepo setups. It allows Argo CD to skip manifest regeneration when changes occur in unrelated paths. However, the annotation is Application-scoped — a single semicolon-separated string shared across all sources. This makes it unsuitable for multi-source Applications where each source may reference different repositories or different directories within the same repository.

This proposal introduces a `manifestGeneratePaths` field (string array) on the `ApplicationSource` struct, allowing each source in a multi-source Application to declare its own set of relevant paths independently.

## Motivation

Multi-source Applications (introduced via the [multiple-sources proposal](https://github.com/argoproj/argo-cd/blob/master/docs/proposals/multiple-sources-for-applications.md)) allow an Application to combine manifests from multiple Git repositories or Helm charts. However, the manifest-generate-paths optimization was never updated to account for this. The current annotation applies uniformly to all sources, which leads to several problems:

1. **Incorrect cache invalidation**: A path change relevant to one source triggers regeneration for all sources.
2. **Over-broad paths**: Users must list paths for all sources in a single annotation, reducing the effectiveness of the optimization.
3. **Fragile format**: The semicolon-separated string is not type-safe and is easy to misconfigure.
4. **No per-source semantics**: There is no way to associate a path with a specific source.

### Goals

* Allow each `ApplicationSource` to independently specify which paths are relevant for manifest generation.
* Maintain full backward compatibility with the existing annotation.
* Provide a clear migration path from the annotation to the new field.
* Improve cache invalidation accuracy for multi-source Applications.

### Non-Goals

* Changing the semantics of path matching (e.g., adding glob support beyond what currently exists).
* Removing or breaking the existing annotation in the near term.
* Addressing other multi-source limitations unrelated to manifest-generate-paths.

## Proposal

### Use cases

#### Use case 1: Multi-source Application with independent path scoping

As a user with a multi-source Application that combines a Helm chart from one repository with values files from another, I want to specify that the Helm chart source should only regenerate when files under `charts/my-app/` change, while the values source should only regenerate when `environments/production/values.yaml` changes.

#### Use case 2: Monorepo with multiple sources pointing to different directories

As a user with a multi-source Application where both sources point to the same monorepo but different directories, I want each source to only trigger regeneration when its own directory changes, not when the other source's directory changes.

#### Use case 3: Single-source Application with type-safe configuration

As a user with a single-source Application, I want to specify manifest-generate-paths as a proper typed field on the source rather than as a fragile annotation string.

### Implementation Details/Notes/Constraints

#### API Change

Add a `ManifestGeneratePaths` field to the `ApplicationSource` struct in `pkg/apis/application/v1alpha1/types.go`:

```go
type ApplicationSource struct {
	// RepoURL is the URL to the repository (Git or Helm) that contains the application manifests
	RepoURL string `json:"repoURL" protobuf:"bytes,1,opt,name=repoURL"`
	// Path is a directory path within the Git repository
	Path string `json:"path,omitempty" protobuf:"bytes,2,opt,name=path"`
	// TargetRevision defines the revision of the source to sync the application to.
	TargetRevision string `json:"targetRevision,omitempty" protobuf:"bytes,4,opt,name=targetRevision"`
	// ... existing fields ...

	// ManifestGeneratePaths is a list of path patterns relevant to this source for
	// manifest generation. When set, Argo CD will only regenerate manifests for this
	// source when changes are detected in the specified paths. Paths are relative to
	// the repository root. A dot (.) refers to the source's own Path field value.
	// This field takes precedence over the argocd.argoproj.io/manifest-generate-paths
	// annotation when both are present for a given source.
	ManifestGeneratePaths []string `json:"manifestGeneratePaths,omitempty" protobuf:"bytes,15,rep,name=manifestGeneratePaths"`
}
```

#### Precedence Rules

1. If `source.ManifestGeneratePaths` is set (non-empty), use it for that source.
2. If `source.ManifestGeneratePaths` is not set, fall back to the Application-level `argocd.argoproj.io/manifest-generate-paths` annotation (existing behavior).
3. If neither is set, regenerate manifests on every change (existing behavior).

#### Path Resolution

Path semantics remain the same as the existing annotation:
* `.` refers to the source's `Path` value.
* Absolute paths (starting with `/`) are relative to the repository root.
* Relative paths are resolved relative to the source's `Path` value.

#### Affected Components

* **Webhook handler** (`util/webhook/`): Must check per-source paths when determining whether to trigger a refresh.
* **Application controller** (`controller/`): Must use per-source paths when deciding whether to regenerate manifests for each source.
* **Repo server** (`reposerver/`): The `ManifestGeneratePathsAnnotation` field in the gRPC request should be extended or supplemented to carry per-source paths.
* **CMP server**: When `--plugin-use-manifest-generate-paths` is enabled, per-source paths should be forwarded.

### Detailed examples

#### Single-source Application (new field)

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
spec:
  source:
    repoURL: https://github.com/org/monorepo.git
    path: apps/my-app
    targetRevision: HEAD
    manifestGeneratePaths:
      - .
      - /libs/shared
```

This is equivalent to the current annotation `argocd.argoproj.io/manifest-generate-paths: .;/libs/shared`, but expressed as a typed array.

#### Multi-source Application (per-source paths)

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-multi-source-app
spec:
  sources:
    - repoURL: https://github.com/org/helm-charts.git
      path: charts/my-app
      targetRevision: HEAD
      manifestGeneratePaths:
        - .
    - repoURL: https://github.com/org/config.git
      path: environments/production
      targetRevision: HEAD
      manifestGeneratePaths:
        - .
        - /base
```

Here, a change to `charts/my-app/` in the first repo only triggers regeneration for the first source. A change to `environments/production/` or `base/` in the second repo only triggers regeneration for the second source.

#### Backward-compatible mixed usage

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  annotations:
    argocd.argoproj.io/manifest-generate-paths: .;/shared
spec:
  sources:
    - repoURL: https://github.com/org/repo.git
      path: apps/frontend
      targetRevision: HEAD
      manifestGeneratePaths:
        - .
        - /libs/ui
    - repoURL: https://github.com/org/repo.git
      path: apps/backend
      targetRevision: HEAD
      # No manifestGeneratePaths — falls back to annotation: ".;/shared"
```

The first source uses its own field. The second source falls back to the annotation.

### Security Considerations

* This proposal does not introduce new attack surfaces. The paths are used purely for cache invalidation optimization and do not affect access control.
* Misconfigured paths could cause manifests to not regenerate when they should, leading to stale deployments. This is the same risk as the current annotation.

### Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Users may not realize the field takes precedence over the annotation, leading to confusion | Clear documentation and a warning log when both are present for the same source |
| Increased API surface area | The field is optional and backward-compatible; no existing workflows break |
| Protobuf field number conflicts during implementation | Coordinate field number allocation during implementation |

### Upgrade / Downgrade Strategy

**Upgrade**: No action required. Existing Applications with the annotation continue to work unchanged. Users can adopt the new field at their own pace.

**Downgrade**: If an Application uses the `manifestGeneratePaths` field and Argo CD is downgraded to a version that does not support it, the field is ignored (standard Kubernetes behavior for unknown fields). The Application will regenerate manifests on every change unless the annotation is also set. Users should ensure the annotation is set as a fallback during the transition period.

## Drawbacks

* **Increased API complexity**: Adding a new field to `ApplicationSource` increases the surface area of the API. However, the field is optional and its semantics are straightforward.
* **Two ways to configure the same thing**: During the transition period, both the annotation and the field exist. This is mitigated by clear precedence rules and documentation.

## Alternatives

### Alternative 1: Indexed annotation format

Instead of a new field, extend the annotation format to support per-source indexing:

```yaml
annotations:
  argocd.argoproj.io/manifest-generate-paths: .;/shared
  argocd.argoproj.io/manifest-generate-paths.0: .
  argocd.argoproj.io/manifest-generate-paths.1: .;/base
```

This avoids an API change but is fragile (index-based), not type-safe, and inconsistent with how other per-source configuration is handled.

### Alternative 2: Named source annotation format

Use source names in the annotation:

```yaml
annotations:
  argocd.argoproj.io/manifest-generate-paths.frontend: .;/libs/ui
  argocd.argoproj.io/manifest-generate-paths.backend: .;/shared
```

This is more readable than index-based but still relies on annotations and is not type-safe. It also requires all sources to have names.

### Alternative 3: Status quo

Continue using the single annotation. Users with multi-source Applications must list all paths for all sources in one string, accepting reduced optimization effectiveness. This is the simplest option but does not address the core problem.
