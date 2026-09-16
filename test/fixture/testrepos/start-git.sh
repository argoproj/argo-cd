#!/usr/bin/env bash

# ARGOCD_E2E_DIR can be set to customize the e2e test data directory on the host
# This is useful on macOS where Docker doesn't share /tmp with the host
# The host directory is mounted to /tmp/argo-e2e inside the container
ARGOCD_E2E_DIR="${ARGOCD_E2E_DIR:-/tmp/argo-e2e}"

# Resolve symlinks (e.g. macOS's /tmp -> /private/tmp) before handing these
# paths to `docker run -v`. The bind-mount source is resolved by the Docker
# daemon, which on Rancher Desktop/Docker Desktop for Mac runs inside a Linux
# VM: only the real host path (/private/tmp) is shared into that VM, not the
# /tmp symlink, so mounting the unresolved /tmp path silently falls back to
# the VM's own disconnected, empty /tmp instead of the host's. This is a
# no-op on Linux, where /tmp isn't a symlink. See docs/developer-guide/test-e2e.md.
resolved_tmp="$(cd /tmp && pwd -P)"
resolved_e2e_dir="$(cd "$ARGOCD_E2E_DIR" && pwd -P)"

docker run --name e2e-git --rm -i \
    -p 2222:2222 -p 9080:9080 -p 9443:9443 -p 9444:9444 -p 9445:9445 \
    -w /go/src/github.com/argoproj/argo-cd \
    -v "$resolved_tmp":/tmp \
    -v "$resolved_e2e_dir":/tmp/argo-e2e \
    -v "$(pwd)":/go/src/github.com/argoproj/argo-cd \
    docker.io/argoproj/argo-cd-ci-builder:v1.0.0 \
    bash -c "goreman -f ./test/fixture/testrepos/Procfile start"
