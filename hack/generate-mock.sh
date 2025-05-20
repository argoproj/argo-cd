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

# output tool versions
mockery --version

mockery --config ${PROJECT_ROOT}/.mockery.yaml