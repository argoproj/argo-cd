#!/bin/sh
###############################################################################
# This file defines the versions of the tools that are installed in the CI
# toolchain and the Docker image.
#
# Updating a tool's version here is not enough, you will need to create a
# checksum file in ./hack/installers/checksums matching the name of the
# downloaded binary with a ".sha256" suffix appended, containing the proper
# SHA256 sum of the binary.
#
# Use ./hack/installers/checksums/add-helm-checksums.sh and
# add-kustomize-checksums.sh to help download checksums.

# GoReleaser (release workflow only — hack/ci; not installed in the image):
# ./hack/ci/checksums/add-goreleaser-checksum.sh
###############################################################################
helm3_version=3.20.1
kustomize5_version=5.8.1
protoc_version=29.3
oras_version=1.2.0
git_lfs_version=3.7.1
# goreleaser CLI for GitHub release workflow (see hack/ci/install-goreleaser.sh)
goreleaser_version=v2.14.3
