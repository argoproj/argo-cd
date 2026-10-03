#! /usr/bin/env bash

set -x
set -o errexit
set -o nounset
set -o pipefail

# shellcheck disable=SC2128
PROJECT_ROOT=$(
    cd "$(dirname "${BASH_SOURCE}")"/..
    pwd
)
PATH="${PROJECT_ROOT}/dist:${PATH}"
# codegen-local runs this while vendor/ exists. In vendor mode the locally replaced gitops-engine resolves
# under vendor/, so mockery would write its mocks there and leave the real ones stale.
export GOFLAGS=-mod=mod

# output tool versions
mockery version

mockery --config "${PROJECT_ROOT}"/.mockery.yaml

# Generate mocks for gitops-engine
cd "${PROJECT_ROOT}"/gitops-engine
mockery --config .mockery.yaml
