#!/bin/bash
set -eux -o pipefail

$(dirname $0)/install-goimports.sh

cd $DOWNLOADS
GO111MODULE=on go get github.com/gogo/protobuf/gogoproto@v1.2.1
GO111MODULE=on go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.0-beta.2
GO111MODULE=on go get github.com/golang/protobuf/protoc-gen-go@v1.3.1
GO111MODULE=on go get github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@v1.9.2
GO111MODULE=on go get github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger@v1.9.2