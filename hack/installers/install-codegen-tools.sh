#!/bin/bash
set -eux -o pipefail

KUSTOMIZE_VERSION=2.0.3 "$(dirname $0)/../install.sh" helm2-linux jq-linux kustomize-linux protoc-linux swagger-linux