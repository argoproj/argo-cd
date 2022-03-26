#!/bin/bash
set -eux -o pipefail

KUSTOMIZE_VERSION=4.5.3 "$(dirname $0)/../install.sh" helm2-linux kustomize-linux protoc
