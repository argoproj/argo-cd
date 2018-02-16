#!/bin/bash

set -x
set -o errexit
set -o nounset
set -o pipefail

PROJECT_ROOT=$(cd $(dirname "$0")/.. ; pwd)

PROTO_FILES=$(find ${PROJECT_ROOT} -name "*.proto" -not -path "${PROJECT_ROOT}/vendor/*")

for i in ${PROTO_FILES}; do
    echo $i
    dir=$(dirname $i)
    #pkgname=$(echo "${dirname}" | sed s%${PROJECT_ROOT}/%%g)
    protofile=$(basename $i)
    # Both /root/go and ${PROJECT_ROOT} are added to the protoc includes, in order to support
    # the requirement of running make inside docker and on desktop, respectively.
    #cd ${dir}
    protoc \
        -I${PROJECT_ROOT} \
        -I/usr/local/include \
        -I/root/go/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
        -I${PROJECT_ROOT}/vendor/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
        --go_out=plugins=grpc:$GOPATH/src \
        --grpc-gateway_out=logtostderr=true:$GOPATH/src \
        $i
done
