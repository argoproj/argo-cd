#!/bin/sh
###############################################################################
# This file defines the versions of the tools that are installed in the CI
# toolchain and the Docker image.
#
# Updating a tool's version here is not enough, you will need to create a
# checksum file in ./hack/installers/checksums matching the name of the
# downloaded binary with a ".sha256" suffix appended, containing the proper
# SHA256 sum of the binary.
###############################################################################
awscliv2_version=2.4.6
helm2_version=2.17.0
helm3_version=3.8.0
kubectl_version=1.17.8
kubectx_version=0.6.3
kustomize4_version=4.4.1
protoc_version=3.17.3
