#!/usr/bin/env bash

# ARGOCD_E2E_DIR can be set to customize the e2e test data directory on the host
# This is useful on macOS where Docker doesn't share /tmp with the host
# The host directory is mounted to /tmp/argo-e2e inside the container
ARGOCD_E2E_DIR="${ARGOCD_E2E_DIR:-/tmp/argo-e2e}"

docker run --name e2e-git --rm -i \
    -p 2222:2222 -p 9080:9080 -p 9443:9443 -p 9444:9444 -p 9445:9445 \
    -w /go/src/github.com/argoproj/argo-cd \
    -v /tmp:/tmp \
    -v "$ARGOCD_E2E_DIR":/tmp/argo-e2e \
    -v "$(pwd)":/go/src/github.com/argoproj/argo-cd \
    docker.io/argoproj/argo-cd-ci-builder:v1.0.0 \
    bash -c "goreman -f ./test/fixture/testrepos/Procfile start"
