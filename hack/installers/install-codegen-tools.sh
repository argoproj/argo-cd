#!/bin/bash
set -eux -o pipefail

KUSTOMIZE_VERSION=4.5.5 "$(dirname $0)/../install.sh" kustomize protoc
