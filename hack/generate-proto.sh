#!/bin/bash

set -x
set -o errexit
set -o nounset
set -o pipefail

PROJECT_ROOT=$(cd $(dirname "$0")/.. ; pwd)

PROTO_FILES=$(find ${PROJECT_ROOT} -name "*.proto" -not -path "${PROJECT_ROOT}/vendor/*")

for i in "${PROTO_FILES}"; do
    echo $i
    dir=$(dirname $i)
    protoc \
        -I ${dir} \
        -I ${PROJECT_ROOT}/vendor/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
        --go_out=plugins=grpc:${dir} \
        --grpc-gateway_out=logtostderr=true:${dir} \
        $i
done
