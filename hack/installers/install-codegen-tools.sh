#!/bin/bash
set -eux -o pipefail

"$(dirname "$0")/../install.sh" helm kustomize protoc
