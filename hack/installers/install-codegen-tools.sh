#!/bin/bash
set -eux -o pipefail

KUSTOMIZE_VERSION=5.4.3 "$(dirname $0)/../install.sh" kustomize protoc
