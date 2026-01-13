# E2E Test for Nil Pointer Bug

## Overview

This directory contains test data to reproduce the nil pointer dereference bug in the permission validator.

## Bug Description

**Issue**: https://github.com/argoproj/argo-cd/issues/25460

The permission validator in `controller/sync.go` was accessing `res.Namespaced` without checking if `res` is nil.

**Stack Trace**:
```
runtime error: invalid memory address or nil pointer dereference
github.com/argoproj/argo-cd/v3/controller.(*appStateManager).SyncAppState.func1(0xc100fc2e90, 0x0)
	controller/sync.go:311
```

## Files

- `custom-resource.yaml`: A RabbitmqCluster custom resource without its CRD
- `kustomization.yaml`: Kustomize configuration
- `README.md`: This file

## The Fix

```go
// Before (vulnerable):
if !project.IsGroupKindNamePermitted(un.GroupVersionKind().GroupKind(), un.GetName(), res.Namespaced) {

// After (fixed):
isNamespaced := true
if res != nil {
    isNamespaced = res.Namespaced
}
if !project.IsGroupKindNamePermitted(un.GroupVersionKind().GroupKind(), un.GetName(), isNamespaced) {
```
