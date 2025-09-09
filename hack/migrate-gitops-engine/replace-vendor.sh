#!/bin/bash

# Script to replace all occurrences of "go work vendor" with "go work vendor" recursively

set -e

# Add common Go binary path to PATH if not already present
export PATH=$PATH:/usr/local/go/bin

go work init ./gitops-engine
go work use .
go work vendor
go mod tidy

echo "Searching for files containing 'go work vendor'..."

# Replace in each file
for file in Makefile Tiltfile hack/generate-proto.sh .github/workflows/bump-major-version.yaml .github/workflows/ci-build.yaml; do
    echo "Processing: $file"
    # Create a backup and replace
    sed -i.bak 's/go mod vendor/go work vendor/g' "$file"
    echo "  - Replaced occurrences in $file"
    echo "  - Backup created: $file.bak"
done

echo
echo "Replacement complete!"
echo "Removing backup files..."
find . -name '*.bak' -delete

echo "Syncing go mod files..."
go work sync
go work edit \
   -replace k8s.io/api=k8s.io/api@v0.34.0 \
   -replace k8s.io/apiextensions-apiserver=k8s.io/apiextensions-apiserver@v0.34.0 \
   -replace k8s.io/apimachinery=k8s.io/apimachinery@v0.34.0 \
   -replace k8s.io/client-go=k8s.io/client-go@v0.34.0 \
   -replace k8s.io/cli-runtime=k8s.io/cli-runtime@v0.34.0 \
   -replace k8s.io/kubectl=k8s.io/kubectl@v0.34.0 \
   -replace k8s.io/apiserver=k8s.io/apiserver@v0.34.0 \
   -replace k8s.io/component-base=k8s.io/component-base@v0.34.0 \
   -replace k8s.io/component-helpers=k8s.io/component-helpers@v0.34.0 \
   -replace k8s.io/controller-manager=k8s.io/controller-manager@v0.34.0 \
   -replace k8s.io/kube-aggregator=k8s.io/kube-aggregator@v0.34.0 \
   -replace k8s.io/cloud-provider=k8s.io/cloud-provider@v0.34.0 \
   -replace k8s.io/cluster-bootstrap=k8s.io/cluster-bootstrap@v0.34.0 \
   -replace k8s.io/code-generator=k8s.io/code-generator@v0.34.0 \
   -replace k8s.io/cri-api=k8s.io/cri-api@v0.34.0 \
   -replace k8s.io/cri-client=k8s.io/cri-client@v0.34.0 \
   -replace k8s.io/csi-translation-lib=k8s.io/csi-translation-lib@v0.34.0 \
   -replace k8s.io/dynamic-resource-allocation=k8s.io/dynamic-resource-allocation@v0.34.0 \
   -replace k8s.io/endpointslice=k8s.io/endpointslice@v0.34.0 \
   -replace k8s.io/externaljwt=k8s.io/externaljwt@v0.34.0 \
   -replace k8s.io/kms=k8s.io/kms@v0.34.0 \
   -replace k8s.io/kube-controller-manager=k8s.io/kube-controller-manager@v0.34.0 \
   -replace k8s.io/kube-proxy=k8s.io/kube-proxy@v0.34.0 \
   -replace k8s.io/kube-scheduler=k8s.io/kube-scheduler@v0.34.0 \
   -replace k8s.io/kubelet=k8s.io/kubelet@v0.34.0 \
   -replace k8s.io/legacy-cloud-providers=k8s.io/legacy-cloud-providers@v0.34.0 \
   -replace k8s.io/metrics=k8s.io/metrics@v0.34.0 \
   -replace k8s.io/mount-utils=k8s.io/mount-utils@v0.34.0 \
   -replace k8s.io/pod-security-admission=k8s.io/pod-security-admission@v0.34.0 \
   -replace k8s.io/sample-apiserver=k8s.io/sample-apiserver@v0.34.0 \
   -replace k8s.io/sample-cli-plugin=k8s.io/sample-cli-plugin@v0.34.0 \
   -replace k8s.io/sample-controller=k8s.io/sample-controller@v0.34.0
go mod tidy
go work vendor

