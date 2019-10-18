#!/bin/bash
set -eux -o pipefail

"$(dirname $0)/../install.sh" helm-linux jq-linux kustomize-linux protoc-linux swagger-linux