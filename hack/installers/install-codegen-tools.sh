#!/bin/bash
set -eux -o pipefail

KUSTOMIZE_VERSION=4.4.1 "$(dirname $0)/../install.sh" kustomize protoc
