#!/bin/bash
set -eux -o pipefail

KUSTOMIZE_VERSION=4.1.2 "$(dirname $0)/../install.sh" helm2-linux jq-linux kustomize-linux protoc-linux swagger-linux
