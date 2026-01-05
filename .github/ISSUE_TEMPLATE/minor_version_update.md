---
name: Update Minor Version
about: Used by our Release Champion to add steps when upgrading minor version
title: ''
assignees: ''
---

# Vx.1 to x.2

## Breaking Changes

What changed that could affect your setup:
Deprecated flags / behavior removals 
Deprecated fields 
Other incompatible behavior or semantics changes between versions
 (Add list here)

## Custom Healthchecks Added

New built-in health checks added in this release:
Add any new CRD health support added in this version)

## Kustomize Upgraded

Bundled Kustomize version bump:
Any breaking behavior 
Notes about specific upstream behavior changes (e.g., namespace propagation fixes).

## Helm Upgraded

Bundled Helm version bump:
Any breaking behavior 
Verify if your charts depend on any features tied to Helm versions.=

## API / Security Changes

Changes in API surface or security behavior:
Any removal of sensitive fields or changed authentication defaults

## Behavioral Improvements / Fixes

Operational stability and performance fixes:
Minor stability, reconciler improvements, bug fixes

## Deprecated Items
Flags / behavior to be removed in future releases
Any deprecated config API or CLI flags

## Upgrade Instructions
Example: CLI upgrade command

