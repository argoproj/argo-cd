#!/bin/bash
set -eux -o pipefail

KUSTOMIZE_VERSION=3.2.1 "$(dirname $0)/../install.sh" helm-linux helm2-linux jq-linux kustomize-linux protoc-linux swagger-linux
