# Releasing

This document describes the `gitops-engine` library releasing process.

# Versioning

* The library is versioned using the [semantic versioning](http://semver.org/): new version will be backwards-compatible
with earlier versions within a single major version.
* The library has its own release cycle and not tied to Argo CD release cycle.
* The first library release is v0.1.0.

# Release process

* Release branch is created for every minor release.
* The branch name should use the following convention: `release-<major>.<minor>`. For example all v0.1 releases should
be in `release-0.1` branch.
* Actual release is a git tag which uses the following naming convention: `v<major>-<minor>-<patch>`. For example: `v0.1.0`, `v0.1.1` etc.