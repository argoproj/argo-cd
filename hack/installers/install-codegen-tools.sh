#!/bin/bash
set -eux -o pipefail

KUSTOMIZE_VERSION=3.8.1 "$(dirname $0)/../install.sh" helm2-linux jq-linux kustomize-linux protoc-linux swagger-linux