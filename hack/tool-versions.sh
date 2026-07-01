#!/bin/sh
###############################################################################
# This file defines the versions of the tools that are installed in the CI
# toolchain and the Docker image.
#
# Production binaries (helm, kustomize, git-lfs) are updated automatically by
# Renovate. Checksum files in ./hack/installers/checksums are refreshed via
# Renovate postUpgradeTasks. Manual bumps still require maintainer review.
#
# For protoc and oras, updating a tool's version here is not enough: you will
# need to create a checksum file in ./hack/installers/checksums matching the
# name of the downloaded binary with a ".sha256" suffix appended, containing
# the proper SHA256 sum of the binary.
#
# Use ./hack/installers/checksums/add-helm-checksums.sh,
# add-kustomize-checksums.sh, and add-git-lfs-checksums.sh to help download
# checksums.
###############################################################################
# renovate: datasource=github-releases depName=helm/helm packageName=helm/helm
HELM_VERSION=4.2.2
# renovate: datasource=github-releases depName=kubernetes-sigs/kustomize packageName=kubernetes-sigs/kustomize extractVersion=^kustomize/v(?<version>.*)$
KUSTOMIZE_VERSION=5.8.1
protoc_version=29.3
oras_version=1.2.0
# renovate: datasource=github-releases depName=git-lfs/git-lfs packageName=git-lfs/git-lfs
GIT_LFS_VERSION=3.7.1
